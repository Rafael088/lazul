package gestures

import "testing"

// La métrica no le cree al embrague: ve el cambio por dónde quedó el puntero.
func TestSaltosPunteroDelataElCambioDeMano(t *testing.T) {
	s := seguimientoSaltos{cal: panelCuadrado(t)}
	p := punteroApp{filtro: NuevoFiltroOneEuroPanel()}
	a, b := manoEnPanel(-.6, 0, .2, false), manoEnPanel(.6, 0, .2, false)
	b.Derecha = true
	for i, m := range []Mano{a, a, b, b, a} {
		mu, _ := MuestraPanelDe(m, 1)
		pos, _ := s.cal.Mapear(mu)
		r := ResultadoEmbrague{TMs: int64(i) * 33, TienePos: true, Dentro: true, X: pos.X, Y: pos.Y}
		p.cuadro(r)
		s.cuadro(cuadroPanel(r.TMs, a, b), r, &p)
	}
	if s.total.CambioDeMano != 2 || s.total.Relevo != 0 {
		t.Fatalf("saltos %+v, esperaba 2 cambios de mano", s.total)
	}
}

func TestSaltosPunteroSeparaElRelevo(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	s := seguimientoSaltos{cal: med.Calibracion}
	p := punteroApp{filtro: NuevoFiltroOneEuroPanel()}
	a, b := manoEnPanel(-.6, 0, .2, false), manoEnPanel(.6, 0, .2, false)
	b.Derecha = true
	for i := range int64(30) {
		c := cuadroPanel(i*33, a, b)
		if i >= 10 {
			c.Manos = c.Manos[1:] // la dueña se fue
		}
		r := med.Cuadro(c)
		p.cuadro(r)
		s.cuadro(c, r, &p)
	}
	if s.total.CambioDeMano != 0 || s.total.Relevo != 1 {
		t.Fatalf("saltos %+v, esperaba un relevo", s.total)
	}
}

func TestEmbragueOtroLadoEncimaNoEsRelevo(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	m := manoEnPanel(.3, 0, .2, false)
	med.Cuadro(cuadroPanel(0, m))
	m = manoEnPanel(.34, 0, .2, false)
	m.Derecha = true
	r := med.Cuadro(cuadroPanel(33, m))
	if r.Relevo || !r.Puntero.Derecha || r.X < .65 {
		t.Fatalf("el cambio de lado de una mano sola: %+v", r)
	}
}
