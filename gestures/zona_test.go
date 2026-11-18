package gestures

import "testing"

func cuerpoPrueba() *Cuerpo {
	var c Cuerpo
	for _, p := range []struct {
		i    int
		x, y float32
	}{
		{HombroIzquierdo, .30, .25},
		{HombroDerecho, .70, .25},
		{CaderaIzquierda, .35, .75},
		{CaderaDerecha, .65, .75},
	} {
		c.Puntos[p.i] = PuntoCuerpo{X: p.x, Y: p.y, Visibilidad: .99}
	}
	return &c
}

func manoEn(x, y float32) Mano {
	var m Mano
	for _, i := range []int{Muneca, IndiceMCP, MedioMCP, AnularMCP, MeniqueMCP} {
		m.Puntos[i] = Punto{X: x, Y: y}
	}
	return m
}

func TestFiltrarPorTorsoSoloDejaLaPalmaDelante(t *testing.T) {
	c := Cuadro{Aspecto: 4.0 / 3.0, Cuerpo: cuerpoPrueba(), Manos: []Mano{
		manoEn(.50, .50), // delante del torso
		manoEn(.50, .10), // rascándose la cabeza
		manoEn(.90, .45), // señalando a un lado
		manoEn(.50, .90), // brazo abajo
	}}
	filtrado, rechazadas := FiltrarPorTorso(c)
	if len(filtrado.Manos) != 1 || rechazadas != 3 {
		t.Fatalf("manos dentro %d, rechazadas %d", len(filtrado.Manos), rechazadas)
	}
	if filtrado.Manos[0].Puntos[Muneca].Y != .50 {
		t.Fatalf("dejó la mano equivocada: %+v", filtrado.Manos[0])
	}
}

func TestFiltrarPorTorsoFallaCerradoSinPoseFiable(t *testing.T) {
	c := Cuadro{Manos: []Mano{manoEn(.5, .5)}}
	if filtrado, rechazadas := FiltrarPorTorso(c); len(filtrado.Manos) != 0 || rechazadas != 1 {
		t.Fatalf("sin cuerpo: manos %d, rechazadas %d", len(filtrado.Manos), rechazadas)
	}

	c.Cuerpo = cuerpoPrueba()
	c.Cuerpo.Puntos[HombroIzquierdo].Visibilidad = .2
	if filtrado, rechazadas := FiltrarPorTorso(c); len(filtrado.Manos) != 0 || rechazadas != 1 {
		t.Fatalf("pose dudosa: manos %d, rechazadas %d", len(filtrado.Manos), rechazadas)
	}
}
