package gestures

import (
	"math"
	"testing"
)

func muestraProyectada(x, y, tamano float64) MuestraPanel {
	return MuestraPanel{X: x * tamano, Y: y * tamano, Tamano: tamano}
}

func cercaPanel(t *testing.T, nombre string, obtuvo, quiere float64) {
	t.Helper()
	if math.Abs(obtuvo-quiere) > 1e-9 {
		t.Errorf("%s %.6f, quiere %.6f", nombre, obtuvo, quiere)
	}
}

func TestPanelMapeaDiagonalCentroYPerspectiva(t *testing.T) {
	// Las esquinas tienen distinto tamaño aparente: la inferior derecha se
	// tomó más cerca de la cámara. La proyección compensada sigue formando el
	// mismo rectángulo físico.
	// La imagen llega sin espejar: la izquierda de la persona está a la
	// derecha de la imagen, por eso X decrece entre ambas esquinas.
	si := muestraProyectada(1.0, -0.8, 0.20)
	id := muestraProyectada(-1.2, 0.9, 0.30)
	c, err := NuevaCalibracionPanel(si, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, caso := range []struct {
		nombre string
		m      MuestraPanel
		x, y   float64
	}{
		{"superior izquierda", si, 0, 0},
		{"inferior derecha", id, 1, 1},
		{"centro", muestraProyectada(-0.1, 0.05, 0.25), 0.5, 0.5},
	} {
		p, dentro := c.Mapear(caso.m)
		if !dentro {
			t.Fatalf("%s quedó fuera: %+v", caso.nombre, p)
		}
		cercaPanel(t, caso.nombre+" x", p.X, caso.x)
		cercaPanel(t, caso.nombre+" y", p.Y, caso.y)
	}
}

func TestPanelRechazaFueraDelRectanguloYDeLaProfundidad(t *testing.T) {
	c, err := NuevaCalibracionPanel(muestraProyectada(-1, -1, .2), muestraProyectada(1, 1, .2))
	if err != nil {
		t.Fatal(err)
	}
	if _, dentro := c.Mapear(muestraProyectada(1.3, 0, .2)); dentro {
		t.Error("aceptó una mano a la derecha del panel")
	}
	if p, dentro := c.Mapear(muestraProyectada(0, 0, .3)); dentro || math.Abs(p.X-.5) > 1e-9 {
		t.Errorf("aceptó una mano demasiado cerca: punto %+v, dentro %v", p, dentro)
	}
}

func TestPanelValidaLaOrientacionDeLasEsquinas(t *testing.T) {
	_, err := NuevaCalibracionPanel(muestraProyectada(-.1, -.1, .2), muestraProyectada(.1, .1, .2))
	if err == nil {
		t.Error("aceptó un panel diminuto")
	}
	_, err = NuevaCalibracionPanel(muestraProyectada(-1, .8, .2), muestraProyectada(1, -.8, .2))
	if err == nil {
		t.Error("aceptó las esquinas verticales al revés")
	}
}

func TestOneEuroReduceTemblorSinPerderElMovimiento(t *testing.T) {
	f := NuevoFiltroOneEuroPanel()
	var errorCrudo, errorFiltrado float64
	for i := 0; i < 90; i++ {
		ruido := 0.035
		if i%2 == 0 {
			ruido = -ruido
		}
		crudo := .5 + ruido
		p := f.Filtrar(int64(i*33), PuntoPanel{X: crudo, Y: crudo})
		if i > 15 {
			errorCrudo += math.Abs(crudo - .5)
			errorFiltrado += math.Abs(p.X - .5)
		}
	}
	if errorFiltrado >= errorCrudo*.4 {
		t.Fatalf("el temblor bajó poco: crudo %.4f, filtrado %.4f", errorCrudo, errorFiltrado)
	}
	var p PuntoPanel
	for i := 90; i < 99; i++ {
		p = f.Filtrar(int64(i*33), PuntoPanel{X: .9, Y: .9})
	}
	if p.X < .84 || p.X > .91 {
		t.Fatalf("el filtro no siguió un movimiento real en 300 ms: %.3f", p.X)
	}
}
