package gestures

import "math"

// Saltos del puntero con dos manos (cotejo-goem2w). No le pregunta al
// embrague de qué mano es el puntero: lo deduce de dónde quedó, así la cifra
// delata cualquier regla que lo vuelva a pasar de una mano a otra.

// SaltoPunteroMin es cuánto se tiene que mover el puntero de un cuadro al
// siguiente, en pantallas, para contar como salto.
const SaltoPunteroMin = 0.2

// SaltosPuntero reparte los saltos del puntero con dos manos en el cuadro o
// en el anterior.
type SaltosPuntero struct {
	Cuadros int // cuadros con dos manos en el panel, o justo después
	// CambioDeMano: pasó a otra con la suya todavía a la vista, o perdida
	// hace menos de lo que tarda en soltar un pellizco. Tiene que ser 0.
	CambioDeMano int
	// Relevo: pasó a otra porque la suya se fue de verdad o salió del panel.
	Relevo int
	// MismaClave: saltó sin cambiar de clave. Es la mano que se movió o, en
	// el orden crudo de la cámara, dos manos que se intercambian el lado.
	MismaClave int
	// Filtrado: saltos que siguen en el puntero ya pasado por el One Euro.
	Filtrado int
}

// SaltosPunteroDe suma los de todas las tomas.
func SaltosPunteroDe(tomas []TomaToques) SaltosPuntero {
	var s SaltosPuntero
	for _, t := range tomas {
		s.Cuadros += t.SaltosPuntero.Cuadros
		s.CambioDeMano += t.SaltosPuntero.CambioDeMano
		s.Relevo += t.SaltosPuntero.Relevo
		s.MismaClave += t.SaltosPuntero.MismaClave
		s.Filtrado += t.SaltosPuntero.Filtrado
	}
	return s
}

type seguimientoSaltos struct {
	cal   CalibracionPanel
	total SaltosPuntero

	n                int
	vista            map[ClaveMano]vistaMano
	clave            ClaveMano // la mano en la que estaba el puntero
	crudo, filtrado  PuntoPanel
	hay, hayFiltrado bool
	manos            int
}

type vistaMano struct {
	n int
	t int64
}

type manoUbicada struct {
	clave ClaveMano
	p     PuntoPanel
}

func (s *seguimientoSaltos) cuadro(c Cuadro, r ResultadoEmbrague, p *punteroApp) {
	if s.vista == nil {
		s.vista = map[ClaveMano]vistaMano{}
	}
	s.n++
	ahora := PuntoPanel{X: acotar01(r.X), Y: acotar01(r.Y)}
	con2 := len(c.Manos) >= 2 || s.manos >= 2
	if con2 {
		s.total.Cuadros++
	}

	// Dónde está cada mano: su dedo si agarra, su palma si no.
	var donde []manoUbicada
	vistos := map[bool]int{}
	for _, mano := range c.Manos {
		clave := ClaveMano{Derecha: mano.Derecha, N: vistos[mano.Derecha]}
		vistos[mano.Derecha]++
		muestra, ok := MuestraPanelDe(mano, c.Aspecto)
		if !ok {
			continue
		}
		pos, _ := s.cal.Mapear(muestra)
		for _, a := range r.Armadas {
			if a.Clave == clave {
				pos = PuntoPanel{X: acotar01(a.X), Y: acotar01(a.Y)}
			}
		}
		donde = append(donde, manoUbicada{clave, pos})
	}
	clave, mejor := s.clave, math.Inf(1)
	for _, d := range donde {
		if x := math.Hypot(d.p.X-ahora.X, d.p.Y-ahora.Y); x < mejor {
			clave, mejor = d.clave, x
		}
	}
	// Quieto esperando a la suya, no está en ninguna de las que se ven.
	if mejor > SaltoPunteroMin {
		clave = s.clave
	}

	if r.TienePos && s.hay && math.Hypot(ahora.X-s.crudo.X, ahora.Y-s.crudo.Y) > SaltoPunteroMin {
		v, habia := s.vista[s.clave]
		presente := false
		for _, d := range donde {
			presente = presente || d.clave == s.clave
		}
		aj := AjustesEmbraguePorDefecto
		seFue := habia && !presente && s.n-v.n >= aj.CuadrosParaSoltar && r.TMs-v.t >= aj.MsParaSoltar
		// El relevo llega cuando ya solo se ve una: cuenta igual.
		switch {
		case clave != s.clave && seFue:
			s.total.Relevo++
		case !con2 && len(r.Armadas) < 2 && clave == s.clave:
			// una mano sola que se mueve rápido
		case clave != s.clave:
			s.total.CambioDeMano++
		default:
			s.total.MismaClave++
		}
	}
	if r.TienePos && r.Dentro && p.tiene {
		if s.hayFiltrado && (con2 || r.Relevo) && math.Hypot(p.pos.X-s.filtrado.X, p.pos.Y-s.filtrado.Y) > SaltoPunteroMin {
			s.total.Filtrado++
		}
		s.filtrado, s.hayFiltrado = p.pos, true
	} else {
		s.hayFiltrado = false
	}
	for _, d := range donde {
		s.vista[d.clave] = vistaMano{n: s.n, t: r.TMs}
	}
	if r.TienePos {
		s.clave = clave
	}
	s.crudo, s.hay, s.manos = ahora, r.TienePos, len(c.Manos)
}
