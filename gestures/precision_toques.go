package gestures

import (
	"math"
	"sort"
	"strings"
)

// Precisión de punta a punta con las tomas reales (cotejo-uy84og, criterios 1
// a 3 de dec-uaex). Las tomas de fixtures/pellizco no guardan blancos en la
// pantalla, pero cada una dice qué pidió hacer, así que de ellas salen tres
// cifras repetibles:
//
//   - Aciertos: en una toma de arrastre, el táctil ve exactamente un toque y
//     es un arrastre en la dirección pedida; en una de zoom, exactamente dos
//     dedos que llegan a estar apoyados a la vez. Un toque de más (un corte
//     del pellizco que parpadea, un pellizco involuntario) ya no es acierto.
//   - Falsos: toques en las tomas que no piden pellizcar (entrar y salir del
//     panel sin pellizco, movimiento normal).
//   - Bordes: cada vez que la mano sale del panel por un borde, ¿llegó antes
//     el puntero a ese borde (barra, dock)? Es el alcance del criterio 2.
//
// Y una cuarta que habla de la puntería del criterio 1: el salto al apoyar,
// cuánto se mueve el puntero mientras se cierra el pellizco (del último
// cuadro con la mano abierta al apoyo del dedo), en unidades del panel. El
// dedo cae donde está el puntero al armar, no donde apuntaba la persona; en
// una pantalla de 1920 px, 0,01 son 19 px.
//
// El recorrido es el de la app (desktop.Panel): filtro del panel con banda
// exterior antes del embrague, embrague, ToquesDe y el puntero con su filtro
// One Euro, que solo avanza con la mano dentro del panel.

// bordeAlcanzado es lo cerca del borde (en unidades de la pantalla) que tiene
// que quedar el puntero para contar como que llegó: un píxel en 1920.
const bordeAlcanzado = 0.0006

// maxMsCerrar es lo más que se acepta entre el último cuadro con la mano
// abierta y el apoyo para medir el salto: más tiempo ya es otro movimiento.
const maxMsCerrar = 500

// PrecisionToques resume la precisión de una o varias tomas.
type PrecisionToques struct {
	Intentos       int       // tomas que piden un gesto (arrastres y zoom)
	Aciertos       int       // de esas, las que salen exactas
	SinPellizco    int       // tomas que no piden pellizcar
	Falsos         int       // toques en las tomas sin pellizco
	DeMas          int       // toques de más en las tomas con gesto
	Bordes         int       // salidas por un borde con el puntero en el borde
	BordesPosibles int       // salidas por un borde
	Saltos         []float64 // salto al apoyar de cada toque de una mano
	// Temblor: paso del puntero por cuadro con la mano quieta; Retraso:
	// distancia entre el puntero y la mano cruda al moverla rápido.
	Temblor, Retraso []float64
}

// Percentil da el percentil q (0..1) de v, o 0 si está vacío.
func Percentil(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[int(math.Round(q*float64(len(s)-1)))]
}

// SaltoPercentil da el percentil p (0..1) del salto al apoyar, o 0 sin
// saltos.
func (p PrecisionToques) SaltoPercentil(q float64) float64 {
	if len(p.Saltos) == 0 {
		return 0
	}
	s := append([]float64(nil), p.Saltos...)
	sort.Float64s(s)
	i := int(math.Round(q * float64(len(s)-1)))
	return s[i]
}

// PrecisionDe juzga cada toma según lo que pidió y suma.
func PrecisionDe(tomas []TomaToques) PrecisionToques {
	var p PrecisionToques
	for _, t := range tomas {
		intento, acierto, falsos, deMas := JuzgarToma(t)
		if intento {
			p.Intentos++
			if acierto {
				p.Aciertos++
			}
		} else {
			p.SinPellizco++
		}
		p.Falsos += falsos
		p.DeMas += deMas
		p.Bordes += t.Bordes
		p.BordesPosibles += t.BordesPosibles
		p.Saltos = append(p.Saltos, t.Saltos...)
		p.Temblor = append(p.Temblor, t.Temblor...)
		p.Retraso = append(p.Retraso, t.Retraso...)
	}
	return p
}

// JuzgarToma dice si la toma pide un gesto, si salió exacto y cuántos toques
// sobran: falsos en una toma sin pellizco, de más en una con gesto.
func JuzgarToma(t TomaToques) (intento, acierto bool, falsos, deMas int) {
	switch {
	case strings.HasPrefix(t.Toma, "arrastrar-"):
		quiere := DireccionArrastre(strings.TrimPrefix(t.Toma, "arrastrar-"))
		if len(t.Toques) > 1 {
			deMas = len(t.Toques) - 1
		}
		acierto = len(t.Toques) == 1 && t.Toques[0].Tipo == ToqueArrastre &&
			direccionDe(t.Toques[0].Inicio, t.Toques[0].Fin) == quiere
		return true, acierto, 0, deMas
	case t.Toma == "zoom-dos-manos":
		if len(t.Toques) > 2 {
			deMas = len(t.Toques) - 2
		}
		acierto = len(t.Toques) == 2 && t.Toques[0].Ranura != t.Toques[1].Ranura &&
			t.Toques[1].InicioMs < t.Toques[0].FinMs
		return true, acierto, 0, deMas
	default:
		return false, false, len(t.Toques), 0
	}
}

func direccionDe(a, b PuntoPanel) DireccionArrastre {
	dx, dy := b.X-a.X, b.Y-a.Y
	if math.Abs(dx) >= math.Abs(dy) {
		if dx > 0 {
			return ArrastreDerecha
		}
		return ArrastreIzquierda
	}
	if dy > 0 {
		return ArrastreAbajo
	}
	return ArrastreArriba
}

// filtroPanelApp es el filtro que la app pasa antes del embrague
// (desktop.Panel.Filtrar): una mano cuenta si su mapeador del panel, con la
// banda exterior, la deja dentro. Las claves salen de todas las manos del
// cuadro, como en la app.
type filtroPanelApp struct {
	cal        CalibracionPanel
	mapeadores map[ClaveMano]*MapeadorPanel
}

func (f *filtroPanelApp) cuadro(c Cuadro) Cuadro {
	if f.mapeadores == nil {
		f.mapeadores = map[ClaveMano]*MapeadorPanel{}
	}
	filtrado := c
	filtrado.Manos = make([]Mano, 0, len(c.Manos))
	vistos := map[bool]int{}
	presentes := map[ClaveMano]bool{}
	for _, mano := range c.Manos {
		clave := ClaveMano{Derecha: mano.Derecha, N: vistos[mano.Derecha]}
		vistos[mano.Derecha]++
		presentes[clave] = true
		muestra, ok := MuestraPanelDe(mano, c.Aspecto)
		if !ok {
			continue
		}
		mapa := f.mapeadores[clave]
		if mapa == nil {
			mapa = NuevoMapeadorPanel(f.cal)
			f.mapeadores[clave] = mapa
		}
		if _, dentro := mapa.Mapear(muestra); dentro {
			filtrado.Manos = append(filtrado.Manos, mano)
		}
	}
	for clave := range f.mapeadores {
		if !presentes[clave] {
			delete(f.mapeadores, clave)
		}
	}
	return filtrado
}

// punteroApp repite lo que hace desktop.Panel.aplicar con el puntero: el
// filtro One Euro, que se reinicia al salir sin agarrar, y la posición que
// solo avanza con la mano dentro (es la que recibe el primer dedo).
type punteroApp struct {
	filtro FiltroOneEuroPanel
	pos    PuntoPanel
	tiene  bool
	// Medición de temblor y retraso (no cambia el puntero).
	crudos           []muestraCruda
	salidaPrevia     PuntoPanel
	haySalidaPrevia  bool
	temblor, retraso []float64
}

type muestraCruda struct {
	t int64
	p PuntoPanel
}

// Ventana para decidir si la mano está quieta o se mueve rápido.
const (
	ventanaTemblorMs = 500
	quietaMax        = 0.012 // recorrido neto de la mano cruda en la ventana
	rapidaMin        = 0.6   // velocidad de la mano cruda, panel por segundo
)

func (p *punteroApp) medir(t int64, crudo, salida PuntoPanel) {
	p.crudos = append(p.crudos, muestraCruda{t, crudo})
	for len(p.crudos) > 1 && t-p.crudos[0].t > ventanaTemblorMs {
		p.crudos = p.crudos[1:]
	}
	if p.haySalidaPrevia && len(p.crudos) >= 5 {
		a := p.crudos[0]
		neto := math.Hypot(crudo.X-a.p.X, crudo.Y-a.p.Y)
		dt := float64(t-a.t) / 1000
		if neto < quietaMax {
			p.temblor = append(p.temblor, math.Hypot(salida.X-p.salidaPrevia.X, salida.Y-p.salidaPrevia.Y))
		} else if dt > 0 && neto/dt > rapidaMin {
			p.retraso = append(p.retraso, math.Hypot(salida.X-crudo.X, salida.Y-crudo.Y))
		}
	}
	p.salidaPrevia, p.haySalidaPrevia = salida, true
}

func (p *punteroApp) cortarMedida() { p.crudos, p.haySalidaPrevia = p.crudos[:0], false }

func (p *punteroApp) cuadro(r ResultadoEmbrague) {
	if r.TienePos {
		entrada := PuntoPanel{X: r.X, Y: r.Y}
		if !r.Dentro && r.Fase == EmbragueAgarrando {
			entrada = PuntoPanel{X: acotar01(entrada.X), Y: acotar01(entrada.Y)}
		}
		pos := p.filtro.Filtrar(r.TMs, entrada)
		if r.Dentro {
			p.pos, p.tiene = pos, true
			if entrada.X > 0 && entrada.X < 1 && entrada.Y > 0 && entrada.Y < 1 {
				p.medir(r.TMs, entrada, pos)
			} else {
				p.cortarMedida()
			}
		} else {
			p.cortarMedida()
		}
	} else {
		p.filtro.Reiniciar()
		p.cortarMedida()
	}
	if !r.Dentro && r.Fase != EmbragueAgarrando {
		p.filtro.Reiniciar()
	}
}

// seguimientoPrecision mide, cuadro a cuadro, los bordes y el salto al
// apoyar. Solo mira cuadros con una mano: con dos, el puntero de la app es el
// de una de ellas y la salida por un borde no se puede atribuir.
type seguimientoPrecision struct {
	cal      CalibracionPanel
	mapa     *MapeadorPanel
	dentro   bool
	ultimo   PuntoPanel // puntero en el último cuadro dentro
	abierta  []PuntoPanel
	tAbierta int64
	armadas  int
	bordes   int
	posibles int
	saltos   []float64
}

func (s *seguimientoPrecision) cuadro(c Cuadro, r ResultadoEmbrague, p *punteroApp) {
	if len(c.Manos) != 1 {
		s.mapa, s.dentro, s.abierta = nil, false, nil
		s.armadas = r.ManosArmadas
		return
	}
	mano := c.Manos[0]
	muestra, ok := MuestraPanelDe(mano, c.Aspecto)
	if !ok {
		s.armadas = r.ManosArmadas
		return
	}
	if s.mapa == nil {
		s.mapa = NuevoMapeadorPanel(s.cal)
	}
	_, dentro := s.mapa.Mapear(muestra)
	if s.dentro && !dentro {
		// Salió: por un borde si la palma rebasó la banda exterior; si solo
		// cambió de distancia, no cuenta para el alcance.
		crudo, _ := s.cal.mapearCrudo(muestra)
		if eje, alto, ok := ejeDeSalida(crudo); ok {
			s.posibles++
			v := s.ultimo.X
			if eje == 1 {
				v = s.ultimo.Y
			}
			if (alto && v >= 1-bordeAlcanzado) || (!alto && v <= bordeAlcanzado) {
				s.bordes++
			}
		}
	}
	s.dentro = dentro
	if dentro && p.tiene {
		s.ultimo = p.pos
	}

	// Salto al apoyar: del último cuadro con el pellizco abierto del todo al
	// apoyo de un solo dedo.
	if r.ManosArmadas == 1 && s.armadas == 0 && p.tiene && len(s.abierta) > 0 && c.TMs-s.tAbierta <= maxMsCerrar {
		ref := s.abierta[len(s.abierta)-1]
		s.saltos = append(s.saltos, math.Hypot(p.pos.X-ref.X, p.pos.Y-ref.Y))
	}
	s.armadas = r.ManosArmadas
	if a, ok := AperturaPellizco(mano, c.Aspecto); ok && a >= AjustesPellizcoPorDefecto.UmbralAbrir && dentro && p.tiene {
		s.abierta, s.tAbierta = append(s.abierta[:0], p.pos), c.TMs
	} else if r.ManosArmadas > 0 {
		s.abierta = s.abierta[:0]
	}
}

// ejeDeSalida dice por qué borde quedó la palma fuera de la banda: eje 0 = X,
// 1 = Y; alto = por el lado del 1.
func ejeDeSalida(p PuntoPanel) (eje int, alto bool, ok bool) {
	excesos := [4]float64{-p.X, p.X - 1, -p.Y, p.Y - 1}
	mejor := -1
	for i, e := range excesos {
		if e > BandaExteriorPanel && (mejor < 0 || e > excesos[mejor]) {
			mejor = i
		}
	}
	if mejor < 0 {
		return 0, false, false
	}
	return mejor / 2, mejor%2 == 1, true
}
