//go:build linux

package linux

import (
	"context"
	"errors"
	"math"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
	"github.com/godbus/dbus/v5"
)

const ventanaJSON = `{
  "address": "0x55d4c8a0b2f0", "mapped": true, "hidden": false,
  "at": [100, 50], "size": [800, 600], "workspace": {"id": 1, "name": "1"},
  "floating": false, "monitor": 0, "class": "firefox", "title": "Pista — Mozilla Firefox",
  "pid": 4242, "xwayland": false, "fullscreen": 0
}`

const monitoresImanJSON = `[
  {"id": 0, "name": "eDP-1", "width": 2880, "height": 1800, "x": 0, "y": 0, "scale": 2.0, "transform": 0, "focused": true, "disabled": false, "mirrorOf": "none"},
  {"id": 1, "name": "HDMI-A-1", "width": 1920, "height": 1080, "x": 1440, "y": 0, "scale": 1.0, "transform": 0, "focused": false, "disabled": false, "mirrorOf": "none"}
]`

func cerca(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func rectObjetivo(x, y, ancho, alto float64) rectA11y {
	return rectA11y{x: x, y: y, ancho: ancho, alto: alto, rol: platform.TargetRoleButton}
}

func TestLeerVentanaActiva(t *testing.T) {
	v, id, err := leerVentanaActiva([]byte(ventanaJSON))
	if err != nil || id != 0x55d4c8a0b2f0 || v.PID != 4242 || v.En != [2]float64{100, 50} || v.Tamano != [2]float64{800, 600} {
		t.Fatalf("ventana = %+v, %x, %v", v, id, err)
	}
	for _, sin := range []string{"", "{}", "Invalid", `{"address": ""}`} {
		if _, _, err := leerVentanaActiva([]byte(sin)); !errors.Is(err, platform.ErrSinVentana) {
			t.Errorf("%q: err = %v, quiero ErrSinVentana", sin, err)
		}
	}
}

func TestAPantallaLlevaLaVentanaAlMonitor(t *testing.T) {
	v, _, _ := leerVentanaActiva([]byte(ventanaJSON))
	m := platform.Display{Name: "eDP-1", Width: 1440, Height: 900}
	marco := rectObjetivo(0, 0, 800, 600)
	rs := []rectA11y{
		rectObjetivo(10, 10, 30, 20),    // botón
		rectObjetivo(0, 0, 800, 600),    // la ventana entera: el borde no es objetivo
		rectObjetivo(20, 20, 780, 560),  // casi toda la ventana
		rectObjetivo(10, 900, 50, 20),   // desplazado fuera de la vista
		rectObjetivo(-200, 10, 50, 20),  // a la izquierda, fuera
		rectObjetivo(40, 40, 0, 20),     // sin ancho
		rectObjetivo(700, 580, 120, 40), // centro dentro, se sale un poco
	}
	s := aPantalla(v, 2, marco, rs, m)
	if s.LogicalWidth != 1440 || s.LogicalHeight != 900 {
		t.Fatalf("tamaño lógico = %vx%v", s.LogicalWidth, s.LogicalHeight)
	}
	if len(s.Targets) != 2 {
		t.Fatalf("objetivos = %+v, quiero 2", s.Targets)
	}
	b := s.Targets[0]
	if !cerca(b.X, 110.0/1440) || !cerca(b.Y, 60.0/900) || !cerca(b.Width, 30.0/1440) || !cerca(b.Height, 20.0/900) || b.Role != platform.TargetRoleButton {
		t.Errorf("botón = %+v", b)
	}
	if !cerca(s.WindowBounds.X, 100.0/1440) || !cerca(s.WindowBounds.Y, 50.0/900) || !cerca(s.WindowBounds.Width, 800.0/1440) {
		t.Errorf("límites de ventana = %+v", s.WindowBounds)
	}
	if !cerca(s.Targets[1].X, 800.0/1440) {
		t.Errorf("segundo = %+v", s.Targets[1])
	}
}

func TestAPantallaConPixelesFisicos(t *testing.T) {
	// Una app de XWayland a escala 2 sin escalar da el doble de todo.
	v, _, _ := leerVentanaActiva([]byte(ventanaJSON))
	m := platform.Display{Width: 1440, Height: 900}
	s := aPantalla(v, 2, rectObjetivo(0, 0, 1600, 1200), []rectA11y{rectObjetivo(20, 20, 60, 40)}, m)
	if len(s.Targets) != 1 {
		t.Fatalf("objetivos = %+v", s.Targets)
	}
	b := s.Targets[0]
	if !cerca(b.X, 110.0/1440) || !cerca(b.Width, 30.0/1440) {
		t.Errorf("botón = %+v, quiero el de píxeles lógicos", b)
	}
	// A escala 1 nunca se divide.
	s = aPantalla(v, 1, rectObjetivo(0, 0, 800, 600), []rectA11y{rectObjetivo(20, 20, 60, 40)}, m)
	if !cerca(s.Targets[0].X, 120.0/1440) {
		t.Errorf("escala 1: %+v", s.Targets[0])
	}
}

func TestAPantallaQuitaLaSombraYElOrigenDelMarco(t *testing.T) {
	v, _, _ := leerVentanaActiva([]byte(ventanaJSON))
	m := platform.Display{Width: 1440, Height: 900}
	// GTK con decoración propia: el marco lleva 20 px de sombra a cada lado.
	s := aPantalla(v, 1, rectObjetivo(0, 0, 840, 640), []rectA11y{rectObjetivo(30, 30, 30, 20)}, m)
	if len(s.Targets) != 1 || !cerca(s.Targets[0].X, 110.0/1440) || !cerca(s.Targets[0].Y, 60.0/900) {
		t.Errorf("con sombra: %+v", s.Targets)
	}
	// Un marco que empieza en (8, 30) de su ventana.
	s = aPantalla(v, 1, rectObjetivo(8, 30, 800, 600), []rectA11y{rectObjetivo(18, 40, 30, 20)}, m)
	if len(s.Targets) != 1 || !cerca(s.Targets[0].X, 110.0/1440) || !cerca(s.Targets[0].Y, 60.0/900) {
		t.Errorf("con origen: %+v", s.Targets)
	}
}

func TestAPantallaEnElSegundoMonitor(t *testing.T) {
	v := ventanaHyprland{En: [2]float64{1540, 100}, Tamano: [2]float64{1000, 800}, Monitor: 1}
	m := platform.Display{Name: "HDMI-A-1", X: 1440, Width: 1920, Height: 1080}
	s := aPantalla(v, 1, rectObjetivo(0, 0, 1000, 800), []rectA11y{rectObjetivo(100, 50, 40, 40)}, m)
	if len(s.Targets) != 1 || !cerca(s.Targets[0].X, 200.0/1920) || !cerca(s.Targets[0].Y, 150.0/1080) {
		t.Errorf("objetivos = %+v", s.Targets)
	}
	// La misma ventana vista desde el otro monitor queda fuera de (0,0)-(1,1).
	s = aPantalla(v, 1, rectObjetivo(0, 0, 1000, 800), []rectA11y{rectObjetivo(100, 50, 40, 40)}, platform.Display{Width: 1440, Height: 900})
	if len(s.Targets) != 0 {
		t.Errorf("desde el otro monitor: %+v", s.Targets)
	}
}

func TestMonitoresDeVentana(t *testing.T) {
	ms, err := monitoresLogicos([]byte(monitoresImanJSON))
	if err != nil || len(ms) != 2 || ms[0].Width != 1440 || ms[1].ID != 1 || ms[0].Escala != 2 {
		t.Fatalf("monitores = %+v, %v", ms, err)
	}
	tactil, propio, ok := monitoresDeVentana(ms, 1, nil)
	if !ok || tactil.Name != "eDP-1" || propio.Name != "HDMI-A-1" {
		t.Errorf("sin destino: táctil %s, ventana %s", tactil.Name, propio.Name)
	}
	tactil, _, _ = monitoresDeVentana(ms, 0, &platform.Display{Name: "HDMI-A-1"})
	if tactil.Name != "HDMI-A-1" {
		t.Errorf("con destino: táctil %s", tactil.Name)
	}
	if _, _, ok := monitoresDeVentana(nil, 0, nil); ok {
		t.Error("sin monitores no hay zona")
	}
}

func TestElegirMarco(t *testing.T) {
	casos := []struct {
		ms     []marcoA11y
		titulo string
		quiero int
	}{
		{[]marcoA11y{{Nombre: "a"}, {Nombre: "b", Activo: true}}, "a", 1},
		{[]marcoA11y{{Nombre: "a"}, {Nombre: "b"}}, "b", 1},
		{[]marcoA11y{{Nombre: "a"}, {Nombre: "b"}}, "c", -1},
		{[]marcoA11y{{Nombre: "solo"}}, "otra", 0},
		{[]marcoA11y{{Nombre: "a", Activo: true}, {Nombre: "b", Activo: true}}, "b", 1},
		{nil, "x", -1},
	}
	for i, c := range casos {
		if got := elegirMarco(c.ms, c.titulo); got != c.quiero {
			t.Errorf("caso %d: %d, quiero %d", i, got, c.quiero)
		}
	}
}

func TestReglaObjetivos(t *testing.T) {
	r := reglaObjetivos()
	if got := dbus.SignatureOf(r).String(); got != "(aiia{ss}iaiiasib)" {
		t.Fatalf("firma = %s, quiero la de AtspiMatchRule", got)
	}
	// LINK (88) es el bit 24 de la palabra 2; SHOWING (25) y SENSITIVE (24),
	// los bits 25 y 24 de la primera.
	if uint32(r.Roles[2])&(1<<24) == 0 || uint32(r.Estados[0]) != 1<<25|1<<24 || r.Estados[1] != 0 {
		t.Errorf("bits: roles %x, estados %x", r.Roles, r.Estados)
	}
	if uint32(r.Roles[4])&(1<<1) == 0 { // PUSH_BUTTON_MENU (129)
		t.Errorf("falta PUSH_BUTTON_MENU: %x", r.Roles)
	}
	for _, rol := range []uint32{rolMarco, rolDialogo, rolVentana} {
		if esRolObjetivo(rol) {
			t.Errorf("el rol %d es una ventana, no un objetivo", rol)
		}
	}
	if !tieneEstado([]uint32{1 << estadoActivo, 0}, estadoActivo) || tieneEstado([]uint32{0}, 40) {
		t.Error("tieneEstado")
	}
}

type a11yPrueba struct {
	marco rectA11y
	rs    []rectA11y
	err   error
	pid   int
}

func (a *a11yPrueba) Leer(_ context.Context, pid int, _ string) (rectA11y, []rectA11y, error) {
	a.pid = pid
	return a.marco, a.rs, a.err
}

func hyprPrueba(ventana string) func(context.Context, string) ([]byte, error) {
	return func(_ context.Context, c string) ([]byte, error) {
		switch c {
		case "activewindow":
			return []byte(ventana), nil
		case "monitors":
			return []byte(monitoresImanJSON), nil
		}
		return nil, errors.New("orden desconocida")
	}
}

func TestTargetsJuntaHyprlandYAccesibilidad(t *testing.T) {
	a := &a11yPrueba{marco: rectObjetivo(0, 0, 800, 600), rs: []rectA11y{rectObjetivo(10, 10, 30, 20), rectObjetivo(0, 0, 800, 600)}}
	o := objetivosHyprland{hypr: hyprPrueba(ventanaJSON), a11y: a}
	id, err := o.FocusedWindow(context.Background())
	if err != nil || id != 0x55d4c8a0b2f0 {
		t.Fatalf("FocusedWindow = %x, %v", id, err)
	}
	s, err := o.Targets(context.Background())
	if err != nil || s.Window != id || a.pid != 4242 || len(s.Targets) != 1 || s.LogicalWidth != 1440 {
		t.Fatalf("Targets = %+v, %v (pid %d)", s, err, a.pid)
	}

	// Con el táctil dirigido al segundo monitor, la ventana del primero
	// queda fuera.
	p := &punteroVirtual{}
	p.dirigir(&platform.Display{Name: "HDMI-A-1", X: 1440, Width: 1920, Height: 1080}, platform.Display{})
	o.puntero = p
	if s, _ := o.Targets(context.Background()); len(s.Targets) != 0 || s.LogicalWidth != 1920 {
		t.Errorf("hacia HDMI-A-1: %+v", s)
	}

	sin := objetivosHyprland{hypr: hyprPrueba("{}"), a11y: a}
	if _, err := sin.Targets(context.Background()); !errors.Is(err, platform.ErrSinVentana) {
		t.Errorf("sin ventana: %v", err)
	}
	a.err = errSinA11y
	if _, err := o.Targets(context.Background()); !errors.Is(err, errSinA11y) {
		t.Errorf("app sin accesibilidad: %v", err)
	}
}

// TestObjetivosVivo lee los objetivos de la ventana con el foco del
// escritorio (LAZUL_IMAN_VIVO=1, con un navegador delante) y dice cuánto
// tarda y cuánta CPU gasta este proceso. La app que contesta (el navegador)
// gasta aparte: se mira con top mientras corre.
func TestObjetivosVivo(t *testing.T) {
	if os.Getenv("LAZUL_IMAN_VIVO") == "" {
		t.Skip("LAZUL_IMAN_VIVO=1 para leer la ventana con el foco")
	}
	o := objetivosHyprland{hypr: consultarHyprland, a11y: &a11yDBus{}}
	if d := os.Getenv("LAZUL_IMAN_ESPERA"); d != "" {
		espera, _ := time.ParseDuration(d)
		t.Logf("pon delante la ventana a medir: %v", espera)
		time.Sleep(espera)
	}
	inicio := time.Now()
	v, err := o.FocusedWindow(context.Background())
	t.Logf("ventana %x en %v (%v)", v, time.Since(inicio), err)
	var total, peor time.Duration
	var cpu0 syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &cpu0)
	const n = 10
	for i := range n {
		inicio := time.Now()
		s, err := o.Targets(context.Background())
		d := time.Since(inicio)
		if err != nil {
			t.Fatal(err)
		}
		total += d
		peor = max(peor, d)
		if i == 0 {
			chicos := 0
			for _, x := range s.Targets {
				if math.Min(x.Width*s.LogicalWidth, x.Height*s.LogicalHeight) <= 48 {
					chicos++
				}
			}
			t.Logf("primera: %d objetivos (%d chicos), zona %.0fx%.0f lógicos, %v", len(s.Targets), chicos, s.LogicalWidth, s.LogicalHeight, d)
			for j, x := range s.Targets[:min(5, len(s.Targets))] {
				t.Logf("  %d: x=%.0f y=%.0f %.0fx%.0f", j, x.X*s.LogicalWidth, x.Y*s.LogicalHeight, x.Width*s.LogicalWidth, x.Height*s.LogicalHeight)
			}
		}
	}
	var cpu1 syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &cpu1)
	usado := time.Duration(cpu1.Utime.Nano()+cpu1.Stime.Nano()-cpu0.Utime.Nano()-cpu0.Stime.Nano()) / n
	t.Logf("media de %d lecturas: %v (peor %v), CPU de Lazul por lectura %v", n, total/n, peor, usado)
	t.Logf("a lo sumo una lectura cada 1,5 s: %.2f %% de un núcleo en Lazul", 100*float64(usado)/float64(1500*time.Millisecond))
}
