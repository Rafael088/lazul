package gestures

import "math"

// AccionTactil es una orden discreta del modo táctil. No hay acciones de
// cursor ni clic: el modo solo produce scroll, zoom y barrido.
type AccionTactil int

const (
	TactilNinguno AccionTactil = iota
	TactilModo
	TactilScroll
	TactilZoom
	TactilBarrido
)

var nombresTactiles = [...]string{"ninguno", "modo", "scroll", "zoom", "barrido"}

func (a AccionTactil) String() string {
	if a < 0 || int(a) >= len(nombresTactiles) {
		return "?"
	}
	return nombresTactiles[a]
}

// DireccionTactil completa las cuatro direcciones espaciales con abrir y
// cerrar, que describen la separación del pellizco de zoom.
type DireccionTactil int

const (
	TactilSinDireccion DireccionTactil = iota
	TactilIzquierda
	TactilDerecha
	TactilArriba
	TactilAbajo
	TactilAbrir
	TactilCerrar
)

var nombresDireccionesTactiles = [...]string{"ninguna", "izquierda", "derecha", "arriba", "abajo", "abrir", "cerrar"}

func (d DireccionTactil) String() string {
	if d < 0 || int(d) >= len(nombresDireccionesTactiles) {
		return "?"
	}
	return nombresDireccionesTactiles[d]
}

// EventoTactil es la salida del clasificador táctil. Activo solo cambia con
// TactilModo y dice el estado resultante después de la L.
type EventoTactil struct {
	Accion    AccionTactil
	Direccion DireccionTactil
	Confianza float32
	Activo    bool
	CuadroMs  int64
}

// AjustesTactiles expresa tiempos en ms y distancias en largos de palma. Los
// umbrales separan una intención de los temblores y movimientos de acomodo:
// pose estable, dirección dominante y un rearme neutral entre dos eventos.
type AjustesTactiles struct {
	SostenerL          int64
	GraciaL            int64
	RearmarL           int64
	PrepararMovimiento int64
	VentanaMovimiento  int64
	GraciaMovimiento   int64
	Rearmar            int64
	UmbralScroll       float64
	UmbralBarrido      float64
	DominioDireccion   float64
	UmbralZoom         float64
	ZoomCerradoMax     float64
	ZoomAbiertoMin     float64
	ZoomCentroMax      float64
}

var AjustesTactilesPorDefecto = AjustesTactiles{
	SostenerL:          400,
	GraciaL:            250,
	RearmarL:           800,
	PrepararMovimiento: 120,
	VentanaMovimiento:  1000,
	GraciaMovimiento:   120,
	Rearmar:            250,
	UmbralScroll:       0.70,
	UmbralBarrido:      1.20,
	DominioDireccion:   1.50,
	UmbralZoom:         0.90,
	ZoomCerradoMax:     0.25,
	ZoomAbiertoMin:     0.95,
	ZoomCentroMax:      0.55,
}

type formaTactil int

const (
	formaTactilNinguna formaTactil = iota
	formaTactilDosDedos
	formaTactilZoom
)

type seguimientoTactil struct {
	forma                   formaTactil
	desde, ultimo           int64
	anclaCentro             vec
	anclaApertura, palma    float64
	cuadros, cuadrosDeForma int
}

func (s seguimientoTactil) confianza() float32 {
	if s.cuadros == 0 {
		return 0
	}
	return float32(s.cuadrosDeForma) / float32(s.cuadros)
}

// EstadoTactil es un valor comparable, sin punteros ni memoria compartida.
// Copiarlo y volver a clasificar el mismo cuadro siempre da el mismo resultado.
type EstadoTactil struct {
	aj            AjustesTactiles
	activo        bool
	l             sosten
	lBloqueada    bool
	lNeutralDesde int64
	seguimiento   seguimientoTactil
	esperaNeutral bool
	desdeNeutral  int64
}

// NuevoEstadoTactil crea el clasificador apagado o activo. El arranque activo
// sirve para reproducir cada grabación de un movimiento sin anteponerle la L.
func NuevoEstadoTactil(activo bool, aj AjustesTactiles) EstadoTactil {
	return EstadoTactil{aj: aj, activo: activo}
}

func (e EstadoTactil) Activo() bool { return e.activo }

func (e EstadoTactil) ajustes() AjustesTactiles {
	if e.aj == (AjustesTactiles{}) {
		return AjustesTactilesPorDefecto
	}
	return e.aj
}

func formaTactilDe(r rasgos) formaTactil {
	idx, med, anu, men := r.ext[0], r.ext[1], r.ext[2], r.ext[3]
	switch {
	case idx > dedoEstirado && med > dedoEstirado && anu < dedoRecogido && men < dedoRecogido:
		return formaTactilDosDedos
	case idx > 0.10 && r.recto > 25 && med < dedoRecogido && anu < dedoRecogido && men < dedoRecogido && r.puntas > 0.04 && r.puntas < 1.35:
		return formaTactilZoom
	default:
		return formaTactilNinguna
	}
}

func (e *EstadoTactil) reiniciarMovimiento() {
	e.seguimiento = seguimientoTactil{}
}

func (e *EstadoTactil) bloquearHastaNeutral() {
	e.reiniciarMovimiento()
	e.esperaNeutral = true
	e.desdeNeutral = 0
}

func (e *EstadoTactil) observarNeutral(f formaTactil, t int64, aj AjustesTactiles) bool {
	if !e.esperaNeutral {
		return false
	}
	if f != formaTactilNinguna {
		e.desdeNeutral = 0
		return true
	}
	if e.desdeNeutral == 0 {
		e.desdeNeutral = t
		return true
	}
	if t-e.desdeNeutral < aj.Rearmar {
		return true
	}
	e.esperaNeutral = false
	e.desdeNeutral = 0
	return false
}

func (e *EstadoTactil) seguir(f formaTactil, r rasgos, t int64, aj AjustesTactiles) {
	s := &e.seguimiento
	if f == formaTactilNinguna {
		if s.forma != formaTactilNinguna {
			s.cuadros++
			if t-s.ultimo > aj.GraciaMovimiento {
				e.reiniciarMovimiento()
			}
		}
		return
	}
	if s.forma != f || t-s.desde > aj.VentanaMovimiento {
		*s = seguimientoTactil{
			forma: f, desde: t, ultimo: t, anclaCentro: r.centro,
			anclaApertura: r.puntas, palma: r.palma, cuadros: 1, cuadrosDeForma: 1,
		}
		return
	}
	s.ultimo = t
	s.cuadros++
	s.cuadrosDeForma++
}

func confianzaMovimiento(s seguimientoTactil, distancia, umbral float64) float32 {
	forma := float64(s.confianza())
	margen := math.Min(1, distancia/umbral)
	return float32(math.Min(1, 0.65*forma+0.35*margen))
}

// ClasificarTactil reconoce la L aunque el modo esté apagado. Scroll, zoom y
// barrido solo cuentan con el modo activo y nunca producen cursor ni clic.
func ClasificarTactil(e EstadoTactil, c Cuadro) (EstadoTactil, EventoTactil) {
	aj := e.ajustes()
	t := c.TMs
	var r rasgos
	hayMano := len(c.Manos) > 0
	if hayMano {
		r = medir(c.Manos[0], c.Aspecto)
	}
	esL := hayMano && r.pose == PoseL
	e.l = e.l.avanzar(esL, t, aj.GraciaL)
	if e.lBloqueada {
		if esL {
			e.lNeutralDesde = 0
		} else if e.lNeutralDesde == 0 {
			e.lNeutralDesde = t
		} else if t-e.lNeutralDesde >= aj.RearmarL {
			e.lBloqueada = false
			e.lNeutralDesde = 0
			e.l = sosten{}
		}
	}
	if !e.lBloqueada && e.l.listo(esL, aj.SostenerL) {
		e.activo = !e.activo
		e.lBloqueada = true
		e.bloquearHastaNeutral()
		return e, EventoTactil{Accion: TactilModo, Confianza: e.l.confianza(), Activo: e.activo, CuadroMs: t}
	}
	if !e.activo {
		e.reiniciarMovimiento()
		return e, EventoTactil{}
	}

	// Mientras la L se está formando no puede ser el inicio de un zoom.
	if esL {
		e.reiniciarMovimiento()
		return e, EventoTactil{}
	}
	f := formaTactilNinguna
	if hayMano {
		f = formaTactilDe(r)
	}
	if e.observarNeutral(f, t, aj) {
		return e, EventoTactil{}
	}
	e.seguir(f, r, t, aj)
	s := e.seguimiento
	if s.forma == formaTactilNinguna || t-s.desde < aj.PrepararMovimiento || s.palma <= 0 {
		return e, EventoTactil{}
	}
	d := r.centro.menos(s.anclaCentro)
	dx, dy := d.x/s.palma, d.y/s.palma

	var ev EventoTactil
	switch s.forma {
	case formaTactilDosDedos:
		switch {
		case math.Abs(dy) >= aj.UmbralScroll && math.Abs(dy) >= aj.DominioDireccion*math.Abs(dx):
			dir := TactilAbajo
			if dy < 0 {
				dir = TactilArriba
			}
			ev = EventoTactil{Accion: TactilScroll, Direccion: dir, Confianza: confianzaMovimiento(s, math.Abs(dy), aj.UmbralScroll), Activo: true, CuadroMs: t}
		case math.Abs(dx) >= aj.UmbralBarrido && math.Abs(dx) >= aj.DominioDireccion*math.Abs(dy):
			dir := TactilIzquierda
			if dx < 0 { // imagen sin espejar: es la derecha de la persona
				dir = TactilDerecha
			}
			ev = EventoTactil{Accion: TactilBarrido, Direccion: dir, Confianza: confianzaMovimiento(s, math.Abs(dx), aj.UmbralBarrido), Activo: true, CuadroMs: t}
		}
	case formaTactilZoom:
		delta := r.puntas - s.anclaApertura
		centro := math.Hypot(dx, dy)
		switch {
		case s.anclaApertura <= aj.ZoomCerradoMax && delta >= aj.UmbralZoom && centro <= aj.ZoomCentroMax:
			ev = EventoTactil{Accion: TactilZoom, Direccion: TactilAbrir, Confianza: confianzaMovimiento(s, delta, aj.UmbralZoom), Activo: true, CuadroMs: t}
		case s.anclaApertura >= aj.ZoomAbiertoMin && -delta >= aj.UmbralZoom && centro <= aj.ZoomCentroMax:
			ev = EventoTactil{Accion: TactilZoom, Direccion: TactilCerrar, Confianza: confianzaMovimiento(s, -delta, aj.UmbralZoom), Activo: true, CuadroMs: t}
		}
	}
	if ev.Accion != TactilNinguno {
		e.bloquearHastaNeutral()
	}
	return e, ev
}
