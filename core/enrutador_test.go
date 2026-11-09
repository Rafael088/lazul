package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rafael088/lazul/core/apps"
	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// mesa: un receptor "Windows" falso que recibe archivos, ya emparejado con
// un controlador, y el Enrutador del controlador apuntándole.
type mesa struct {
	otro     *falso.Falso
	local    *falso.Falso
	e        *Enrutador
	dir      string
	receptor *Identidad
	ctrl     *Identidad
	bandeja  string
}

func nuevaMesa(t *testing.T) *mesa {
	t.Helper()
	cat, err := apps.Cargar()
	if err != nil {
		t.Fatal(err)
	}
	m := &mesa{otro: falso.New("win", "Code.exe"), local: falso.New("linux"), bandeja: t.TempDir()}
	m.receptor, _ = NuevaIdentidad("receptor")
	m.ctrl, _ = NuevaIdentidad("controlador")
	p := &Puerta{Identidad: m.receptor, Pares: NuevosPares()}
	if err := p.Pares.Fijar(Par{Huella: m.ctrl.Huella, Nombre: "Controlador"}); err != nil {
		t.Fatal(err)
	}
	r := &Receptor{Sistema: m.otro.Sistema(), Hola: &lazulv1.Hola{Nombre: "PC Windows"}, Catalogo: &cat, SO: apps.SistemaWindows,
		Archivos: &localsend.Servidor{Bandeja: m.bandeja}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go Servir(ctx, ln, r, p)
	m.dir = ln.Addr().String()
	con, err := conectar(m.dir, m.ctrl, m.receptor.Huella)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { con.Cerrar() })
	otro := &Remoto{Nombre: "PC Windows", Con: con, HTTP: ClienteHTTP(m.ctrl, m.receptor.Huella), Base: "https://" + m.dir}
	m.e = &Enrutador{Local: m.local.Sistema(), Catalogo: &cat, SO: apps.SistemaLinux, PorDefecto: otro}
	return m
}

func (m *mesa) gesto(t *testing.T, g lazulv1.Gesto) (Hecho, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.e.Gesto(ctx, &lazulv1.EventoGesto{Gesto: g, Direccion: lazulv1.Direccion_DIRECCION_DERECHA})
}

func TestSoltarSinAgarrar(t *testing.T) {
	m := nuevaMesa(t)
	if _, err := m.gesto(t, lazulv1.Gesto_GESTO_SOLTAR); !errors.Is(err, ErrNadaAgarrado) {
		t.Fatalf("soltar sin agarrar: %v", err)
	}
}

func TestCapturaDePuntaAPunta(t *testing.T) {
	for _, adelantar := range []bool{false, true} {
		m := nuevaMesa(t)
		m.e.Adelantar = adelantar
		if _, err := m.gesto(t, lazulv1.Gesto_GESTO_AGARRAR); err != nil {
			t.Fatal(err)
		}
		h, err := m.gesto(t, lazulv1.Gesto_GESTO_SOLTAR)
		if err != nil {
			t.Fatalf("adelantar=%v: %v", adelantar, err)
		}
		if h.Adelantada != adelantar || h.Que != "captura" || h.Destino != "PC Windows" {
			t.Errorf("adelantar=%v: %+v", adelantar, h)
		}
		ab := m.otro.Abiertas()
		if len(ab) != 1 || filepath.Dir(ab[0].Archivo) != m.bandeja {
			t.Fatalf("adelantar=%v: el receptor abrió %v", adelantar, ab)
		}
		img, _ := m.local.Capture(context.Background())
		if b, _ := os.ReadFile(ab[0].Archivo); !bytes.Equal(b, img.PNG) {
			t.Errorf("adelantar=%v: la captura recibida no es la de este equipo", adelantar)
		}
		// Lo agarrado se suelta una vez.
		if _, err := m.gesto(t, lazulv1.Gesto_GESTO_SOLTAR); !errors.Is(err, ErrNadaAgarrado) {
			t.Errorf("adelantar=%v: soltó dos veces: %v", adelantar, err)
		}
	}
}

func TestLocalSendSoloEmparejados(t *testing.T) {
	m := nuevaMesa(t)
	intruso, _ := NuevaIdentidad("intruso")
	// Con su propia identidad y la huella buena del receptor: el TLS pasa,
	// pero el receptor no lo tiene emparejado.
	_, err := localsend.Enviar(context.Background(), ClienteHTTP(intruso, m.receptor.Huella), "https://"+m.dir,
		localsend.Dispositivo{}, localsend.Archivo{Nombre: "x.png", Datos: []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("un no emparejado subió un archivo: %v", err)
	}
	// Un cliente que espera otra huella ni siquiera llega a hablar.
	otra, _ := NuevaIdentidad("otra")
	_, err = localsend.Enviar(context.Background(), ClienteHTTP(m.ctrl, otra.Huella), "https://"+m.dir,
		localsend.Dispositivo{}, localsend.Archivo{Nombre: "x.png", Datos: []byte("x")})
	if !errors.Is(err, ErrHuellaCambiada) {
		t.Fatalf("subió a un receptor con otra huella: %v", err)
	}
	if entradas, _ := os.ReadDir(m.bandeja); len(entradas) != 0 {
		t.Fatalf("quedaron archivos en la bandeja: %v", entradas)
	}
}

func TestEntregarArchivoQueNoLlego(t *testing.T) {
	m := nuevaMesa(t)
	err := m.e.PorDefecto.Entregar(context.Background(), &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Archivo{Archivo: &lazulv1.Archivo{
		Nombre: "a.png", Sha256: make([]byte, 32), TransferenciaId: "no-existe"}}})
	if err == nil || !strings.Contains(err.Error(), "NO_ENCONTRADO") {
		t.Fatalf("aceptó un archivo que no llegó: %v", err)
	}
}

func TestGesto4PorNiveles(t *testing.T) {
	m := nuevaMesa(t)
	doc := filepath.Join(t.TempDir(), "informe.txt")
	if err := os.WriteFile(doc, []byte("hola"), 0o600); err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		ventana platform.App
		que     string
		abre    func(platform.AppRef) bool
	}{
		{platform.App{ID: "firefox", URL: "https://lazul.dev/x"}, "url", func(r platform.AppRef) bool { return r.URL == "https://lazul.dev/x" }},
		{platform.App{ID: "editor", Archivo: doc}, "archivo", func(r platform.AppRef) bool {
			b, _ := os.ReadFile(r.Archivo)
			return filepath.Base(r.Archivo) == "informe.txt" && string(b) == "hola"
		}},
		// Code en Linux es vscode en el catálogo, y en Windows Code.exe.
		{platform.App{ID: "Code", Nombre: "Code"}, "app", func(r platform.AppRef) bool { return r.ID == "Code.exe" }},
		// Obsidian no está instalado en el receptor: cae a su web.
		{platform.App{ID: "obsidian"}, "app", func(r platform.AppRef) bool { return r.URL == "https://obsidian.md/" }},
	}
	for i, k := range casos {
		m.local.PonerApp(k.ventana)
		h, err := m.gesto(t, lazulv1.Gesto_GESTO_MANDAR_APP)
		if err != nil {
			t.Fatalf("%s: %v", k.ventana.ID, err)
		}
		ab := m.otro.Abiertas()
		if h.Que != k.que || len(ab) != i+1 || !k.abre(ab[i]) {
			t.Errorf("%s: hizo %q y el receptor abrió %v", k.ventana.ID, h.Que, ab)
		}
	}
	// Una app que el catálogo no conoce y el receptor no tiene: falla.
	m.local.PonerApp(platform.App{ID: "app-rara", Nombre: "Rara"})
	if _, err := m.gesto(t, lazulv1.Gesto_GESTO_MANDAR_APP); err == nil || !strings.Contains(err.Error(), "NO_ENCONTRADO") {
		t.Fatalf("app desconocida: %v", err)
	}
}

func TestArchivoEjecutableLlegaPeroNoSeAbre(t *testing.T) {
	m := nuevaMesa(t)
	datos := []byte("MZ no soy un programa")
	suma := sha256.Sum256(datos)
	id, err := localsend.Enviar(context.Background(), ClienteHTTP(m.ctrl, m.receptor.Huella), "https://"+m.dir,
		localsend.Dispositivo{}, localsend.Archivo{Nombre: "instalador.exe", Datos: datos})
	if err != nil {
		t.Fatal(err)
	}
	err = m.e.PorDefecto.Entregar(context.Background(), &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Archivo{Archivo: &lazulv1.Archivo{
		Nombre: "instalador.exe", Sha256: suma[:], TransferenciaId: id}}})
	if err != nil {
		t.Fatal(err)
	}
	if ab := m.otro.Abiertas(); len(ab) != 0 {
		t.Fatalf("el receptor abrió un ejecutable: %v", ab)
	}
	if entradas, _ := os.ReadDir(m.bandeja); len(entradas) != 1 {
		t.Fatalf("el archivo tenía que quedar en la bandeja: %v", entradas)
	}
}
