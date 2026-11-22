package gestures

import (
	"math"
	"testing"
)

// manoApertura: palma de 0,2 (de la muñeca al nudillo del medio) y puntas del
// pulgar e índice separadas por apertura*0,2, así la apertura medida es la
// pedida sin depender del aspecto.
func manoApertura(a float64) Mano {
	var m Mano
	m.Puntos[Muneca] = Punto{0.5, 0.5, 0}
	m.Puntos[MedioMCP] = Punto{0.5, 0.7, 0}
	m.Puntos[PulgarPunta] = Punto{0.5, 0.5, 0}
	m.Puntos[IndicePunta] = Punto{0.5, float32(0.5 + a*0.2), 0}
	return m
}

func TestAperturaPellizco(t *testing.T) {
	for _, a := range []float64{0, 0.15, 0.30, 0.45, 0.90} {
		got, ok := AperturaPellizco(manoApertura(a), 4.0/3)
		if !ok || math.Abs(got-a) > 1e-4 {
			t.Errorf("apertura %.2f: %v %v", a, got, ok)
		}
	}
	// Una palma de longitud cero no se puede medir.
	if _, ok := AperturaPellizco(Mano{}, 4.0/3); ok {
		t.Error("midió una mano degenerada")
	}
	// El aspecto corrige X: sin corrección, esta apertura saldría la mitad.
	var m = manoApertura(0.4)
	m.Puntos[PulgarPunta] = Punto{0.46, 0.5, 0}
	m.Puntos[IndicePunta] = Punto{0.54, 0.5, 0}
	if got, ok := AperturaPellizco(m, 2); !ok || math.Abs(got-0.8) > 1e-4 {
		t.Errorf("con aspecto 2: %v %v", got, ok)
	}
}

func TestPellizcoConHisteresis(t *testing.T) {
	// Cierra por debajo de 0,20, sigue cerrado hasta pasar 0,45 y vuelve a
	// abrirse; un cuadro en 0,35 no lo hace parpadear. Los dedos cerca sin
	// tocarse (0,22–0,28) ya no cierran (cotejo-lgbapm).
	medidor := NuevoMedidorPellizco(AjustesPellizcoPorDefecto)
	casos := []struct {
		apertura float64
		cerrado  bool
	}{
		{0.60, false}, {0.40, false}, {0.28, false}, {0.22, false}, {0.18, true}, {0.35, true},
		{0.44, true}, {0.46, false}, {0.15, true}, {0.30, true}, {0.60, false},
	}
	for i, c := range casos {
		estados := medidor.Cuadro([]Mano{manoApertura(c.apertura)}, 4.0/3)
		if len(estados) != 1 || estados[0].Cerrado != c.cerrado {
			t.Errorf("cuadro %d (apertura %.2f): %+v", i, c.apertura, estados)
		}
	}
}

func TestPellizcoDosManos(t *testing.T) {
	medidor := NuevoMedidorPellizco(AjustesPellizcoPorDefecto)
	derecha := manoApertura(0.15)
	derecha.Derecha = true
	izquierda := manoApertura(0.80)
	izquierda.Derecha = false

	estados := medidor.Cuadro([]Mano{derecha, izquierda}, 4.0/3)
	if !estados[0].Cerrado || estados[1].Cerrado {
		t.Fatalf("dos manos: %+v", estados)
	}
	// La izquierda se acerca: se cierra sin tocar la derecha.
	izquierda = manoApertura(0.18)
	izquierda.Derecha = false
	estados = medidor.Cuadro([]Mano{derecha, izquierda}, 4.0/3)
	if !estados[0].Cerrado || !estados[1].Cerrado {
		t.Fatalf("las dos cerradas: %+v", estados)
	}
	// Si desaparece la izquierda y vuelve, empieza abierta.
	estados = medidor.Cuadro([]Mano{derecha}, 4.0/3)
	if len(estados) != 1 || !estados[0].Cerrado {
		t.Fatalf("una mano: %+v", estados)
	}
	izquierda = manoApertura(0.80)
	izquierda.Derecha = false
	estados = medidor.Cuadro([]Mano{derecha, izquierda}, 4.0/3)
	if estados[1].Cerrado {
		t.Fatalf("la mano que vuelve no empieza abierta: %+v", estados)
	}
	// Y se cierra de nuevo al acercar las puntas.
	izquierda = manoApertura(0.18)
	izquierda.Derecha = false
	if estados = medidor.Cuadro([]Mano{derecha, izquierda}, 4.0/3); !estados[1].Cerrado {
		t.Fatalf("la mano que vuelve no se cierra: %+v", estados)
	}
}

func TestAnotarPellizcoNoTocaElOriginal(t *testing.T) {
	g := Grabacion{Cuadros: []Cuadro{
		{TMs: 0, Aspecto: 4.0 / 3, Manos: []Mano{manoApertura(0.60)}},
		{TMs: 33, Aspecto: 4.0 / 3, Manos: []Mano{manoApertura(0.15)}},
	}}
	anotada := AnotarPellizco(g)
	if anotada.Cuadros[1].Manos[0].Pellizco == nil || !anotada.Cuadros[1].Manos[0].Pellizco.Cerrado {
		t.Fatalf("no anotó: %+v", anotada.Cuadros)
	}
	if g.Cuadros[1].Manos[0].Pellizco != nil {
		t.Fatal("tocó los cuadros originales")
	}
	r := ResumirPellizco(anotada.Cuadros)
	if r.Cuadros != 2 || r.CuadrosConPellizco != 1 || r.Cierres != 1 || r.Manos != 2 {
		t.Fatalf("resumen: %+v", r)
	}
	if r.Fraccion() != 0.5 {
		t.Fatalf("fracción: %v", r.Fraccion())
	}
}

func TestResumirPellizcoCuentaCierres(t *testing.T) {
	cuadros := []Cuadro{
		{Manos: []Mano{{Pellizco: &EstadoPellizco{}}}},
		{Manos: []Mano{{Pellizco: &EstadoPellizco{Cerrado: true}}}},
		{Manos: []Mano{{Pellizco: &EstadoPellizco{Cerrado: true}}}},
		{Manos: []Mano{{Pellizco: &EstadoPellizco{}}}},
		{Manos: []Mano{{Pellizco: &EstadoPellizco{Cerrado: true}}}},
	}
	r := ResumirPellizco(cuadros)
	if r.CuadrosConPellizco != 3 || r.Cierres != 2 || r.Manos != 5 {
		t.Fatalf("resumen: %+v", r)
	}
}

func TestTomasPellizco(t *testing.T) {
	quiere := []string{
		"arrastrar-arriba", "arrastrar-abajo", "arrastrar-izquierda", "arrastrar-derecha",
		"entrar-panel", "salir-panel", "zoom-dos-manos", "normal",
		"tres-derecha", "tres-izquierda",
	}
	if len(TomasPellizco) != len(quiere) {
		t.Fatalf("tomas: %v", TomasPellizco)
	}
	for i, t2 := range quiere {
		if TomasPellizco[i] != t2 || !TomaPellizcoValida(t2) {
			t.Errorf("%q", t2)
		}
	}
	if TomaPellizcoValida("saludar") {
		t.Error("aceptó una toma que no existe")
	}
}

// AjustesPellizcoPara fija los umbrales a la mano del dueño con márgenes y
// topes: el dueño de las tomas (0,09) da los de por defecto, una mano que
// mide más sube los umbrales y una medida muy apretada no los deja por debajo
// de lo que da un pellizco en movimiento.
func TestAjustesPellizcoPara(t *testing.T) {
	cerca := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if aj := AjustesPellizcoPara(0.09); !cerca(aj.UmbralCerrar, AjustesPellizcoPorDefecto.UmbralCerrar) ||
		!cerca(aj.UmbralFirme, AjustesPellizcoPorDefecto.UmbralFirme) {
		t.Fatalf("0,09 debería dar los de por defecto: %+v", aj)
	}
	for _, m := range []float64{0.001, 0.04, 0.09, 0.15, 0.20, MaxPellizcoCerrado} {
		aj := AjustesPellizcoPara(m)
		if aj.UmbralCerrar < 0.18 || aj.UmbralCerrar > 0.30 || aj.UmbralFirme < 0.08 ||
			aj.UmbralFirme > aj.UmbralCerrar-0.06+1e-9 || aj.UmbralAbrir != AjustesPellizcoPorDefecto.UmbralAbrir {
			t.Errorf("%.3f: %+v", m, aj)
		}
		if m >= aj.UmbralCerrar {
			t.Errorf("%.3f: la mano calibrada no cerraría con %+v", m, aj)
		}
	}
	for _, m := range []float64{0, -0.1, MaxPellizcoCerrado + 0.01, math.NaN()} {
		if aj := AjustesPellizcoPara(m); aj != AjustesPellizcoPorDefecto {
			t.Errorf("%v debería dar los de por defecto: %+v", m, aj)
		}
	}
}
