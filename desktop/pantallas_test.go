package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// pantallasPrueba es un escritorio falso. moverFoco imita a Hyprland (Target
// lleva el foco); sin él, a Windows (el foco se queda donde estaba).
type pantallasPrueba struct {
	ds        []platform.Display
	targets   []string
	moverFoco bool
}

func (f *pantallasPrueba) List(context.Context) ([]platform.Display, error) {
	return append([]platform.Display(nil), f.ds...), nil
}

func (f *pantallasPrueba) Target(_ context.Context, name string) error {
	f.targets = append(f.targets, name)
	if f.moverFoco {
		f.enfocar(name)
	}
	return nil
}

func (f *pantallasPrueba) enfocar(name string) {
	for i := range f.ds {
		f.ds[i].Focused = f.ds[i].Name == name
	}
}

func dosMonitores(foco string) []platform.Display {
	ds := []platform.Display{
		{Name: "eDP-1", X: 0, Y: 0, Width: 1440, Height: 900},
		{Name: "DP-1", X: 1440, Y: 0, Width: 1920, Height: 1080},
	}
	for i := range ds {
		ds[i].Focused = ds[i].Name == foco
	}
	return ds
}

var (
	manoAbiertaUna  sync.Once
	manoAbiertaGuar gestures.Mano
	manoAbiertaAsp  float32
)

// manoAbierta saca una mano abierta real de las grabaciones.
func manoAbierta(t *testing.T) (gestures.Mano, float32) {
	t.Helper()
	manoAbiertaUna.Do(func() { manoAbiertaGuar, manoAbiertaAsp = buscarManoAbierta(t) })
	return manoAbiertaGuar, manoAbiertaAsp
}

func buscarManoAbierta(t *testing.T) (gestures.Mano, float32) {
	gs, err := gestures.LeerDirectorio("../fixtures/gestos")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gs {
		for _, c := range g.Cuadros {
			if len(c.Manos) == 1 && gestures.PoseDe(c.Manos[0], c.Aspecto) == gestures.PoseAbierta {
				return c.Manos[0], c.Aspecto
			}
		}
	}
	t.Fatal("ninguna mano abierta")
	return gestures.Mano{}, 0
}

// barrido arma un barrido rápido (9 cuadros, 264 ms) de la mano: hacia la
// derecha de la persona la mano va a la X menor (imagen sin espejar).
func barrido(t *testing.T, t0 int64, derecha bool) []gestures.Cuadro {
	m, asp := manoAbierta(t)
	dx := float32(0.35)
	if derecha {
		dx = -dx
	}
	cs := make([]gestures.Cuadro, 9)
	for i := range cs {
		f := float32(i)/8 - 0.5
		mano := m
		for k := range mano.Puntos {
			mano.Puntos[k].X += dx * f
		}
		cs[i] = gestures.Cuadro{TMs: t0 + int64(i)*33, Aspecto: asp, Manos: []gestures.Mano{mano}}
	}
	return cs
}

func pasarPantallas(p *Pantallas, cs []gestures.Cuadro, libre bool, t0 time.Time) int {
	cambios := 0
	for _, c := range cs {
		if p.Cuadro(c, libre, t0.Add(time.Duration(c.TMs)*time.Millisecond)) {
			cambios++
		}
	}
	return cambios
}

func TestPantallasUnMonitorNoCambiaNada(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1")[:1]}
	p := NuevasPantallas(f)
	var avisos []string
	p.AlCambiar = func(d platform.Display) { avisos = append(avisos, d.Name) }
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if n := pasarPantallas(p, barrido(t, 0, true), true, t0); n != 0 {
		t.Fatalf("con un monitor el barrido cambió %d veces", n)
	}
	if len(f.targets) != 0 {
		t.Fatalf("con un monitor se llamó a Target: %v", f.targets)
	}
	if d, ok := p.Actual(); !ok || d.Name != "eDP-1" || len(avisos) != 1 {
		t.Fatalf("actual = %v %v, avisos %v", d.Name, ok, avisos)
	}
}

func TestPantallasSinDisplays(t *testing.T) {
	p := NuevasPantallas(nil)
	p.Iniciar(time.Unix(0, 0))
	if pasarPantallas(p, barrido(t, 0, true), true, time.Unix(0, 0)) != 0 {
		t.Fatal("sin Displays hubo cambio")
	}
	if _, ok := p.Actual(); ok {
		t.Fatal("sin Displays hay monitor actual")
	}
}

func TestPantallasArrancaEnElFocoYBarre(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1"), moverFoco: true}
	dir := t.TempDir()
	reg, err := AbrirRegistro(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	p := NuevasPantallas(f)
	p.Reg = reg
	var avisos []string
	p.AlCambiar = func(d platform.Display) { avisos = append(avisos, d.Name) }
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if strings.Join(f.targets, ",") != "eDP-1" {
		t.Fatalf("al iniciar, targets = %v; quiere el del foco", f.targets)
	}
	// Barrido a la derecha de la persona: pasa a la externa.
	if n := pasarPantallas(p, barrido(t, 0, true), true, t0); n != 1 {
		t.Fatalf("barrido a la derecha: %d cambios", n)
	}
	if d, _ := p.Actual(); d.Name != "DP-1" {
		t.Fatalf("actual = %s, quiere DP-1", d.Name)
	}
	// Otro a la derecha (pasado el refractario): no hay nada más allá.
	if n := pasarPantallas(p, barrido(t, 3000, true), true, t0); n != 0 {
		t.Fatalf("sin vecino a la derecha cambió %d veces", n)
	}
	// Y a la izquierda vuelve al portátil.
	if n := pasarPantallas(p, barrido(t, 6000, false), true, t0); n != 1 {
		t.Fatalf("barrido a la izquierda: %d cambios", n)
	}
	if strings.Join(f.targets, ",") != "eDP-1,DP-1,eDP-1" || strings.Join(avisos, ",") != "eDP-1,DP-1,eDP-1" {
		t.Fatalf("targets = %v, avisos = %v", f.targets, avisos)
	}
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, quiere := range []string{`"tipo":"pantalla","dir":"derecha","resultado":"barrido"`, `"dir":"derecha","resultado":"sin_vecino"`, `"dir":"izquierda","resultado":"barrido"`} {
		if !strings.Contains(s, quiere) {
			t.Errorf("el registro no tiene %s:\n%s", quiere, s)
		}
	}
	if strings.Contains(s, `"x":`) || strings.Contains(s, "1440") {
		t.Errorf("el registro guarda posiciones o geometría: %s", s)
	}
}

func TestPantallasAgarrandoNoCambia(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1"), moverFoco: true}
	p := NuevasPantallas(f)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if n := pasarPantallas(p, barrido(t, 0, true), false, t0); n != 0 {
		t.Fatalf("agarrando, el barrido cambió %d veces", n)
	}
	// La mitad del barrido agarrando y la otra mitad libre tampoco basta.
	cs := barrido(t, 3000, true)
	if n := pasarPantallas(p, cs[:5], false, t0) + pasarPantallas(p, cs[5:], true, t0); n != 0 {
		t.Fatalf("medio barrido agarrando cambió %d veces", n)
	}
}

func TestPantallasSigueAlFoco(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1")}
	p := NuevasPantallas(f)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	quieto := []gestures.Cuadro{{TMs: 0}}
	// La persona hace clic en la externa: al siguiente vistazo, el panel va allí.
	f.enfocar("DP-1")
	if p.Cuadro(quieto[0], true, t0.Add(500*time.Millisecond)) {
		t.Fatal("se consultó el foco antes de ConsultarFocoCada")
	}
	if !p.Cuadro(quieto[0], true, t0.Add(ConsultarFocoCada)) {
		t.Fatal("no siguió al foco")
	}
	if d, _ := p.Actual(); d.Name != "DP-1" {
		t.Fatalf("actual = %s", d.Name)
	}
}

func TestPantallasBarridoSinMoverFocoNoSeDeshace(t *testing.T) {
	// Como en Windows: Target no mueve el foco. El seguimiento no debe
	// devolver el control al monitor del foco tras el barrido.
	f := &pantallasPrueba{ds: dosMonitores("eDP-1")}
	p := NuevasPantallas(f)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if pasarPantallas(p, barrido(t, 0, true), true, t0) != 1 {
		t.Fatal("el barrido no cambió")
	}
	for i := 1; i <= 3; i++ {
		p.Cuadro(gestures.Cuadro{TMs: int64(i) * 1000}, true, t0.Add(time.Duration(i)*1100*time.Millisecond))
	}
	if d, _ := p.Actual(); d.Name != "DP-1" {
		t.Fatalf("el seguimiento del foco deshizo el barrido: actual = %s", d.Name)
	}
}

type pantallasRotas struct{ listas int }

func (f *pantallasRotas) List(context.Context) ([]platform.Display, error) {
	f.listas++
	return nil, errors.New("hyprctl: no hay Hyprland")
}
func (f *pantallasRotas) Target(context.Context, string) error { return errors.New("no") }

func TestPantallasSinCompositorAnotaUnaVez(t *testing.T) {
	dir := t.TempDir()
	reg, err := AbrirRegistro(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	f := &pantallasRotas{}
	p := NuevasPantallas(f)
	p.Reg = reg
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	for i := 1; i <= 5; i++ {
		if p.Cuadro(gestures.Cuadro{}, true, t0.Add(time.Duration(i)*ConsultarFocoCada)) {
			t.Fatal("sin monitores hubo cambio")
		}
	}
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, NombreRegistro))
	if n := strings.Count(string(b), "error_listar"); n != 1 || f.listas != 6 {
		t.Fatalf("error_listar %d veces en %d consultas:\n%s", n, f.listas, b)
	}
}
