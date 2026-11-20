package gestures

import "testing"

// manoAbiertaReal saca la primera mano abierta de una grabación real de
// soltar: el barrido se prueba con una mano de verdad desplazada.
func manoAbiertaReal(t *testing.T) (Mano, float32) {
	t.Helper()
	gs, err := LeerDirectorio("../fixtures/gestos")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gs {
		for _, c := range g.Cuadros {
			if len(c.Manos) == 1 && PoseDe(c.Manos[0], c.Aspecto) == PoseAbierta {
				return c.Manos[0], c.Aspecto
			}
		}
	}
	t.Fatal("ninguna mano abierta en las grabaciones")
	return Mano{}, 0
}

func desplazar(m Mano, dx, dy float32) Mano {
	for i := range m.Puntos {
		m.Puntos[i].X += dx
		m.Puntos[i].Y += dy
	}
	return m
}

// recorrido arma n cuadros cada 33 ms que llevan la mano dx (en unidades de
// imagen) de una punta a otra.
func recorrido(m Mano, aspecto float32, t0 int64, n int, dx, dy float32) []Cuadro {
	cs := make([]Cuadro, n)
	for i := range cs {
		f := float32(i) / float32(n-1)
		cs[i] = Cuadro{TMs: t0 + int64(i)*33, Aspecto: aspecto, Manos: []Mano{desplazar(m, dx*f, dy*f)}}
	}
	return cs
}

func palmaImagen(m Mano, aspecto float32) float32 {
	// medir trabaja con X escalada por el aspecto; el desplazamiento va en X
	// cruda, así que una palma en X cruda es palma/aspecto.
	return float32(medir(m, aspecto).palma / aspectoDe(aspecto))
}

func barrer(d *DetectorBarridoPantalla, cs []Cuadro) []LadoPantalla {
	var lados []LadoPantalla
	for _, c := range cs {
		if l := d.Cuadro(c); l != PantallaNinguna {
			lados = append(lados, l)
		}
	}
	return lados
}

func TestBarridoPantallaLados(t *testing.T) {
	m, asp := manoAbiertaReal(t)
	p := palmaImagen(m, asp)
	// La imagen no se espeja: hacia la X menor es la derecha de la persona.
	casos := []struct {
		nombre string
		dx     float32
		quiere LadoPantalla
	}{
		{"a su derecha", -3.5 * p, PantallaDerecha},
		{"a su izquierda", 3.5 * p, PantallaIzquierda},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m := desplazar(m, -c.dx/2, 0) // centrado, para no salirse de la imagen
			lados := barrer(NuevoDetectorBarridoPantalla(), recorrido(m, asp, 0, 9, c.dx, 0))
			if len(lados) != 1 || lados[0] != c.quiere {
				t.Fatalf("lados = %v, quiere [%v]", lados, c.quiere)
			}
		})
	}
}

func TestBarridoPantallaNoDispara(t *testing.T) {
	m, asp := manoAbiertaReal(t)
	p := palmaImagen(m, asp)
	m = desplazar(m, -1.75*p, 0)
	pinza := m
	// Junta las puntas del pulgar y el índice: la pose pasa a pinza.
	pinza.Puntos[IndicePunta] = pinza.Puntos[PulgarPunta]
	if PoseDe(pinza, asp) != PosePinza {
		t.Fatalf("la mano de prueba no queda en pinza: %v", PoseDe(pinza, asp))
	}

	t.Run("lento", func(t *testing.T) {
		// El mismo recorrido en 1 s: eso es mover el puntero, no barrer.
		if l := barrer(NuevoDetectorBarridoPantalla(), recorrido(m, asp, 0, 31, 3.5*p, 0)); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("corto", func(t *testing.T) {
		if l := barrer(NuevoDetectorBarridoPantalla(), recorrido(m, asp, 0, 9, 1.5*p, 0)); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("diagonal", func(t *testing.T) {
		if l := barrer(NuevoDetectorBarridoPantalla(), recorrido(m, asp, 0, 9, 3.5*p, 2.5*p)); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("con pellizco", func(t *testing.T) {
		cs := recorrido(m, asp, 0, 9, 3.5*p, 0)
		cs[3].Manos[0] = desplazar(pinza, 3.5*p*3/8, 0)
		if l := barrer(NuevoDetectorBarridoPantalla(), cs); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("pinza arrastrada", func(t *testing.T) {
		cs := recorrido(pinza, asp, 0, 9, 3.5*p, 0)
		if l := barrer(NuevoDetectorBarridoPantalla(), cs); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("dos manos", func(t *testing.T) {
		cs := recorrido(m, asp, 0, 9, 3.5*p, 0)
		for i := range cs {
			cs[i].Manos = append(cs[i].Manos, desplazar(m, 0, 0.3))
		}
		if l := barrer(NuevoDetectorBarridoPantalla(), cs); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("salto del seguimiento", func(t *testing.T) {
		// La mano aparece de golpe 3,5 palmas más allá: no es un barrido.
		cs := []Cuadro{}
		for i := range 4 {
			cs = append(cs, Cuadro{TMs: int64(i) * 33, Aspecto: asp, Manos: []Mano{m}})
		}
		for i := range 4 {
			cs = append(cs, Cuadro{TMs: int64(4+i) * 33, Aspecto: asp, Manos: []Mano{desplazar(m, 3.5*p, 0)}})
		}
		if l := barrer(NuevoDetectorBarridoPantalla(), cs); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
	t.Run("vaiven", func(t *testing.T) {
		ida := recorrido(m, asp, 0, 5, 1.0*p, 0)
		vuelta := recorrido(desplazar(m, 1.0*p, 0), asp, 165, 3, -1.0*p, 0)
		ida2 := recorrido(m, asp, 264, 5, 3.0*p, 0)
		cs := append(append(ida, vuelta...), ida2[1:]...)
		if l := barrer(NuevoDetectorBarridoPantalla(), cs); len(l) != 0 {
			t.Fatalf("disparó %v", l)
		}
	})
}

func TestBarridoPantallaRefractario(t *testing.T) {
	m, asp := manoAbiertaReal(t)
	p := palmaImagen(m, asp)
	m = desplazar(m, 1.75*p, 0)
	d := NuevoDetectorBarridoPantalla()
	ida := recorrido(m, asp, 0, 9, -3.5*p, 0)
	// La vuelta enseguida (rebote de la mano) no devuelve el control.
	vuelta := recorrido(desplazar(m, -3.5*p, 0), asp, 400, 9, 3.5*p, 0)
	if l := barrer(d, append(ida, vuelta...)); len(l) != 1 || l[0] != PantallaDerecha {
		t.Fatalf("lados = %v, quiere [derecha]", l)
	}
	// Pasado el refractario, otro barrido sí cuenta.
	otra := recorrido(m, asp, 2000, 9, -3.5*p, 0)
	if l := barrer(d, otra); len(l) != 1 || l[0] != PantallaDerecha {
		t.Fatalf("segundo barrido: %v", l)
	}
}

// TestBarridoPantallaSinFalsos pasa todas las grabaciones reales por el
// detector: ninguna tiene un barrido de pantalla, así que nada dispara. Con
// el umbral en 1,5 palmas disparan 7 (señalar, entrar al panel, soltar,
// movimiento normal); desde 2,0, ninguna.
func TestBarridoPantallaSinFalsos(t *testing.T) {
	var gs []Grabacion
	for _, dir := range []string{"../fixtures/gestos", "../fixtures/pellizco"} {
		g, err := LeerDirectorio(dir)
		if err != nil {
			t.Fatal(err)
		}
		gs = append(gs, g...)
	}
	if len(gs) < 100 {
		t.Fatalf("solo %d grabaciones", len(gs))
	}
	for _, g := range gs {
		d := NuevoDetectorBarridoPantalla()
		for _, c := range g.Cuadros {
			if l := d.Cuadro(c); l != PantallaNinguna {
				t.Errorf("%s: barrido %v a los %d ms", g.Archivo, l, c.TMs)
			}
		}
	}
}
