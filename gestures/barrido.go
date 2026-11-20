package gestures

import "math"

// LadoPantalla es hacia dónde pasa el control con dos pantallas, visto por la
// persona: su izquierda o su derecha (la imagen llega sin espejar).
type LadoPantalla int

const (
	PantallaNinguna LadoPantalla = iota
	PantallaIzquierda
	PantallaDerecha
)

func (l LadoPantalla) String() string {
	switch l {
	case PantallaIzquierda:
		return "izquierda"
	case PantallaDerecha:
		return "derecha"
	}
	return "ninguna"
}

// AjustesBarridoPantalla fija cuándo un movimiento de la mano abierta cuenta
// como barrido para cambiar de pantalla. Es el mismo movimiento que suelta en
// el clasificador (mano abierta hacia un lado), pero más largo y rápido: el
// de soltar llega a 2,7 palmas en 400 ms y entrar al panel, a 2,0.
type AjustesBarridoPantalla struct {
	Ventana     int64   // ms en que se mide el recorrido
	Umbral      float64 // recorrido horizontal mínimo, en palmas
	Dominio     float64 // |dx| debe ser al menos Dominio·|dy|
	Abierta     float64 // fracción mínima de cuadros con la mano abierta
	MinCuadros  int     // cuadros mínimos dentro de la ventana
	SaltoMax    float64 // un paso entre cuadros mayor que esto (palmas) es un salto del seguimiento
	Refractario int64   // ms sin otro barrido tras uno
}

// AjustesBarridoPorDefecto salen de medir las grabaciones reales con
// TestBarridoPantallaSinFalsos: ningún negativo (pellizco, arrastre, entrar y
// salir del panel, zoom, señalar, movimiento normal ni gestos 0.1) dispara.
var AjustesBarridoPorDefecto = AjustesBarridoPantalla{
	Ventana:     350,
	Umbral:      2.5,
	Dominio:     2.5,
	Abierta:     0.8,
	MinCuadros:  4,
	SaltoMax:    1.5,
	Refractario: 1000,
}

type muestraBarrido struct {
	t       int64
	centro  vec
	palma   float64
	abierta bool
	pinza   bool
}

// DetectorBarridoPantalla reconoce, cuadro a cuadro, el barrido de una sola
// mano abierta hacia un lado. Con dos manos o sin mano olvida lo visto: dos
// manos son el zoom. Un pellizco en la ventana lo anula: la pinza arrastra.
type DetectorBarridoPantalla struct {
	Aj       AjustesBarridoPantalla
	hist     []muestraBarrido
	ultimo   int64
	disparos int
}

// NuevoDetectorBarridoPantalla usa AjustesBarridoPorDefecto.
func NuevoDetectorBarridoPantalla() *DetectorBarridoPantalla {
	return &DetectorBarridoPantalla{Aj: AjustesBarridoPorDefecto}
}

// Reiniciar olvida el recorrido (por ejemplo, mientras el pellizco agarra).
func (d *DetectorBarridoPantalla) Reiniciar() { d.hist = d.hist[:0] }

// Cuadro devuelve el lado del barrido cuando acaba de completarse y
// PantallaNinguna en los demás cuadros.
func (d *DetectorBarridoPantalla) Cuadro(c Cuadro) LadoPantalla {
	if len(c.Manos) != 1 {
		d.Reiniciar()
		return PantallaNinguna
	}
	r := medir(c.Manos[0], c.Aspecto)
	if r.palma < 1e-6 {
		d.Reiniciar()
		return PantallaNinguna
	}
	m := muestraBarrido{t: c.TMs, centro: r.centro, palma: r.palma, abierta: r.pose == PoseAbierta, pinza: r.pose == PosePinza}
	if n := len(d.hist); n > 0 {
		ant := d.hist[n-1]
		if m.t < ant.t || dist(m.centro, ant.centro) > d.Aj.SaltoMax*(m.palma+ant.palma)/2 {
			// Salto del seguimiento (o reloj hacia atrás): empezar de nuevo.
			d.Reiniciar()
		}
	}
	d.hist = append(d.hist, m)
	i := 0
	for i < len(d.hist) && m.t-d.hist[i].t > d.Aj.Ventana {
		i++
	}
	d.hist = d.hist[i:]
	if d.disparos > 0 && m.t-d.ultimo < d.Aj.Refractario {
		return PantallaNinguna
	}
	if !m.abierta || len(d.hist) < d.Aj.MinCuadros {
		return PantallaNinguna
	}
	abiertas, palma := 0, 0.0
	for _, s := range d.hist {
		if s.pinza {
			return PantallaNinguna
		}
		if s.abierta {
			abiertas++
		}
		palma += s.palma
	}
	palma /= float64(len(d.hist))
	if float64(abiertas) < d.Aj.Abierta*float64(len(d.hist)) {
		return PantallaNinguna
	}
	dx := m.centro.x - d.hist[0].centro.x
	dy := m.centro.y - d.hist[0].centro.y
	if math.Abs(dx) < d.Aj.Umbral*palma || math.Abs(dx) < d.Aj.Dominio*math.Abs(dy) {
		return PantallaNinguna
	}
	// El recorrido tiene que ir siempre hacia el mismo lado: un vaivén
	// (saludar, gesticular) no cambia de pantalla.
	for k := 1; k < len(d.hist); k++ {
		paso := d.hist[k].centro.x - d.hist[k-1].centro.x
		if paso*dx < -0.25*palma {
			return PantallaNinguna
		}
	}
	d.ultimo, d.disparos = m.t, d.disparos+1
	d.Reiniciar()
	if dx < 0 { // imagen sin espejar: es la derecha de la persona
		return PantallaDerecha
	}
	return PantallaIzquierda
}
