package gestures

import "math"

// Embrague (modelo de manipulación directa, dec-fa72): el pellizco pulgar-
// índice es el botón del ratón. Mientras está cerrado se agarra; al abrirlo
// se suelta, así el regreso de la mano no cuenta como parte del arrastre.
//
// Tres reglas salen de las tomas reales de fixtures/pellizco:
//
//   - El pellizco solo cuenta si empieza dentro del panel calibrado. En 38 s
//     de movimiento normal hubo 16 pellizcos involuntarios y los 16 empezaron
//     fuera; uno que empieza fuera y entra se ignora hasta soltarlo.
//   - El pellizco parpadea: 29 veces se abrió uno o dos cuadros en medio de un
//     arrastre a 15 fps. Para soltar se exigen 3 cuadros abiertos y 200 ms
//     (las dos cosas: a fps bajos manda el tiempo y a fps altos, los cuadros),
//     así un parpadeo no corta el arrastre.
//   - Dos manos pellizcando y separándose o juntándose son un zoom; el resto
//     de los casos es un arrastre, con la dirección del eje que más se movió.
//
// La apertura y el estado de cada mano salen de MedidorPellizco (pellizco.go):
// aquí no se duplica el umbral ni la histéresis.

// FaseEmbrague es lo que ve el aura: la mano está fuera del panel, dentro sin
// agarrar, o agarrando (el botón virtual está pulsado).
type FaseEmbrague string

const (
	EmbragueFuera     FaseEmbrague = "fuera"
	EmbragueDentro    FaseEmbrague = "dentro"
	EmbragueAgarrando FaseEmbrague = "agarrando"
)

// DireccionArrastre es el eje dominante del desplazamiento agarrado.
type DireccionArrastre string

const (
	ArrastreArriba    DireccionArrastre = "arriba"
	ArrastreAbajo     DireccionArrastre = "abajo"
	ArrastreIzquierda DireccionArrastre = "izquierda"
	ArrastreDerecha   DireccionArrastre = "derecha"
)

// DireccionZoom distingue separar las manos (acercar) de juntarlas (alejar).
type DireccionZoom string

const (
	ZoomAcercar DireccionZoom = "acercar"
	ZoomAlejar  DireccionZoom = "alejar"
)

// AjustesEmbrague reúne los umbrales del embrague. CuadrosParaSoltar y
// MsParaSoltar se exigen juntos: el que tarde más decide, así el parpadeo no
// corta el arrastre ni con fps altos ni con fps bajos.
type AjustesEmbrague struct {
	Pellizco          AjustesPellizco
	CuadrosParaSoltar int
	MsParaSoltar      int64
	DistanciaMinima   float64 // en unidades del panel (0 a 1)
	ZoomMinimo        float64 // cambio de separación entre manos
}

var AjustesEmbraguePorDefecto = AjustesEmbrague{
	Pellizco:          AjustesPellizcoPorDefecto,
	CuadrosParaSoltar: 3,
	MsParaSoltar:      200,
	// Un arrastre tiene que mover al menos un 15 % del panel: así un
	// pellizco involuntario que apenas se desplaza no cuenta.
	DistanciaMinima: 0.15,
	ZoomMinimo:      0.15,
}

// Arrastre es un pellizco que se movió y se soltó.
type Arrastre struct {
	Direccion DireccionArrastre `json:"direccion"`
	Distancia float64           `json:"distancia"`
	DX        float64           `json:"dx"`
	DY        float64           `json:"dy"`
}

// Zoom es un pellizco de dos manos que cambió su separación.
type Zoom struct {
	Direccion  DireccionZoom `json:"direccion"`
	Separacion float64       `json:"separacion"`
}

// ResultadoEmbrague es el estado de un cuadro y los gestos que terminaron en
// él.
type ResultadoEmbrague struct {
	Fase     FaseEmbrague `json:"fase"`
	Dentro   bool         `json:"dentro"`
	TienePos bool         `json:"tiene_pos"`
	// ManosArmadas es cuántas manos tienen el pellizco agarrado. Con una, el
	// botón virtual se pulsa; con dos, el gesto es zoom y no se arrastra.
	ManosArmadas int       `json:"manos_armadas"`
	X            float64   `json:"x"`
	Y            float64   `json:"y"`
	Arrastre     *Arrastre `json:"arrastre,omitempty"`
	Zoom         *Zoom     `json:"zoom,omitempty"`
}

// estadoManoEmbrague sigue una mano entre cuadros. La clave es la misma que
// usa MedidorPellizco (lado y orden), así la histéresis y el embrague hablan
// de la misma mano.
type estadoManoEmbrague struct {
	clave         claveMano
	cerrada       bool
	armada        bool // el pellizco empezó dentro del panel
	inicio        PuntoPanel
	ultima        PuntoPanel
	tienePos      bool
	abiertos      int
	ultimoCerrado int64
	sinArrastre   bool // participó en un zoom: al soltar no es arrastre
}

type estadoZoom struct {
	activo     bool
	claves     [2]claveMano
	referencia float64
	ultima     float64
}

// MedidorEmbrague convierte los cuadros de la cámara en fase, arrastres y
// zoom. Es puro respecto de la cámara: la misma entrada da la misma salida.
type MedidorEmbrague struct {
	Calibracion CalibracionPanel
	Ajustes     AjustesEmbrague

	pellizco *MedidorPellizco
	manos    map[claveMano]*estadoManoEmbrague
	zoom     estadoZoom
}

// NuevoMedidorEmbrague arranca con la calibración del panel.
func NuevoMedidorEmbrague(c CalibracionPanel) *MedidorEmbrague {
	return NuevoMedidorEmbragueCon(c, AjustesEmbraguePorDefecto)
}

func NuevoMedidorEmbragueCon(c CalibracionPanel, aj AjustesEmbrague) *MedidorEmbrague {
	if aj == (AjustesEmbrague{}) {
		aj = AjustesEmbraguePorDefecto
	}
	return &MedidorEmbrague{
		Calibracion: c,
		Ajustes:     aj,
		pellizco:    NuevoMedidorPellizco(aj.Pellizco),
		manos:       map[claveMano]*estadoManoEmbrague{},
	}
}

type manoMedida struct {
	clave  claveMano
	estado EstadoPellizco
	pos    PuntoPanel
	dentro bool
	ok     bool
}

// Cuadro avanza el embrague con las manos de un cuadro. Devuelve la fase y
// los gestos que terminan en este cuadro.
func (m *MedidorEmbrague) Cuadro(c Cuadro) ResultadoEmbrague {
	estados := m.pellizco.Cuadro(c.Manos, c.Aspecto)

	vistos := map[bool]int{}
	medidas := make([]manoMedida, len(c.Manos))
	presentes := make(map[claveMano]bool, len(c.Manos))
	for i, mano := range c.Manos {
		clave := claveMano{derecha: mano.Derecha, n: vistos[mano.Derecha]}
		vistos[mano.Derecha]++
		presentes[clave] = true
		md := manoMedida{clave: clave, estado: estados[i]}
		if muestra, ok := MuestraPanelDe(mano, c.Aspecto); ok {
			md.pos, md.dentro = m.Calibracion.Mapear(muestra)
			md.ok = true
		}
		medidas[i] = md
	}

	var soltadas []*estadoManoEmbrague
	for _, md := range medidas {
		if est := m.actualizar(md, c.TMs); est != nil {
			soltadas = append(soltadas, est)
		}
	}
	// Una mano que desaparece cuenta como cuadro abierto: si se va y vuelve
	// enseguida con el pellizco cerrado, el arrastre sigue; si no vuelve, el
	// botón se suelta para que no quede pulsado.
	for clave, est := range m.manos {
		if !presentes[clave] {
			if soltada := m.abrir(est, c.TMs); soltada != nil {
				soltadas = append(soltadas, soltada)
			}
		}
	}

	armadas := make([]*estadoManoEmbrague, 0, 2)
	for _, est := range m.manos {
		if est.armada {
			armadas = append(armadas, est)
		}
	}

	r := ResultadoEmbrague{ManosArmadas: len(armadas)}
	m.resolverZoom(armadas, soltadas, &r)
	for _, est := range soltadas {
		if est.sinArrastre {
			est.sinArrastre = false
			continue
		}
		if a := m.arrastreDe(est); a != nil && r.Arrastre == nil {
			r.Arrastre = a
		}
	}

	m.completarResultado(medidas, armadas, &r)
	return r
}

// actualizar aplica el cuadro a una mano presente. Devuelve la mano si el
// pellizco acaba de soltarse.
func (m *MedidorEmbrague) actualizar(md manoMedida, t int64) *estadoManoEmbrague {
	est := m.manos[md.clave]
	if est == nil {
		est = &estadoManoEmbrague{clave: md.clave}
		m.manos[md.clave] = est
	}
	if md.estado.Cerrado {
		if !est.cerrada {
			// Empieza un pellizco. Solo se arma dentro del panel; si empieza
			// fuera, se queda sin agarrar hasta que se suelte.
			est.cerrada = true
			est.abiertos = 0
			if !est.armada {
				if md.dentro {
					est.armada = true
					est.inicio = md.pos
					est.ultima = md.pos
					est.tienePos = true
					est.sinArrastre = false
				}
			}
		}
		if est.armada && md.ok {
			est.ultima = md.pos
			est.tienePos = true
		}
		est.ultimoCerrado = t
		return nil
	}

	// La mano está abierta (o trae una apertura suficiente para la
	// histéresis): cuenta para soltar, pero no borra la posición del último
	// cuadro agarrado, que es la que cierra el arrastre.
	est.cerrada = false
	return m.abrir(est, t)
}

// abrir suma un cuadro abierto y suelta cuando se cumplen los dos umbrales.
func (m *MedidorEmbrague) abrir(est *estadoManoEmbrague, t int64) *estadoManoEmbrague {
	if !est.armada {
		return nil
	}
	est.abiertos++
	if est.abiertos < m.Ajustes.CuadrosParaSoltar || t-est.ultimoCerrado < m.Ajustes.MsParaSoltar {
		return nil
	}
	est.armada = false
	est.cerrada = false
	est.abiertos = 0
	return est
}

// resolverZoom actualiza o cierra el zoom de dos manos y arranca uno nuevo
// cuando las dos manos se agarran a la vez. Las tomas reales separan y juntan
// las manos varias veces en la misma toma, así que el zoom se emite por
// tramos: cada cambio de separación mayor que ZoomMinimo es un paso.
func (m *MedidorEmbrague) resolverZoom(armadas []*estadoManoEmbrague, soltadas []*estadoManoEmbrague, r *ResultadoEmbrague) {
	if m.zoom.activo {
		if len(armadas) == 2 {
			d, ok := separacion(clavesDe(armadas), m.manos)
			if !ok {
				return
			}
			m.zoom.ultima = d
			cambio := d - m.zoom.referencia
			if cambio >= m.Ajustes.ZoomMinimo {
				r.Zoom = &Zoom{Direccion: ZoomAcercar, Separacion: cambio}
				m.zoom.referencia = d
			} else if -cambio >= m.Ajustes.ZoomMinimo {
				r.Zoom = &Zoom{Direccion: ZoomAlejar, Separacion: cambio}
				m.zoom.referencia = d
			}
			return
		}
		// Se soltó una mano: el zoom termina. La mano que siga agarrada
		// continúa como un arrastre desde donde terminó el zoom, no desde
		// donde empezó.
		involucradas := m.zoom.claves
		m.zoom = estadoZoom{}
		for _, est := range armadas {
			est.sinArrastre = false
			est.inicio = est.ultima
		}
		for _, est := range soltadas {
			if est.clave == involucradas[0] || est.clave == involucradas[1] {
				est.sinArrastre = true
			}
		}
		return
	}
	if len(armadas) == 2 && len(soltadas) == 0 {
		claves := clavesDe(armadas)
		if d, ok := separacion(claves, m.manos); ok {
			m.zoom = estadoZoom{activo: true, claves: claves, referencia: d, ultima: d}
			for _, est := range armadas {
				est.sinArrastre = true
			}
		}
	}
}

func clavesDe(ests []*estadoManoEmbrague) [2]claveMano {
	var c [2]claveMano
	for i := 0; i < 2 && i < len(ests); i++ {
		c[i] = ests[i].clave
	}
	return c
}

func separacion(claves [2]claveMano, manos map[claveMano]*estadoManoEmbrague) (float64, bool) {
	a, b := manos[claves[0]], manos[claves[1]]
	if a == nil || b == nil || !a.tienePos || !b.tienePos {
		return 0, false
	}
	return math.Hypot(b.ultima.X-a.ultima.X, b.ultima.Y-a.ultima.Y), true
}

// arrastreDe clasifica el desplazamiento agarrado de una mano soltada.
func (m *MedidorEmbrague) arrastreDe(est *estadoManoEmbrague) *Arrastre {
	if !est.tienePos {
		return nil
	}
	dx := est.ultima.X - est.inicio.X
	dy := est.ultima.Y - est.inicio.Y
	d := math.Hypot(dx, dy)
	if d < m.Ajustes.DistanciaMinima {
		return nil
	}
	a := &Arrastre{Distancia: d, DX: dx, DY: dy}
	if math.Abs(dx) >= math.Abs(dy) {
		if dx > 0 {
			a.Direccion = ArrastreDerecha
		} else {
			a.Direccion = ArrastreIzquierda
		}
	} else {
		if dy > 0 {
			a.Direccion = ArrastreAbajo
		} else {
			a.Direccion = ArrastreArriba
		}
	}
	return a
}

// completarResultado elige la mano del puntero (la que agarra, si hay una) y
// deja la fase que lee el aura.
func (m *MedidorEmbrague) completarResultado(medidas []manoMedida, armadas []*estadoManoEmbrague, r *ResultadoEmbrague) {
	var elegida *manoMedida
	for i := range medidas {
		if elegida == nil {
			elegida = &medidas[i]
		}
		for _, est := range armadas {
			if est.clave == medidas[i].clave {
				elegida = &medidas[i]
			}
		}
	}
	if elegida != nil {
		r.Dentro = elegida.dentro
		if elegida.ok {
			r.TienePos, r.X, r.Y = true, elegida.pos.X, elegida.pos.Y
		}
	} else if len(armadas) == 1 && armadas[0].tienePos {
		// La mano que agarra desapareció: el puntero se queda donde estaba y
		// el botón sigue pulsado hasta que se cumpla el soltar.
		r.TienePos, r.X, r.Y = true, armadas[0].ultima.X, armadas[0].ultima.Y
	}
	if len(armadas) > 0 {
		r.Fase = EmbragueAgarrando
	} else if r.Dentro {
		r.Fase = EmbragueDentro
	} else {
		r.Fase = EmbragueFuera
	}
}

// Parar suelta todo y olvida las manos: se usa al apagar la cámara o al
// recalibrar, para no dejar el botón pulsado.
func (m *MedidorEmbrague) Parar() {
	m.pellizco = NuevoMedidorPellizco(m.Ajustes.Pellizco)
	m.manos = map[claveMano]*estadoManoEmbrague{}
	m.zoom = estadoZoom{}
}
