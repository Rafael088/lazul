package gestures

import (
	"math"
	"sort"
)

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
//     arrastre a 15 fps. Para soltar se exigen 3 cuadros abiertos y 250 ms
//     (las dos cosas: a fps bajos manda el tiempo y a fps altos, los cuadros),
//     así un parpadeo no corta el arrastre. Para apoyar, dos niveles
//     (cotejo-lgbapm): un pellizco muy cerrado (por debajo de UmbralFirme)
//     apoya desde el primer cuadro, para que cuente el tap rápido; uno tibio
//     espera 3 cuadros cerrados seguidos, porque un cierre tibio de uno o dos
//     cuadros es el parpadeo y no un toque. Y el dedo cae en el ancla, donde
//     apuntaba la mano abierta (cotejo-uy84og).
//   - Dos manos pellizcando y separándose o juntándose son un zoom; el resto
//     de los casos es un arrastre, con la dirección del eje que más se movió.
//   - El puntero tiene una mano dueña (cotejo-goem2w): no se lo quita otra
//     que llega o pellizca, solo lo cede si se va (elegirDuena). Cada dedo,
//     en cambio, sale siempre de su propia mano (Armadas).
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
	// CuadrosParaApoyar es cuántos cuadros seguidos tiene que estar cerrado
	// un pellizco tibio que empezó dentro para armarse: un cierre de un solo
	// cuadro (el parpadeo) no apoya el dedo. Uno por debajo de
	// Pellizco.UmbralFirme apoya sin esperar.
	CuadrosParaApoyar int
	// MsAncla: el dedo cae donde estaba el puntero en el último cuadro con
	// la mano abierta, si fue hace como mucho este tiempo. Cerrar el
	// pellizco mueve la palma y el puntero; sin ancla, el dedo cae donde
	// quedó la palma al cerrar y no donde se apuntaba. 0 lo apaga.
	MsAncla         int64
	DistanciaMinima float64 // en unidades del panel (0 a 1)
	ZoomMinimo      float64 // cambio de separación entre manos
}

var AjustesEmbraguePorDefecto = AjustesEmbrague{
	Pellizco:          AjustesPellizcoPorDefecto,
	CuadrosParaSoltar: 3,
	MsParaSoltar:      250,
	CuadrosParaApoyar: 3,
	MsAncla:           400,
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

// ManoArmada es una mano con el pellizco agarrado en este cuadro. Clave la
// identifica entre cuadros (el lado y, si hay dos del mismo lado, el orden),
// así que la misma clave en dos resultados es la misma mano; X e Y son su
// posición en el panel (puede salirse del rectángulo mientras el pellizco
// sigue armado).
type ManoArmada struct {
	Clave ClaveMano `json:"clave"`
	X     float64   `json:"x"`
	Y     float64   `json:"y"`
}

// ResultadoEmbrague es el estado de un cuadro y los gestos que terminaron en
// él.
type ResultadoEmbrague struct {
	TMs      int64        `json:"t_ms"`
	Fase     FaseEmbrague `json:"fase"`
	Dentro   bool         `json:"dentro"`
	TienePos bool         `json:"tiene_pos"`
	// ManosArmadas es cuántas manos tienen el pellizco agarrado. Con una, el
	// botón virtual se pulsa; con dos, el gesto es zoom y no se arrastra.
	ManosArmadas int `json:"manos_armadas"`
	// Armadas es cada una de esas manos con su clave y su posición, ordenadas
	// por clave para que el orden no cambie entre cuadros. ToquesDe las usa
	// para mantener un dedo por mano.
	Armadas []ManoArmada `json:"armadas,omitempty"`
	X       float64      `json:"x"`
	Y       float64      `json:"y"`
	// Puntero es la mano dueña de X e Y, que no tiene por qué ser la que
	// pellizca. Relevo dice que en este cuadro pasaron a otra mano: quien las
	// filtre empieza de cero para no arrastrar el salto.
	Puntero  ClaveMano `json:"puntero"`
	Relevo   bool      `json:"relevo,omitempty"`
	Arrastre *Arrastre `json:"arrastre,omitempty"`
	Zoom     *Zoom     `json:"zoom,omitempty"`
}

// estadoManoEmbrague sigue una mano entre cuadros. La clave es la misma que
// usa MedidorPellizco (lado y orden), así la histéresis y el embrague hablan
// de la misma mano.
type estadoManoEmbrague struct {
	clave         ClaveMano
	cerrada       bool
	armada        bool // el pellizco empezó dentro del panel
	pendiente     bool // empezó dentro y espera CuadrosParaApoyar
	cerrados      int  // cuadros seguidos cerrado
	inicio        PuntoPanel
	ultima        PuntoPanel
	inicioGesto   PuntoPanel
	ultimaGesto   PuntoPanel
	tienePos      bool
	abiertos      int
	ultimoCerrado int64
	sinArrastre   bool // participó en un zoom: al soltar no es arrastre
	// ancla: posición con la mano abierta del todo y su tiempo; desfase:
	// lo que se suma a la palma mientras el dedo está apoyado.
	ancla    PuntoPanel
	tAncla   int64
	hayAncla bool
	desfase  PuntoPanel
}

type estadoZoom struct {
	activo     bool
	claves     [2]ClaveMano
	referencia float64
	ultima     float64
}

// MedidorEmbrague convierte los cuadros de la cámara en fase, arrastres y
// zoom. Es puro respecto de la cámara: la misma entrada da la misma salida.
type MedidorEmbrague struct {
	Calibracion CalibracionPanel
	Ajustes     AjustesEmbrague

	pellizco *MedidorPellizco
	manos    map[ClaveMano]*estadoManoEmbrague
	panel    map[ClaveMano]*MapeadorPanel
	zoom     estadoZoom
	duena    duenaPuntero
}

// duenaPuntero es la mano que mueve el puntero. clave sobrevive a la pérdida
// para saber si la siguiente es un relevo.
type duenaPuntero struct {
	clave    ClaveMano
	hay      bool
	tVista   int64
	ausente  int // cuadros seguidos sin verla en el panel
	palma    MuestraPanel
	manos    int        // cuántas había en su último cuadro
	posicion PuntoPanel // último punto que dio
}

// NuevoMedidorEmbrague arranca con la calibración del panel.
// Los umbrales del pellizco salen de la calibración (los del dueño si se
// midieron).
func NuevoMedidorEmbrague(c CalibracionPanel) *MedidorEmbrague {
	aj := AjustesEmbraguePorDefecto
	aj.Pellizco = c.AjustesPellizco()
	return NuevoMedidorEmbragueCon(c, aj)
}

func NuevoMedidorEmbragueCon(c CalibracionPanel, aj AjustesEmbrague) *MedidorEmbrague {
	if aj == (AjustesEmbrague{}) {
		aj = AjustesEmbraguePorDefecto
	}
	return &MedidorEmbrague{
		Calibracion: c,
		Ajustes:     aj,
		pellizco:    NuevoMedidorPellizco(aj.Pellizco),
		manos:       map[ClaveMano]*estadoManoEmbrague{},
		panel:       map[ClaveMano]*MapeadorPanel{},
	}
}

type manoMedida struct {
	clave      ClaveMano
	estado     EstadoPellizco
	pos        PuntoPanel
	gesto      PuntoPanel
	dentro     bool
	dentroReal bool
	ok         bool
	palma      MuestraPanel
	llega      bool // no estaba en el cuadro anterior
}

// Cuadro avanza el embrague con las manos de un cuadro. Devuelve la fase y
// los gestos que terminan en este cuadro.
func (m *MedidorEmbrague) Cuadro(c Cuadro) ResultadoEmbrague {
	estados := m.pellizco.Cuadro(c.Manos, c.Aspecto)

	vistos := map[bool]int{}
	medidas := make([]manoMedida, len(c.Manos))
	presentes := make(map[ClaveMano]bool, len(c.Manos))
	for i, mano := range c.Manos {
		clave := ClaveMano{Derecha: mano.Derecha, N: vistos[mano.Derecha]}
		vistos[mano.Derecha]++
		presentes[clave] = true
		md := manoMedida{clave: clave, estado: estados[i]}
		if muestra, ok := MuestraPanelDe(mano, c.Aspecto); ok {
			mapa := m.panel[clave]
			if mapa == nil {
				mapa = NuevoMapeadorPanel(m.Calibracion)
				m.panel[clave] = mapa
				md.llega = true
			}
			md.palma = muestra
			md.pos, md.gesto, md.dentro = mapa.mapear(muestra)
			_, md.dentroReal = m.Calibracion.Mapear(muestra)
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
			delete(m.panel, clave)
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
	// m.manos es un mapa: sin ordenar, dos manos que empiezan a la vez
	// cambiarían de orden entre cuadros (y de ranura en ToquesDe).
	sort.Slice(armadas, func(i, j int) bool {
		return antesQue(armadas[i].clave, armadas[j].clave)
	})

	r := ResultadoEmbrague{TMs: c.TMs, ManosArmadas: len(armadas)}
	for _, est := range armadas {
		r.Armadas = append(r.Armadas, ManoArmada{
			Clave: est.clave,
			X:     est.ultima.X + est.desfase.X,
			Y:     est.ultima.Y + est.desfase.Y,
		})
	}
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

	r.Relevo = m.elegirDuena(medidas, c.TMs)
	r.Puntero = m.duena.clave
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
			// Empieza un pellizco. Solo se arma si empieza dentro del panel;
			// si empieza fuera, se queda sin agarrar hasta que se suelte.
			est.cerrada = true
			est.abiertos = 0
			est.cerrados = 0
			est.pendiente = !est.armada && md.dentroReal
		}
		est.cerrados++
		if est.pendiente && (est.cerrados >= m.Ajustes.CuadrosParaApoyar || m.firme(md.estado)) {
			est.pendiente = false
			est.armada = true
			est.inicio = md.pos
			est.ultima = md.pos
			est.inicioGesto = md.gesto
			est.ultimaGesto = md.gesto
			est.tienePos = md.ok
			est.sinArrastre = false
			est.desfase = PuntoPanel{}
			if md.ok && m.anclaVigente(est, t) {
				est.desfase = PuntoPanel{X: est.ancla.X - md.pos.X, Y: est.ancla.Y - md.pos.Y}
			}
		}
		if est.armada && md.ok {
			est.ultima = md.pos
			est.ultimaGesto = md.gesto
			est.tienePos = true
		}
		est.ultimoCerrado = t
		return nil
	}

	// La mano está abierta (o trae una apertura suficiente para la
	// histéresis): cuenta para soltar, pero no borra la posición del último
	// cuadro agarrado, que es la que cierra el arrastre.
	if md.ok && md.dentro && float64(md.estado.Apertura) >= m.Ajustes.Pellizco.UmbralAbrir {
		est.ancla, est.tAncla, est.hayAncla = md.pos, t, true
	}
	est.cerrada = false
	est.pendiente = false
	return m.abrir(est, t)
}

// firme dice si el pellizco está tan cerrado que apoya sin esperar a
// CuadrosParaApoyar: el tap rápido dura uno o dos cuadros (~66 ms a 30 fps,
// ~130 ms a 15 fps) y con tres no llegaba a contar.
func (m *MedidorEmbrague) firme(e EstadoPellizco) bool {
	f := m.Ajustes.Pellizco.UmbralFirme
	return f > 0 && float64(e.Apertura) < f
}

// anclaVigente dice si la mano tiene un ancla reciente (MsAncla).
func (m *MedidorEmbrague) anclaVigente(est *estadoManoEmbrague, t int64) bool {
	return m.Ajustes.MsAncla > 0 && est.hayAncla && t-est.tAncla <= m.Ajustes.MsAncla
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
			est.inicioGesto = est.ultimaGesto
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

// antesQue ordena las claves: primero la izquierda de la persona y, dentro
// del mismo lado, por orden de aparición.
func antesQue(a, b ClaveMano) bool {
	if a.Derecha != b.Derecha {
		return !a.Derecha
	}
	return a.N < b.N
}

func clavesDe(ests []*estadoManoEmbrague) [2]ClaveMano {
	var c [2]ClaveMano
	for i := 0; i < 2 && i < len(ests); i++ {
		c[i] = ests[i].clave
	}
	return c
}

func separacion(claves [2]ClaveMano, manos map[ClaveMano]*estadoManoEmbrague) (float64, bool) {
	a, b := manos[claves[0]], manos[claves[1]]
	if a == nil || b == nil || !a.tienePos || !b.tienePos {
		return 0, false
	}
	return math.Hypot(b.ultimaGesto.X-a.ultimaGesto.X, b.ultimaGesto.Y-a.ultimaGesto.Y), true
}

// arrastreDe clasifica el desplazamiento agarrado de una mano soltada.
func (m *MedidorEmbrague) arrastreDe(est *estadoManoEmbrague) *Arrastre {
	if !est.tienePos {
		return nil
	}
	dx := est.ultimaGesto.X - est.inicioGesto.X
	dy := est.ultimaGesto.Y - est.inicioGesto.Y
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

// elegirDuena mantiene la mano del puntero y dice si pasó a otra. Estar
// quieta no cuenta: quieta es como se apunta. La dueña solo lo cede cuando
// lleva fuera de la vista o del panel lo mismo que se exige para soltar un
// pellizco (cuadros y tiempo). Sin dueña lo toma la primera mano que esté en
// el panel o, si alguna ya agarra, la primera de esas.
func (m *MedidorEmbrague) elegirDuena(medidas []manoMedida, t int64) (relevo bool) {
	armada := func(md *manoMedida) bool { return m.manos[md.clave].armada }
	enPanel := func(md *manoMedida) bool { return md.ok && (md.dentro || armada(md)) }
	d := &m.duena
	if d.hay {
		for i := range medidas {
			if medidas[i].clave == d.clave && enPanel(&medidas[i]) {
				d.tVista, d.ausente, d.palma = t, 0, medidas[i].palma
				d.manos = len(medidas)
				return false
			}
		}
		// La misma mano con otro lado: en el cuadro siguiente hay las mismas
		// manos, la clave de la dueña ya no está y otra nace encima, a menos de
		// media palma. No es un relevo y el filtro sigue, salvo que llegue
		// apoyando junto a otra: ese toque es suyo y no hereda el filtro. La
		// app ya fija el lado (seguimiento.mjs); esto es para el orden crudo
		// de la cámara.
		if d.ausente == 0 && d.manos == len(medidas) && !claveEn(medidas, d.clave) {
			for i := range medidas {
				md := &medidas[i]
				if md.llega && enPanel(md) &&
					math.Hypot(md.palma.X-d.palma.X, md.palma.Y-d.palma.Y) < d.palma.Tamano/2 {
					d.clave, d.tVista, d.palma = md.clave, t, md.palma
					return armada(md) && len(medidas) > 1
				}
			}
		}
		d.ausente++
		if d.ausente < m.Ajustes.CuadrosParaSoltar || t-d.tVista < m.Ajustes.MsParaSoltar {
			return false
		}
		d.hay = false
	}
	var nueva *manoMedida
	for i := range medidas {
		md := &medidas[i]
		if enPanel(md) && (nueva == nil || (armada(md) && !armada(nueva))) {
			nueva = md
		}
	}
	if nueva == nil {
		return false
	}
	relevo = nueva.clave != d.clave
	*d = duenaPuntero{clave: nueva.clave, hay: true, tVista: t, palma: nueva.palma, manos: len(medidas)}
	return relevo
}

func claveEn(medidas []manoMedida, clave ClaveMano) bool {
	for _, md := range medidas {
		if md.clave == clave {
			return true
		}
	}
	return false
}

// completarResultado pone el puntero en su mano dueña y deja la fase que lee
// el aura.
func (m *MedidorEmbrague) completarResultado(medidas []manoMedida, armadas []*estadoManoEmbrague, r *ResultadoEmbrague) {
	d := &m.duena
	var elegida *manoMedida
	for i := range medidas {
		if !d.hay || medidas[i].clave == d.clave {
			elegida = &medidas[i]
			break
		}
	}
	switch {
	case elegida != nil:
		r.Dentro = elegida.dentro
		if elegida.ok {
			p, est := elegida.pos, m.manos[elegida.clave]
			if est.armada {
				// Agarrando, el puntero es el dedo: palma más el desfase del ancla.
				p.X, p.Y = p.X+est.desfase.X, p.Y+est.desfase.Y
			} else if m.anclaVigente(est, r.TMs) && float64(elegida.estado.Apertura) < m.Ajustes.Pellizco.UmbralAbrir {
				// Cerrando el pellizco (ya no abierta del todo y sin armar), el
				// puntero se queda en el ancla: lo que se mueve la palma al
				// cerrar no mueve lo que la persona apuntó.
				p = est.ancla
			}
			r.TienePos, r.X, r.Y = true, p.X, p.Y
			d.posicion = p
		}
	case len(medidas) > 0:
		// La dueña falta y hay otra a la vista: el puntero la espera quieto,
		// sin irse a la otra.
		for _, md := range medidas {
			r.Dentro = r.Dentro || md.dentro
		}
		r.TienePos, r.X, r.Y = true, d.posicion.X, d.posicion.Y
	case d.hay && len(armadas) == 1 && armadas[0].clave == d.clave && armadas[0].tienePos:
		// La mano que agarra desapareció: el puntero se queda donde estaba y
		// el botón sigue pulsado hasta que se cumpla el soltar.
		a := armadas[0]
		r.TienePos, r.X, r.Y = true, a.ultima.X+a.desfase.X, a.ultima.Y+a.desfase.Y
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
	m.manos = map[ClaveMano]*estadoManoEmbrague{}
	m.panel = map[ClaveMano]*MapeadorPanel{}
	m.zoom = estadoZoom{}
	m.duena = duenaPuntero{}
}
