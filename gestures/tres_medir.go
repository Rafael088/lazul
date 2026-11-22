package gestures

// MedicionTres es lo que deja pasar una grabación por DetectorTresDedos.
type MedicionTres struct {
	Archivo  string
	Disparos []IntentoTres    // en orden
	Motivos  map[string]int   // intentos por motivo, disparos incluidos
	Cuadros  int              // cuadros con la mano apartada del panel (la pose y su gracia)
	Quiere   LadoPantalla     // el lado de la toma (tres-derecha/izquierda) o PantallaNinguna
	Primero  map[string]int64 // ms del primer intento de cada motivo
}

// MedirTresDedos pasa cada grabación por un detector nuevo, como lo haría la
// app con la mano libre (sin pellizco agarrando).
func MedirTresDedos(gs []Grabacion) []MedicionTres {
	out := make([]MedicionTres, 0, len(gs))
	for _, g := range gs {
		m := MedicionTres{Archivo: g.Archivo, Motivos: map[string]int{}, Primero: map[string]int64{}, Quiere: LadoEsperadoTres(g.Cabecera.Toma)}
		d := NuevoDetectorTresDedos()
		anotar := func(in *IntentoTres, t int64) {
			if in == nil {
				return
			}
			if _, ok := m.Primero[in.Motivo]; !ok {
				m.Primero[in.Motivo] = t
			}
			m.Motivos[in.Motivo]++
			if in.Motivo == MotivoDisparo {
				m.Disparos = append(m.Disparos, *in)
			}
		}
		var fin int64
		for _, c := range g.Cuadros {
			r := d.Cuadro(c)
			if r.Mano >= 0 {
				m.Cuadros++
			}
			anotar(r.Intento, c.TMs)
			fin = c.TMs
		}
		anotar(d.Reiniciar(MotivoCorto), fin)
		out = append(out, m)
	}
	return out
}

// LadoEsperadoTres dice qué lado pide una toma de los tres dedos
// (tres-derecha, tres-izquierda); en cualquier otra, ninguno.
func LadoEsperadoTres(toma string) LadoPantalla {
	switch toma {
	case "tres-derecha":
		return PantallaDerecha
	case "tres-izquierda":
		return PantallaIzquierda
	}
	return PantallaNinguna
}

// Acierto: la toma pedía un lado y hubo exactamente un disparo hacia él, o
// no pedía ninguno y no hubo disparos.
func (m MedicionTres) Acierto() bool {
	if m.Quiere == PantallaNinguna {
		return len(m.Disparos) == 0
	}
	return len(m.Disparos) == 1 && m.Disparos[0].Lado == m.Quiere
}
