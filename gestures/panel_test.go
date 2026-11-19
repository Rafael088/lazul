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

func TestMapeadorPanelBandaHistéresisYGanancia(t *testing.T) {
	c, err := NuevaCalibracionPanel(muestraProyectada(-1, -1, .2), muestraProyectada(1, 1, .2))
	if err != nil {
		t.Fatal(err)
	}
	// x está en la proyección [-1,1] del panel: -0,9 y 0,9 son el 5 % y
	// 95 %, que con la ganancia deben llegar exactamente a la pantalla.
	casos := []struct {
		nombre         string
		x              float64
		dentro         bool
		pantallaX      float64
		reiniciarAntes bool
	}{
		{"no entra desde la banda", -1.10, false, 0, false},
		{"entra por el borde real", -1.00, true, 0, false},
		{"cinco por ciento llega a cero", -0.90, true, 0, false},
		{"noventa y cinco por ciento llega a uno", 0.90, true, 1, false},
		{"permanece diez por ciento fuera", 1.20, true, 1, false},
		{"sale más allá del quince por ciento", 1.31, false, 1, false},
		{"no reentra por la banda", 1.10, false, 1, false},
		{"reentra al cruzar el borde", 1.00, true, 1, false},
		{"reiniciada tampoco entra por la banda", -1.10, false, 0, true},
	}
	mapa := NuevoMapeadorPanel(c)
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if caso.reiniciarAntes {
				mapa.Reiniciar()
			}
			p, dentro := mapa.Mapear(muestraProyectada(caso.x, 0, .2))
			if dentro != caso.dentro || math.Abs(p.X-caso.pantallaX) > 1e-9 {
				t.Fatalf("x %.2f: punto %+v, dentro %v; quería x %.2f, dentro %v", caso.x, p, dentro, caso.pantallaX, caso.dentro)
			}
		})
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

func TestOneEuroAlcanzaLosExtremos(t *testing.T) {
	f := NuevoFiltroOneEuroPanel()
	f.Filtrar(0, PuntoPanel{X: .5, Y: .5})
	for _, caso := range []struct {
		nombre string
		t      int64
		p      PuntoPanel
	}{
		{"cero", 33, PuntoPanel{X: 0, Y: 0}},
		{"uno", 66, PuntoPanel{X: 1, Y: 1}},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			obtuvo := f.Filtrar(caso.t, caso.p)
			if obtuvo != caso.p {
				t.Fatalf("el filtro dejó corto el extremo: obtuvo %+v, quería %+v", obtuvo, caso.p)
			}
		})
	}
}

func TestDiagnosticarDiceElMotivo(t *testing.T) {
	cal, err := NuevaCalibracionPanel(
		MuestraPanel{X: -.2, Y: -.2, Tamano: .1},
		MuestraPanel{X: .2, Y: .2, Tamano: .1})
	if err != nil {
		t.Fatal(err)
	}
	a := 4.0 / 3.0
	casos := []struct {
		m      MuestraPanel
		motivo string
	}{
		{MuestraPanel{X: 0, Y: 0, Tamano: .1}, "dentro"},
		{MuestraPanel{X: .35, Y: 0, Tamano: .1}, "fuera_borde"},
		{MuestraPanel{X: 0, Y: 0, Tamano: .16}, "cerca"},
		{MuestraPanel{X: 0, Y: 0, Tamano: .05}, "lejos"},
	}
	for _, c := range casos {
		d, ok := cal.Diagnosticar(c.m, a)
		if !ok || d.Motivo != c.motivo {
			t.Errorf("%+v: motivo %q (ok %v), quería %q", c.m, d.Motivo, ok, c.motivo)
		}
		if _, dentro := cal.Mapear(c.m); dentro != d.Dentro {
			t.Errorf("%+v: Dentro %v distinto de Mapear %v", c.m, d.Dentro, dentro)
		}
	}
	// A la distancia de la calibración, el rectángulo coincide con las esquinas.
	d, _ := cal.Diagnosticar(MuestraPanel{X: 0, Y: 0, Tamano: .1}, a)
	x0, y0, x1, y1 := cal.RectanguloCalibrado(a)
	if math.Abs(d.X0-x0) > 1e-9 || math.Abs(d.Y0-y0) > 1e-9 || math.Abs(d.X1-x1) > 1e-9 || math.Abs(d.Y1-y1) > 1e-9 {
		t.Fatalf("rectángulo %v,%v,%v,%v contra calibrado %v,%v,%v,%v", d.X0, d.Y0, d.X1, d.Y1, x0, y0, x1, y1)
	}
	if !(d.BX0 < d.X0 && d.BY0 < d.Y0 && d.BX1 > d.X1 && d.BY1 > d.Y1) {
		t.Fatalf("la banda no rodea el panel: %+v", d)
	}
}
