//go:build linux

package linux

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform/contrato"
)

// TestContratoVivo abre navegadores y visores de verdad, cada uno con un
// perfil y archivos temporales propios (nunca los del dueño), en el espacio 6
// sin foco, y comprueba que niveles() lee su URL o su documento. Al terminar
// cierra solo lo que abrió y borra sus temporales. Corre con
// LAZUL_CONTRATO_ABRIR=1 ./dev contrato; lo que no esté instalado se salta.
func TestContratoVivo(t *testing.T) {
	contrato.Exigir(t)
	if os.Getenv("LAZUL_CONTRATO_ABRIR") != "1" {
		t.Skip("abre ventanas: LAZUL_CONTRATO_ABRIR=1 ./dev contrato")
	}
	if _, err := exec.LookPath("hyprctl"); err != nil {
		t.Skip("sin Hyprland")
	}
	marca := fmt.Sprintf("vivo-%d", time.Now().UnixNano())
	url := "https://example.org/?lazul=" + marca

	chromium := func(bin string) func(dir string, abre string) string {
		return func(dir, abre string) string {
			return fmt.Sprintf("%s --user-data-dir=%s --no-first-run --no-default-browser-check --password-store=basic %s", bin, dir, abre)
		}
	}
	mozilla := func(bin string) func(dir string, abre string) string {
		return func(dir, abre string) string {
			// Perfil nuevo y temporal: sin bienvenida, y guarda la sesión cada
			// segundo para no esperar los 15 s de siempre.
			prefs := strings.Join([]string{
				`user_pref("browser.sessionstore.interval", 1000);`,
				`user_pref("browser.aboutwelcome.enabled", false);`,
				`user_pref("browser.startup.homepage_override.mstone", "ignore");`,
				`user_pref("browser.shell.checkDefaultBrowser", false);`,
				`user_pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);`,
				`user_pref("zen.welcome-screen.seen", true);`,
			}, "\n")
			if err := os.WriteFile(filepath.Join(dir, "user.js"), []byte(prefs), 0o600); err != nil {
				t.Fatal(err)
			}
			return fmt.Sprintf("%s --no-remote --profile %s %s", bin, dir, abre)
		}
	}
	navegadores := []struct {
		nombre, bin string
		orden       func(dir, abre string) string
	}{
		{"chromium", "chromium", chromium("chromium")},
		{"chrome", "google-chrome-stable", chromium("google-chrome-stable")},
		{"edge", "microsoft-edge-stable", chromium("microsoft-edge-stable")},
		{"brave", "brave", chromium("brave")},
		{"firefox", "firefox", mozilla("firefox")},
		{"zen", "zen-browser", mozilla("zen-browser")},
	}
	for _, n := range navegadores {
		t.Run("URL/"+n.nombre, func(t *testing.T) {
			dir := temporal(t, n.bin)
			pid := abrirEn6(t, n.orden(dir, url), dir)
			esperarNivel(t, pid, func(u, _ string) bool { return u == url })
		})
	}
	t.Run("Archivo/chromium_pdf", func(t *testing.T) {
		dir := temporal(t, "chromium")
		pdf := filepath.Join(dir, "lazul-"+marca+".pdf")
		escribir(t, pdf, pdfMinimo())
		pid := abrirEn6(t, chromium("chromium")(dir, "file://"+pdf), dir)
		esperarNivel(t, pid, func(u, a string) bool { return u == "" && a == pdf })
	})

	visores := []struct {
		nombre, bin, ext string
		datos            func() []byte
		orden            func(archivo string) string
	}{
		{"imv", "imv", ".png", pngMinimo, func(a string) string { return "imv " + a }},
		{"evince", "evince", ".pdf", pdfMinimo, func(a string) string { return "evince " + a }},
		{"mpv", "mpv", ".wav", wavMinimo, func(a string) string { return "mpv --pause --mute=yes --force-window=yes " + a }},
	}
	for _, v := range visores {
		t.Run("Archivo/"+v.nombre, func(t *testing.T) {
			dir := temporal(t, v.bin)
			archivo := filepath.Join(dir, "lazul-"+marca+v.ext)
			escribir(t, archivo, v.datos())
			pid := abrirEn6(t, v.orden(archivo), dir)
			esperarNivel(t, pid, func(u, a string) bool { return u == "" && a == archivo })
		})
	}
}

// temporal crea un directorio propio de la prueba (se salta si bin no está).
func temporal(t *testing.T, bin string) string {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s no está instalado", bin)
	}
	dir, err := os.MkdirTemp("", "lazul-vivo-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func escribir(t *testing.T, ruta string, datos []byte) {
	t.Helper()
	if err := os.WriteFile(ruta, datos, 0o600); err != nil {
		t.Fatal(err)
	}
}

type cliente struct {
	PID    int    `json:"pid"`
	Titulo string `json:"title"`
}

func clientes(t *testing.T) []cliente {
	t.Helper()
	datos, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		t.Fatalf("hyprctl clients: %v", err)
	}
	var cs []cliente
	if err := json.Unmarshal(datos, &cs); err != nil {
		t.Fatal(err)
	}
	return cs
}

// abrirEn6 lanza orden en el espacio 6 sin foco (no molesta al dueño) y
// devuelve el PID de la ventana cuyo proceso lleva marca en sus argumentos.
// Al terminar la prueba cierra ese proceso.
func abrirEn6(t *testing.T, orden, marca string) int {
	t.Helper()
	if strings.Contains(orden, "'") {
		t.Fatalf("orden con comillas: %q", orden)
	}
	lua := fmt.Sprintf("hl.dsp.exec_cmd('%s', { workspace = '6 silent', no_initial_focus = true })", orden)
	if salida, err := exec.Command("hyprctl", "dispatch", lua).CombinedOutput(); err != nil || !bytes.HasPrefix(salida, []byte("ok")) {
		t.Fatalf("hyprctl dispatch: %v %s", err, salida)
	}
	limite := time.Now().Add(30 * time.Second)
	for time.Now().Before(limite) {
		for _, c := range clientes(t) {
			linea, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", c.PID))
			if err == nil && bytes.Contains(linea, []byte(marca)) {
				t.Cleanup(func() { cerrar(c.PID) })
				return c.PID
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("no apareció la ventana de %q", orden)
	return 0
}

func cerrar(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// esperarNivel repite niveles() con el título del momento hasta que ok diga
// que sí (el navegador guarda la sesión con retraso). No registra la URL.
func esperarNivel(t *testing.T, pid int, ok func(url, archivo string) bool) {
	t.Helper()
	limite := time.Now().Add(40 * time.Second)
	var u, a string
	for time.Now().Before(limite) {
		for _, c := range clientes(t) {
			if c.PID == pid {
				u, a = niveles(pid, c.Titulo)
			}
		}
		if ok(u, a) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("niveles() no dio lo esperado: url vacía=%v, archivo=%q", u == "", a)
}

func pngMinimo() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewGray(image.Rect(0, 0, 8, 8)))
	return b.Bytes()
}

// pdfMinimo: una página en blanco, sin título (los visores muestran el nombre del archivo).
func pdfMinimo() []byte {
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	var offs []int
	for i, o := range objs {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// wavMinimo: un segundo de silencio.
func wavMinimo() []byte {
	const tasa, muestras = 8000, 8000
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + muestras))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(1))
	w(uint32(tasa))
	w(uint32(tasa))
	w(uint16(1))
	w(uint16(8))
	b.WriteString("data")
	w(uint32(muestras))
	b.Write(bytes.Repeat([]byte{128}, muestras))
	return b.Bytes()
}
