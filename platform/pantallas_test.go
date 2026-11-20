package platform

import (
	"math"
	"testing"
)

// El escritorio de este equipo: el portátil (2880×1800 a escala 2) a la
// izquierda y la externa 1920×1080 a su derecha, en píxeles lógicos.
var dosPantallas = []Display{
	{Name: "eDP-1", X: 0, Y: 0, Width: 1440, Height: 900},
	{Name: "DP-1", X: 1440, Y: 0, Width: 1920, Height: 1080, Focused: true},
}

func TestVecino(t *testing.T) {
	casos := []struct {
		actual    string
		izquierda bool
		quiere    string
	}{
		{"eDP-1", false, "DP-1"},
		{"DP-1", true, "eDP-1"},
		{"eDP-1", true, ""},
		{"DP-1", false, ""},
		{"HDMI-A-1", false, ""}, // el actual ya no existe
	}
	for _, c := range casos {
		d, ok := Vecino(dosPantallas, c.actual, c.izquierda)
		if c.quiere == "" {
			if ok {
				t.Errorf("Vecino(%s, izq=%v) = %s, quiere ninguno", c.actual, c.izquierda, d.Name)
			}
			continue
		}
		if !ok || d.Name != c.quiere {
			t.Errorf("Vecino(%s, izq=%v) = %q %v, quiere %s", c.actual, c.izquierda, d.Name, ok, c.quiere)
		}
	}
}

func TestVecinoUnaPantalla(t *testing.T) {
	una := dosPantallas[:1]
	for _, izq := range []bool{true, false} {
		if d, ok := Vecino(una, "eDP-1", izq); ok {
			t.Fatalf("con un monitor hay vecino: %s", d.Name)
		}
	}
}

func TestVecinoTresPantallas(t *testing.T) {
	// Tres en fila y una encima de la del medio: a la derecha del medio va
	// la más cercana, y la de arriba (mismo centro) no cuenta como lado.
	ds := []Display{
		{Name: "A", X: 0, Y: 0, Width: 1000, Height: 1000},
		{Name: "B", X: 1000, Y: 0, Width: 1000, Height: 1000},
		{Name: "C", X: 2000, Y: 0, Width: 1000, Height: 1000},
		{Name: "D", X: 1000, Y: -800, Width: 1000, Height: 800},
	}
	if d, _ := Vecino(ds, "B", false); d.Name != "C" {
		t.Errorf("derecha de B = %s, quiere C", d.Name)
	}
	if d, _ := Vecino(ds, "B", true); d.Name != "A" {
		t.Errorf("izquierda de B = %s, quiere A", d.Name)
	}
	if d, _ := Vecino(ds, "A", false); d.Name != "B" {
		t.Errorf("derecha de A = %s, quiere B (la más cercana, no C ni D)", d.Name)
	}
}

func TestEnfocado(t *testing.T) {
	if d, _ := Enfocado(dosPantallas); d.Name != "DP-1" {
		t.Errorf("enfocado = %s", d.Name)
	}
	sinFoco := []Display{{Name: "X"}, {Name: "Y"}}
	if d, _ := Enfocado(sinFoco); d.Name != "X" {
		t.Errorf("sin foco marcado = %s, quiere el primero", d.Name)
	}
	if _, ok := Enfocado(nil); ok {
		t.Error("sin monitores hay enfocado")
	}
}

func cerca(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMapeoPorMonitor(t *testing.T) {
	caja := Abarque(dosPantallas)
	if caja != (Display{X: 0, Y: 0, Width: 3360, Height: 1080}) {
		t.Fatalf("abarque = %+v", caja)
	}
	casos := []struct {
		pantalla Display
		x, y     float64
		px, py   float64 // en el escritorio
		nx, ny   float64 // sobre el abarque
	}{
		{dosPantallas[0], 0, 0, 0, 0, 0, 0},
		{dosPantallas[0], 1, 1, 1440, 900, 1440.0 / 3360, 900.0 / 1080},
		{dosPantallas[0], .5, .5, 720, 450, 720.0 / 3360, 450.0 / 1080},
		{dosPantallas[1], 0, 0, 1440, 0, 1440.0 / 3360, 0},
		{dosPantallas[1], .5, .5, 2400, 540, 2400.0 / 3360, .5},
		{dosPantallas[1], 1, 1, 3360, 1080, 1, 1},
	}
	for _, c := range casos {
		px, py := c.pantalla.Map(c.x, c.y)
		if !cerca(px, c.px) || !cerca(py, c.py) {
			t.Errorf("%s.Map(%v,%v) = (%v,%v), quiere (%v,%v)", c.pantalla.Name, c.x, c.y, px, py, c.px, c.py)
		}
		nx, ny := EnAbarque(c.pantalla, caja, c.x, c.y)
		if !cerca(nx, c.nx) || !cerca(ny, c.ny) {
			t.Errorf("%s EnAbarque(%v,%v) = (%v,%v), quiere (%v,%v)", c.pantalla.Name, c.x, c.y, nx, ny, c.nx, c.ny)
		}
	}
}

func TestMapeoUnaPantallaNoCambia(t *testing.T) {
	una := dosPantallas[:1]
	caja := Abarque(una)
	for _, v := range [][2]float64{{0, 0}, {.3, .7}, {1, 1}} {
		nx, ny := EnAbarque(una[0], caja, v[0], v[1])
		if !cerca(nx, v[0]) || !cerca(ny, v[1]) {
			t.Errorf("un monitor: (%v,%v) → (%v,%v)", v[0], v[1], nx, ny)
		}
	}
	// Monitor desplazado (posición distinta de 0,0): sigue siendo identidad.
	d := Display{Name: "X", X: -1920, Y: 200, Width: 1920, Height: 1080}
	nx, ny := EnAbarque(d, Abarque([]Display{d}), .25, .75)
	if !cerca(nx, .25) || !cerca(ny, .75) {
		t.Errorf("desplazado: (%v,%v)", nx, ny)
	}
}
