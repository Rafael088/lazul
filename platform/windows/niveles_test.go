//go:build windows

package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/Rafael088/lazul/platform/contrato"
	"golang.org/x/sys/windows"
)

func TestDocumentoAbierto(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "informe final.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linea := fmt.Sprintf(`"C:\Program Files\Visor\visor.exe" /n "%s"`, pdf)
	if got := documentoAbierto(linea, "informe final.pdf - Visor"); got != pdf {
		t.Errorf("con el documento en los argumentos: %q", got)
	}
	if got := documentoAbierto(linea, "Visor"); got != "" {
		t.Errorf("sin el nombre en el título: %q", got)
	}
	if got := documentoAbierto(`visor.exe`, pdf+" - Visor"); got != pdf {
		t.Errorf("con la ruta en el título: %q", got)
	}
	if got := documentoAbierto(`visor.exe "C:\no\existe.pdf"`, "existe.pdf - Visor"); got != "" {
		t.Errorf("con un archivo que no existe: %q", got)
	}
}

func TestLineaDeOrdenes(t *testing.T) {
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(p)
	linea := lineaDeOrdenes(p)
	if !strings.Contains(strings.ToLower(linea), strings.ToLower(filepath.Base(os.Args[0]))) {
		t.Errorf("la línea de órdenes de esta prueba no trae su ejecutable: %q", linea)
	}
}

// TestContratoVivo abre los navegadores instalados, cada uno con un perfil
// temporal propio (nunca el del dueño), y comprueba que se lee su URL por UI
// Automation y que un PDF local abierto en el navegador sale como documento.
// Cierra solo lo que abrió y borra sus temporales. Corre en la sesión del
// escritorio: LAZUL_CONTRATO_ABRIR=1 ./dev win contrato.
func TestContratoVivo(t *testing.T) {
	contrato.Exigir(t)
	if os.Getenv("LAZUL_CONTRATO_ABRIR") != "1" {
		t.Skip("abre ventanas: LAZUL_CONTRATO_ABRIR=1 ./dev win contrato")
	}
	marca := fmt.Sprintf("vivo-%d", time.Now().UnixNano())
	url := "https://example.org/?lazul=" + marca
	pf, pf86, local := os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")
	chromium := func(dir, abre string) []string {
		return []string{"--user-data-dir=" + dir, "--no-first-run", "--no-default-browser-check", "--new-window", abre}
	}
	navegadores := []struct {
		nombre string
		rutas  []string
		args   func(dir, abre string) []string
	}{
		{"chrome", []string{pf + `\Google\Chrome\Application\chrome.exe`, local + `\Google\Chrome\Application\chrome.exe`}, chromium},
		{"edge", []string{pf86 + `\Microsoft\Edge\Application\msedge.exe`, pf + `\Microsoft\Edge\Application\msedge.exe`}, chromium},
		{"brave", []string{pf + `\BraveSoftware\Brave-Browser\Application\brave.exe`}, chromium},
		{"firefox", []string{pf + `\Mozilla Firefox\firefox.exe`}, func(dir, abre string) []string {
			return []string{"-no-remote", "-profile", dir, abre}
		}},
	}
	for _, n := range navegadores {
		exe := ""
		for _, r := range n.rutas {
			if _, err := os.Stat(r); err == nil {
				exe = r
				break
			}
		}
		t.Run("URL/"+n.nombre, func(t *testing.T) {
			if exe == "" {
				t.Skipf("%s no está instalado", n.nombre)
			}
			dir := temporal(t)
			v := abrirYBuscar(t, exe, n.args(dir, url), dir)
			esperarNivel(t, v, func(u, _ string) bool { return u == url })
		})
		t.Run("Archivo/"+n.nombre+"_pdf", func(t *testing.T) {
			if exe == "" || n.nombre == "firefox" {
				t.Skipf("%s no está instalado o no se prueba con PDF", n.nombre)
			}
			dir := temporal(t)
			pdf := filepath.Join(dir, "lazul-"+marca+".pdf")
			if err := os.WriteFile(pdf, pdfMinimo(), 0o600); err != nil {
				t.Fatal(err)
			}
			v := abrirYBuscar(t, exe, n.args(dir, "file:///"+filepath.ToSlash(pdf)), dir)
			esperarNivel(t, v, func(u, a string) bool { return u == "" && strings.EqualFold(a, pdf) })
		})
	}
}

func temporal(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "lazul-vivo-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// El navegador suelta sus archivos un poco después de cerrarse.
		for i := 0; i < 40; i++ {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
		t.Logf("no pude borrar %s", dir)
	})
	return dir
}

type ventanaViva struct {
	hwnd uintptr
	pid  uint32
}

var (
	enumWindowsVivo     = user32AppActiva.NewProc("EnumWindows")
	isWindowVisibleVivo = user32AppActiva.NewProc("IsWindowVisible")
)

// ventanasDe: ventanas visibles con título de procesos cuya línea de órdenes
// lleva marca.
func ventanasDe(marca string) []ventanaViva {
	var vs []ventanaViva
	lineas := map[uint32]string{}
	cb := syscall.NewCallback(func(h, _ uintptr) uintptr {
		if v, _, _ := isWindowVisibleVivo.Call(h); v == 0 {
			return 1
		}
		if titulo, _ := tituloVentana(h); titulo == "" {
			return 1
		}
		var pid uint32
		getWindowThreadProcessID.Call(h, uintptr(unsafe.Pointer(&pid)))
		linea, ok := lineas[pid]
		if !ok {
			if p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid); err == nil {
				linea = lineaDeOrdenes(p)
				windows.CloseHandle(p)
			}
			lineas[pid] = linea
		}
		if strings.Contains(linea, marca) {
			vs = append(vs, ventanaViva{h, pid})
		}
		return 1
	})
	enumWindowsVivo.Call(cb, 0)
	return vs
}

// abrirYBuscar lanza exe y espera su ventana; al terminar cierra los
// procesos que lleven marca (solo los suyos: el perfil temporal).
func abrirYBuscar(t *testing.T, exe string, args []string, marca string) ventanaViva {
	t.Helper()
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pids := map[uint32]bool{uint32(cmd.Process.Pid): true}
		for _, v := range ventanasDe(marca) {
			pids[v.pid] = true
		}
		for pid := range pids {
			_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(pid)).Run()
		}
		_ = cmd.Wait()
	})
	limite := time.Now().Add(30 * time.Second)
	for time.Now().Before(limite) {
		if vs := ventanasDe(marca); len(vs) > 0 {
			return vs[0]
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("no apareció la ventana de %s", filepath.Base(exe))
	return ventanaViva{}
}

// esperarNivel repite niveles() hasta que ok diga que sí. No registra la URL.
func esperarNivel(t *testing.T, v ventanaViva, ok func(url, archivo string) bool) {
	t.Helper()
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, v.pid)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(p)
	ruta, err := rutaEjecutable(p)
	if err != nil {
		t.Fatal(err)
	}
	limite := time.Now().Add(40 * time.Second)
	var u, a string
	var dur time.Duration
	for time.Now().Before(limite) {
		titulo, _ := tituloVentana(v.hwnd)
		inicio := time.Now()
		u, a = niveles(v.hwnd, p, filepath.Base(ruta), titulo)
		dur = time.Since(inicio)
		if ok(u, a) {
			t.Logf("leído en %v", dur.Round(time.Millisecond))
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("niveles() no dio lo esperado (última lectura en %v): url vacía=%v, archivo=%q", dur, u == "", a)
}

// pdfMinimo: una página en blanco.
func pdfMinimo() []byte {
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>",
	}
	var b strings.Builder
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
	return []byte(b.String())
}
