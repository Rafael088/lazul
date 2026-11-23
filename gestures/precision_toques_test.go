package gestures

import (
	"path/filepath"
	"testing"
)

func TestJuzgarToma(t *testing.T) {
	arr := func(x0, y0, x1, y1 float64) Toque {
		return Toque{Inicio: PuntoPanel{X: x0, Y: y0}, Fin: PuntoPanel{X: x1, Y: y1}, FinMs: 600, Tipo: ToqueArrastre}
	}
	casos := []struct {
		nombre           string
		toma             TomaToques
		intento, acierto bool
		falsos, deMas    int
	}{
		{"arrastre en la dirección pedida", TomaToques{Toma: "arrastrar-abajo", Toques: []Toque{arr(.5, .2, .5, .6)}}, true, true, 0, 0},
		{"arrastre al revés", TomaToques{Toma: "arrastrar-arriba", Toques: []Toque{arr(.5, .2, .5, .6)}}, true, false, 0, 0},
		{"un corte deja dos toques", TomaToques{Toma: "arrastrar-abajo", Toques: []Toque{arr(.5, .2, .5, .4), arr(.5, .4, .5, .6)}}, true, false, 0, 1},
		{"arrastre sin toque", TomaToques{Toma: "arrastrar-derecha"}, true, false, 0, 0},
		{"las tomas de zoom ya no piden gesto", TomaToques{Toma: "zoom-dos-manos", Toques: []Toque{
			{Ranura: 0, InicioMs: 0, FinMs: 900}}}, false, false, 0, 0},
		{"toque en movimiento normal es falso", TomaToques{Toma: "normal", Toques: []Toque{{Tipo: ToqueTap}}}, false, false, 1, 0},
		{"salir sin toques", TomaToques{Toma: "salir-panel"}, false, false, 0, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in, ac, f, dm := JuzgarToma(c.toma)
			if in != c.intento || ac != c.acierto || f != c.falsos || dm != c.deMas {
				t.Fatalf("obtuvo intento=%v acierto=%v falsos=%d de más=%d", in, ac, f, dm)
			}
		})
	}
}

func TestEjeDeSalida(t *testing.T) {
	casos := []struct {
		p        PuntoPanel
		eje      int
		alto, ok bool
	}{
		{PuntoPanel{X: 1.3, Y: .5}, 0, true, true},
		{PuntoPanel{X: -.2, Y: .5}, 0, false, true},
		{PuntoPanel{X: .5, Y: 1.2}, 1, true, true},
		{PuntoPanel{X: .5, Y: -.4}, 1, false, true},
		{PuntoPanel{X: .5, Y: .5}, 0, false, false},  // salió por distancia
		{PuntoPanel{X: 1.1, Y: .5}, 0, false, false}, // sigue en la banda
		{PuntoPanel{X: 1.2, Y: 1.4}, 1, true, true},  // manda el eje que más rebasa
	}
	for _, c := range casos {
		eje, alto, ok := ejeDeSalida(c.p)
		if ok != c.ok || (ok && (eje != c.eje || alto != c.alto)) {
			t.Fatalf("%+v: eje %d alto %v ok %v", c.p, eje, alto, ok)
		}
	}
}

// TestPrecisionConTomasReales fija lo que no debe romperse de la métrica con
// las tomas reales: cuentas coherentes y que se vean salidas por los bordes.
// Las cifras se leen con ./dev replay -toques.
func TestPrecisionConTomasReales(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "pellizco"))
	if err != nil {
		t.Fatal(err)
	}
	tomas, err := MedirToques(gs)
	if err != nil {
		t.Fatal(err)
	}
	p := PrecisionDe(tomas)
	if p.Intentos+p.SinPellizco != len(tomas) {
		t.Fatalf("intentos %d + sin pellizco %d != %d tomas", p.Intentos, p.SinPellizco, len(tomas))
	}
	if p.Aciertos > p.Intentos || p.Bordes > p.BordesPosibles {
		t.Fatalf("cuentas incoherentes: %+v", p)
	}
	if p.BordesPosibles < 10 || len(p.Saltos) < 30 {
		t.Fatalf("pocas salidas por un borde (%d) o pocos apoyos (%d)", p.BordesPosibles, len(p.Saltos))
	}
}
