package gestures

import (
	"math"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Modo del clasificador. Los gestos 2 a 4 solo cuentan después de la L, así
// un puño o una pinza sin querer no hacen nada:
//
//	Reposo   --L sostenida--------------------------> Activo     (ACTIVAR)
//	Activo   --puño---------------------------------> Agarrando  (AGARRAR)
//	Activo   --pinza y arrastrar--------------------> Reposo     (MANDAR_APP)
//	Agarrando --abrir la mano moviéndola a un lado---> Reposo     (SOLTAR)
//
// Activo vuelve solo a Reposo si pasa VentanaActivo sin la L ni un gesto;
// Agarrando, a los TiempoAgarrando, o si la mano se abre sin moverse a un
// lado (eso cancela sin soltar).
type Modo int

const (
	ModoReposo Modo = iota
	ModoActivo
	ModoAgarrando
)

func (m Modo) String() string {
	switch m {
	case ModoActivo:
		return "activo"
	case ModoAgarrando:
		return "agarrando"
	}
	return "reposo"
}

// ModoDeNombre traduce "activo" a ModoActivo ("" o "reposo" a ModoReposo).
func ModoDeNombre(s string) (Modo, bool) {
	switch s {
	case "", "reposo":
		return ModoReposo, true
	case "activo":
		return ModoActivo, true
	case "agarrando":
		return ModoAgarrando, true
	}
	return 0, false
}

// Ajustes son los tiempos (ms) y distancias (en largos de palma) del
// clasificador. Los valores por defecto son provisionales: se fijan con las
// grabaciones reales (cotejo-x2hyu0); ./dev replay -barrer mide la L con
// varios tiempos de sostén.
type Ajustes struct {
	SostenerL       int64   // cuánto se sostiene la L para activar
	SostenerPuno    int64   // cuánto se sostiene el puño para agarrar
	SostenerPinza   int64   // cuánto se sostiene la pinza antes de que el arrastre cuente
	Gracia          int64   // un hueco así (mano perdida o pose dudosa) no rompe un sostén
	VentanaActivo   int64   // sin gesto ni L en este tiempo, Activo vuelve a Reposo
	VentanaSoltar   int64   // el movimiento de soltar se mide en esta ventana alrededor de la apertura
	TiempoAgarrando int64   // tope de Agarrando sin soltar
	UmbralArrastre  float64 // la pinza tiene que moverse esto para mandar la app
	UmbralSoltar    float64 // la mano tiene que moverse esto a un lado para soltar
}

// AjustesPorDefecto son los que usa un Estado sin ajustes.
var AjustesPorDefecto = Ajustes{
	SostenerL:       400,
	SostenerPuno:    150,
	SostenerPinza:   100,
	Gracia:          120,
	VentanaActivo:   5000,
	VentanaSoltar:   500,
	TiempoAgarrando: 15000,
	UmbralArrastre:  0.8,
	UmbralSoltar:    0.8,
}

// sosten sigue cuánto lleva una pose sostenida, tolerando huecos cortos.
type sosten struct {
	activo             bool
	desde, ultimo      int64
	cuadros, coinciden int
}

func (s sosten) avanzar(coincide bool, t, gracia int64) sosten {
	switch {
	case coincide && !s.activo:
		return sosten{activo: true, desde: t, ultimo: t, cuadros: 1, coinciden: 1}
	case coincide:
		s.ultimo = t
		s.cuadros++
		s.coinciden++
	case s.activo && t-s.ultimo > gracia:
		return sosten{}
	case s.activo:
		s.cuadros++
	}
	return s
}

// listo: la pose está en este cuadro y lleva al menos ms sostenida.
func (s sosten) listo(coincide bool, ms int64) bool {
	return coincide && s.activo && s.ultimo-s.desde >= ms
}

func (s sosten) confianza() float32 {
	if s.cuadros == 0 {
		return 0
	}
	return float32(s.coinciden) / float32(s.cuadros)
}

const tamHistoria = 32 // ~1 s a 30 fps

type muestra struct {
	t int64
	c vec
}

// Estado guarda lo que el clasificador recuerda entre cuadros. Es un valor:
// copiarlo no comparte nada, así Clasificar sigue siendo pura. El valor cero
// está en Reposo con AjustesPorDefecto.
type Estado struct {
	aj        Ajustes
	modo      Modo
	iniciado  bool
	vence     int64 // cuándo se acaba Activo o Agarrando
	l         sosten
	lBloqueda bool // tras activar, la L tiene que deshacerse antes de volver a contar
	puno      sosten
	pinza     sosten
	anclada   bool
	ancla     vec // donde quedó la pinza al sostenerse, o la mano antes de abrirse
	abriendo  bool
	abreT     int64
	hist      [tamHistoria]muestra
	nHist     int
}

// NuevoEstado arranca en un modo dado (las grabaciones de AGARRAR empiezan
// en Activo y las de SOLTAR en Agarrando) con unos ajustes.
func NuevoEstado(m Modo, aj Ajustes) Estado { return Estado{aj: aj, modo: m} }

// Modo en el que está el clasificador.
func (e Estado) Modo() Modo { return e.modo }

func (e Estado) ajustes() Ajustes {
	if e.aj == (Ajustes{}) {
		return AjustesPorDefecto
	}
	return e.aj
}

func (e *Estado) pasarA(m Modo, t int64, aj Ajustes) {
	e.modo = m
	e.puno, e.pinza, e.anclada, e.abriendo = sosten{}, sosten{}, false, false
	switch m {
	case ModoActivo:
		e.vence = t + aj.VentanaActivo
	case ModoAgarrando:
		e.vence = t + aj.TiempoAgarrando
	}
}

// centroHace: dónde estaba la mano hace ms (la muestra más vieja dentro de
// esa ventana).
func (e *Estado) centroHace(t, ms int64) (vec, bool) {
	var mejor muestra
	hay := false
	for i := 0; i < e.nHist && i < tamHistoria; i++ {
		s := e.hist[i]
		if t-s.t <= ms && (!hay || s.t < mejor.t) {
			mejor, hay = s, true
		}
	}
	return mejor.c, hay
}

// direccion del movimiento d (imagen sin espejar) visto por la persona: si
// la mano va a su derecha, en la imagen va a la izquierda.
func direccion(d vec) lazulv1.Direccion {
	if math.Abs(d.x) >= math.Abs(d.y) {
		if d.x < 0 {
			return lazulv1.Direccion_DIRECCION_DERECHA
		}
		return lazulv1.Direccion_DIRECCION_IZQUIERDA
	}
	if d.y < 0 {
		return lazulv1.Direccion_DIRECCION_ARRIBA
	}
	return lazulv1.Direccion_DIRECCION_ABAJO
}

// Clasificar avanza un cuadro. Devuelve el estado nuevo y, si en este cuadro
// se completó un gesto, el Evento (si no, Gesto es GESTO_NINGUNO). Si hay
// varias manos usa la primera.
func Clasificar(e Estado, c Cuadro) (Estado, Evento) {
	aj := e.ajustes()
	t := c.TMs
	if !e.iniciado {
		e.iniciado = true
		e.pasarA(e.modo, t, aj)
	}
	if e.modo != ModoReposo && t > e.vence {
		e.pasarA(ModoReposo, t, aj)
	}

	pose := PoseNinguna
	var r rasgos
	if len(c.Manos) > 0 {
		r = medir(c.Manos[0], c.Aspecto)
		pose = r.pose
		e.hist[e.nHist%tamHistoria] = muestra{t, r.centro}
		e.nHist++
		if e.nHist >= 2*tamHistoria {
			e.nHist -= tamHistoria // mantiene el índice acotado sin perder el orden circular
		}
	}

	esL := pose == PoseL
	e.l = e.l.avanzar(esL, t, aj.Gracia)
	if !e.l.activo {
		e.lBloqueda = false
	}

	var ev Evento
	switch e.modo {
	case ModoReposo:
		if !e.lBloqueda && e.l.listo(esL, aj.SostenerL) {
			ev = Evento{Gesto: lazulv1.Gesto_GESTO_ACTIVAR, Confianza: e.l.confianza()}
			e.lBloqueda = true
			e.pasarA(ModoActivo, t, aj)
		}

	case ModoActivo:
		if esL {
			e.vence = t + aj.VentanaActivo
		}
		esPuno := pose == PosePuno
		e.puno = e.puno.avanzar(esPuno, t, aj.Gracia)
		if e.puno.listo(esPuno, aj.SostenerPuno) {
			ev = Evento{Gesto: lazulv1.Gesto_GESTO_AGARRAR, Confianza: e.puno.confianza()}
			e.pasarA(ModoAgarrando, t, aj)
			break
		}
		esPinza := pose == PosePinza
		e.pinza = e.pinza.avanzar(esPinza, t, aj.Gracia)
		if !e.pinza.activo {
			e.anclada = false
			break
		}
		if !e.anclada && e.pinza.listo(esPinza, aj.SostenerPinza) {
			e.anclada, e.ancla = true, r.pinza
		}
		if e.anclada && esPinza {
			if d := r.pinza.menos(e.ancla); d.largo() > aj.UmbralArrastre*r.palma {
				ev = Evento{Gesto: lazulv1.Gesto_GESTO_MANDAR_APP, Confianza: e.pinza.confianza(), Direccion: direccion(d)}
				e.lBloqueda = e.l.activo
				e.pasarA(ModoReposo, t, aj)
			}
		}

	case ModoAgarrando:
		switch {
		case pose == PosePuno:
			e.abriendo = false
		case pose == PoseAbierta:
			if !e.abriendo {
				ancla, ok := e.centroHace(t, aj.VentanaSoltar)
				if !ok {
					ancla = r.centro
				}
				e.abriendo, e.abreT, e.ancla = true, t, ancla
			}
			if d := r.centro.menos(e.ancla); math.Abs(d.x) > aj.UmbralSoltar*r.palma && math.Abs(d.x) >= math.Abs(d.y) {
				ev = Evento{Gesto: lazulv1.Gesto_GESTO_SOLTAR, Confianza: 1, Direccion: direccion(vec{d.x, 0})}
				e.pasarA(ModoReposo, t, aj)
				break
			}
		}
		if e.abriendo && e.modo == ModoAgarrando && t-e.abreT > aj.VentanaSoltar {
			e.pasarA(ModoReposo, t, aj) // se abrió sin ir a ningún lado: cancela
		}
	}
	return e, ev
}
