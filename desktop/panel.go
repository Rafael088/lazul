package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

const (
	NombreCalibracionPanel = "panel.json"
	muestrasParaEsquina    = 8
	maxMuestrasEsquina     = 15
	toleranciaQuieta       = .2 // en palmas, respecto a la media de las últimas muestras
	// El tercer paso mide el pellizco del dueño: 8 cuadros con las puntas
	// juntas que no varíen más de esto entre sí (MediaPipe tiembla 0,03–0,05).
	toleranciaPellizco = .08
)

// motivos con que el registro anota una mano que el panel dejó fuera. Los
// tres primeros son los del diagnóstico del panel (gestures); los otros dos
// dicen por qué no hubo diagnóstico.
const (
	MotivoFueraBorde = gestures.PanelFueraBorde
	MotivoCerca      = gestures.PanelCerca
	MotivoLejos      = gestures.PanelLejos
	MotivoSinPalma   = "sin_palma" // la palma no se pudo medir (sale cortada del cuadro)
	MotivoSinPanel   = "sin_panel" // todavía no hay un panel calibrado
)

// RechazoPanel dice qué manos dejó fuera el panel calibrado en un cuadro y
// por qué, para anotarlo en el registro. Solo cuenta y clasifica: nunca
// guarda una posición.
type RechazoPanel struct {
	Total     int
	PorMotivo map[string]int // MotivoFueraBorde, MotivoCerca, MotivoLejos, MotivoSinPalma o MotivoSinPanel
}

// EstadoPanel es la parte del panel calibrado que pinta la ventana y lee el
// aura. Fase es fuera, dentro o agarrando; mientras está agarrando, el
// pellizco apoya un dedo táctil (o pulsa el botón en modo de respaldo).
type EstadoPanel struct {
	Calibrado        bool    `json:"calibrado"`
	Calibrando       bool    `json:"calibrando"`
	Paso             int     `json:"paso,omitempty"`
	ListoCapturar    bool    `json:"listo_capturar"`
	Instruccion      string  `json:"instruccion"`
	Dentro           bool    `json:"dentro"`
	Fase             string  `json:"fase"` // fuera | dentro | agarrando
	ManosArmadas     int     `json:"manos_armadas,omitempty"`
	Arrastre         string  `json:"arrastre,omitempty"`
	Zoom             string  `json:"zoom,omitempty"`
	ModoEntrada      string  `json:"modo_entrada"` // tactil | respaldo
	AvisoEntrada     string  `json:"aviso_entrada,omitempty"`
	AvisoCalibracion string  `json:"aviso_calibracion,omitempty"`
	X                float64 `json:"x,omitempty"`
	Y                float64 `json:"y,omitempty"`
	Error            string  `json:"error,omitempty"`
	// Encendido es el interruptor de la L: apagado, el panel no mueve el
	// puntero ni toca nada. Arranca apagado.
	Encendido bool `json:"encendido"`
}

// Panel toma las muestras de la mano, guía la calibración, mueve el puntero
// absoluto y traduce el pellizco en dedos de la pantalla táctil. Si esta no
// se puede preparar, conserva el botón del ratón y el zoom por teclado. El
// archivo solo guarda las dos muestras agregadas, nunca cuadros ni puntos de
// MediaPipe.
type Panel struct {
	mu      sync.Mutex
	Puntero platform.AbsolutePointer
	Tactil  platform.TouchScreen
	Reg     *Registro
	ruta    string
	// Acciones ejecuta el zoom de respaldo (Ctrl+ más/menos). Es opcional.
	Acciones platform.TactileActions
	// Iman es la asistencia de puntería de nivel 2 (frena el puntero sobre
	// los botones chicos y lleva a ellos el toque que cae casi encima). Es
	// opcional: nil o apagado, el panel se porta como siempre.
	Iman *Iman

	// apagado: el interruptor de la L. Mientras esté apagado Cuadro no toca
	// nada (salvo para calibrar). NuevoPanel lo deja apagado.
	apagado          bool
	calibracion      gestures.CalibracionPanel
	calibrado        bool
	calibrando       bool
	paso             int
	primera          gestures.MuestraPanel
	nueva            gestures.CalibracionPanel // las dos esquinas, esperando el pellizco
	muestras         []gestures.MuestraPanel
	aperturas        []float64
	aspecto          float64
	avisoCalibracion string
	filtro           gestures.FiltroOneEuroPanel
	mapeadores       map[gestures.ClaveMano]*gestures.MapeadorPanel
	embrague         *gestures.MedidorEmbrague
	dentro           bool
	fase             gestures.FaseEmbrague
	armadas          int
	arrastre         string
	zoom             string
	presionado       bool
	tactilListo      bool
	avisoTactil      string
	traductor        *gestures.TraductorToques
	filtrosDedo      [2]gestures.FiltroOneEuroPanel
	dedosAbajo       [2]bool
	ultimaDedo       [2]gestures.PuntoPanel
	freno            FrenoIman
	desfase          [2]gestures.PuntoPanel // del dedo apoyado respecto a la mano, fijado al apoyar
	conDesfase       [2]bool
	ultimoAjuste     ajusteHecho
	ahora            func() time.Time // reloj del imán; nil es time.Now
	x, y             float64
	err              string
}

// NuevoPanel carga la calibración anterior. Si el archivo está dañado,
// devuelve un panel utilizable (sin calibrar) junto con el error para que la
// app lo muestre y permita repetirla.
func NuevoPanel(ruta string, puntero platform.AbsolutePointer) (*Panel, error) {
	p := &Panel{apagado: true, Puntero: puntero, ruta: ruta, filtro: gestures.NuevoFiltroOneEuroPanel(), traductor: gestures.NuevoTraductorToques(), mapeadores: map[gestures.ClaveMano]*gestures.MapeadorPanel{}}
	for i := range p.filtrosDedo {
		p.filtrosDedo[i] = gestures.NuevoFiltroOneEuroPanel()
	}
	c, err := LeerCalibracionPanel(ruta)
	if err != nil {
		p.err = err.Error()
		return p, err
	}
	if c.Version != 0 {
		p.calibracion, p.calibrado = c, true
		p.embrague = gestures.NuevoMedidorEmbrague(c)
	}
	return p, nil
}

func LeerCalibracionPanel(ruta string) (gestures.CalibracionPanel, error) {
	b, err := os.ReadFile(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return gestures.CalibracionPanel{}, nil
	}
	if err != nil {
		return gestures.CalibracionPanel{}, err
	}
	var c gestures.CalibracionPanel
	if err := json.Unmarshal(b, &c); err != nil {
		return gestures.CalibracionPanel{}, fmt.Errorf("%s: %w", NombreCalibracionPanel, err)
	}
	if err := c.Valida(); err != nil {
		return gestures.CalibracionPanel{}, fmt.Errorf("%s: %w", NombreCalibracionPanel, err)
	}
	return c, nil
}

func GuardarCalibracionPanel(ruta string, c gestures.CalibracionPanel) error {
	if err := c.Valida(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := ruta + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, ruta); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// EmpezarCalibracion conserva la calibración anterior hasta que las dos
// esquinas nuevas se validen y se guarden.
func (p *Panel) EmpezarCalibracion() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.soltarBoton()
	p.levantarDedos()
	p.calibrando, p.paso = true, 1
	p.muestras, p.aperturas = nil, nil
	p.dentro = false
	p.fase = gestures.EmbragueFuera
	p.armadas = 0
	p.err = ""
	p.avisoCalibracion = ""
	p.filtro.Reiniciar()
	p.mapeadores = map[gestures.ClaveMano]*gestures.MapeadorPanel{}
	p.embrague = nil
}

func (p *Panel) muestraEstable() (gestures.MuestraPanel, error) {
	if len(p.muestras) < muestrasParaEsquina {
		return gestures.MuestraPanel{}, fmt.Errorf("mantén la mano quieta: faltan %d cuadros", muestrasParaEsquina-len(p.muestras))
	}
	ultimas := p.muestras[len(p.muestras)-muestrasParaEsquina:]
	var media gestures.MuestraPanel
	for _, m := range ultimas {
		media.X += m.X
		media.Y += m.Y
		media.Tamano += m.Tamano
	}
	n := float64(len(ultimas))
	media.X, media.Y, media.Tamano = media.X/n, media.Y/n, media.Tamano/n
	// Tolerancia en palmas: 0,2 de palma son ~10 px a 640 con la mano a un
	// brazo de la cámara. MediaPipe tiembla 3–7 px por cuadro con la mano
	// quieta (tomas reales), más con el brazo estirado hacia una esquina;
	// con 0,07 (~3,5 px) solo se lograba con el codo apoyado.
	qx, qy := media.X/media.Tamano, media.Y/media.Tamano
	for _, m := range ultimas {
		if math.Abs(m.X/m.Tamano-qx) > toleranciaQuieta || math.Abs(m.Y/m.Tamano-qy) > toleranciaQuieta ||
			math.Abs(m.Tamano/media.Tamano-1) > .12 {
			return gestures.MuestraPanel{}, errors.New("mantén la mano quieta un momento antes de capturar")
		}
	}
	return media, nil
}

// pellizcoEstable da la apertura mediana de los últimos cuadros con las
// puntas juntas, o por qué todavía no.
func (p *Panel) pellizcoEstable() (float64, error) {
	if len(p.aperturas) < muestrasParaEsquina {
		return 0, fmt.Errorf("junta las puntas del pulgar y el índice: faltan %d cuadros", muestrasParaEsquina-len(p.aperturas))
	}
	ultimas := append([]float64(nil), p.aperturas[len(p.aperturas)-muestrasParaEsquina:]...)
	sort.Float64s(ultimas)
	if ultimas[len(ultimas)-1] > gestures.MaxPellizcoCerrado {
		return 0, errors.New("junta más las puntas del pulgar y el índice, hasta que se toquen")
	}
	if ultimas[len(ultimas)-1]-ultimas[0] > toleranciaPellizco {
		return 0, errors.New("mantén el pellizco quieto un momento antes de capturar")
	}
	return (ultimas[len(ultimas)/2-1] + ultimas[len(ultimas)/2]) / 2, nil
}

// CapturarEsquina toma la muestra estable del paso actual. El segundo paso
// valida el rectángulo; el tercero mide el pellizco del dueño y persiste la
// calibración completa.
func (p *Panel) CapturarEsquina() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.calibrando {
		return errors.New("empieza la calibración primero")
	}
	if p.paso == 3 {
		return p.capturarPellizco()
	}
	m, err := p.muestraEstable()
	if err != nil {
		p.err = err.Error()
		return err
	}
	if p.paso == 1 {
		p.primera, p.paso, p.muestras, p.err = m, 2, nil, ""
		p.avisoCalibracion = avisoEsquinaCercaDelBorde("La esquina superior izquierda", m, p.aspecto)
		return nil
	}
	c, err := gestures.NuevaCalibracionPanel(p.primera, m)
	if err != nil {
		p.err = err.Error()
		return err
	}
	if aviso := avisoEsquinaCercaDelBorde("La esquina inferior derecha", m, p.aspecto); aviso != "" {
		if p.avisoCalibracion != "" {
			p.avisoCalibracion += " "
		}
		p.avisoCalibracion += aviso
	}
	p.nueva, p.paso, p.muestras, p.aperturas, p.err = c, 3, nil, nil, ""
	return nil
}

// capturarPellizco cierra la calibración con el pellizco del dueño: los
// umbrales del embrague se fijan a su mano (gestures.AjustesPellizcoPara).
// Se llama con p.mu tomado.
func (p *Panel) capturarPellizco() error {
	a, err := p.pellizcoEstable()
	if err != nil {
		p.err = err.Error()
		return err
	}
	c := p.nueva
	c.PellizcoCerrado = a
	if err := GuardarCalibracionPanel(p.ruta, c); err != nil {
		p.err = "no se guardó la calibración: " + err.Error()
		return err
	}
	p.calibracion, p.calibrado = c, true
	p.embrague = gestures.NuevoMedidorEmbrague(c)
	p.calibrando, p.paso, p.muestras, p.aperturas, p.err = false, 0, nil, nil, ""
	p.filtro.Reiniciar()
	p.mapeadores = map[gestures.ClaveMano]*gestures.MapeadorPanel{}
	return nil
}

func avisoEsquinaCercaDelBorde(nombre string, m gestures.MuestraPanel, aspecto float64) string {
	if aspecto <= 0 {
		return ""
	}
	x, y := m.X/aspecto+.5, m.Y+.5
	if x < .1 || x > .9 || y < .1 || y > .9 {
		return nombre + " está a menos del 10 % del borde de la imagen; la mano puede cortarse y perderse. Recalibra un poco más hacia el centro."
	}
	return ""
}

func (p *Panel) observarMuestra(m gestures.MuestraPanel, tMs int64, mover bool) {
	if p.calibrando {
		p.muestras = append(p.muestras, m)
		if len(p.muestras) > maxMuestrasEsquina {
			p.muestras = p.muestras[len(p.muestras)-maxMuestrasEsquina:]
		}
		p.dentro = false
		return
	}
	if !p.calibrado {
		return
	}
	pos, dentro := p.calibracion.Mapear(m)
	if !dentro {
		p.dentro = false
		p.filtro.Reiniciar()
		return
	}
	pos = p.filtro.Filtrar(tMs, pos)
	p.dentro, p.x, p.y = true, pos.X, pos.Y
	if !mover || p.Puntero == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := p.Puntero.MoveAbsolute(ctx, pos.X, pos.Y); err != nil {
		p.err = err.Error()
	} else {
		p.err = ""
	}
}

// Filtrar conserva solo las manos que caen dentro del panel calibrado y dice
// por qué dejó fuera las demás. Sin calibración no deja pasar ninguna;
// durante el asistente sí devuelve las muestras que van a definir el
// rectángulo, pero la app las entrega solo al Panel y no al flujo de gestos.
// La zona corporal se aplica antes.
func (p *Panel) Filtrar(c gestures.Cuadro) (gestures.Cuadro, RechazoPanel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calibrando {
		return c, RechazoPanel{}
	}
	if !p.calibrado {
		filtrado := c
		filtrado.Manos = nil
		rechazo := RechazoPanel{Total: len(c.Manos)}
		if rechazo.Total > 0 {
			rechazo.PorMotivo = map[string]int{MotivoSinPanel: rechazo.Total}
		}
		return filtrado, rechazo
	}
	filtrado := c
	filtrado.Manos = make([]gestures.Mano, 0, len(c.Manos))
	vistos := map[bool]int{}
	presentes := map[gestures.ClaveMano]bool{}
	var rechazo RechazoPanel
	for _, mano := range c.Manos {
		clave := gestures.ClaveMano{Derecha: mano.Derecha, N: vistos[mano.Derecha]}
		vistos[mano.Derecha]++
		presentes[clave] = true
		motivo := MotivoSinPalma
		muestra, ok := gestures.MuestraPanelDe(mano, c.Aspecto)
		if ok {
			mapa := p.mapeadores[clave]
			if mapa == nil {
				mapa = gestures.NuevoMapeadorPanel(p.calibracion)
				p.mapeadores[clave] = mapa
			}
			if _, ok = mapa.Mapear(muestra); !ok {
				motivo = motivoFueraPanel(p.calibracion, muestra, c.Aspecto)
			}
		}
		if ok {
			filtrado.Manos = append(filtrado.Manos, mano)
			continue
		}
		if rechazo.PorMotivo == nil {
			rechazo.PorMotivo = map[string]int{}
		}
		rechazo.Total++
		rechazo.PorMotivo[motivo]++
	}
	for clave := range p.mapeadores {
		if !presentes[clave] {
			delete(p.mapeadores, clave)
		}
	}
	return filtrado, rechazo
}

// motivoFueraPanel pregunta al diagnóstico del panel por qué una mano que el
// mapeador rechazó no cuenta: rebasó el borde o está a otra distancia. Se
// consulta solo al rechazar: el diagnóstico no tiene memoria, y una mano que
// sigue en la banda exterior no debe salir como si hubiera salido.
func motivoFueraPanel(cal gestures.CalibracionPanel, m gestures.MuestraPanel, aspecto float32) string {
	if d, ok := cal.Diagnosticar(m, float64(aspecto)); ok && d.Motivo != gestures.PanelDentro {
		return d.Motivo
	}
	return MotivoSinPalma
}

// Cuadro pasa el cuadro por el embrague: mueve el puntero y manda los dedos
// al táctil (o el botón y zoom de respaldo). mover permite a la prueba de
// trabajo normal observar el panel sin tocar el sistema real.
func (p *Panel) Cuadro(c gestures.Cuadro, mover bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calibrando {
		p.aspecto = float64(c.Aspecto)
		if len(c.Manos) == 0 {
			p.muestras, p.aperturas = nil, nil
			return
		}
		if p.paso == 3 {
			if a, ok := gestures.AperturaPellizco(c.Manos[0], c.Aspecto); ok {
				p.aperturas = append(p.aperturas, a)
				if len(p.aperturas) > maxMuestrasEsquina {
					p.aperturas = p.aperturas[len(p.aperturas)-maxMuestrasEsquina:]
				}
			}
			return
		}
		if m, ok := gestures.MuestraPanelDe(c.Manos[0], c.Aspecto); ok {
			p.observarMuestra(m, c.TMs, false)
		}
		return
	}
	if !p.calibrado || p.apagado {
		return
	}
	c = sinL(c)
	if p.embrague == nil {
		p.embrague = gestures.NuevoMedidorEmbrague(p.calibracion)
	}
	p.aplicar(p.embrague.Cuadro(c), c.TMs, mover)
}

// sinL quita del cuadro las manos que están haciendo la L. La L es el
// interruptor, no un gesto del panel: mientras se forma (o para apagar), el
// panel la ve como una mano que se va, así que suelta lo que tuviera apoyado
// y no puede dejar un toque suelto. No modifica el cuadro de quien llama.
func sinL(c gestures.Cuadro) gestures.Cuadro {
	hayL := false
	for _, m := range c.Manos {
		if gestures.PoseDe(m, c.Aspecto) == gestures.PoseL {
			hayL = true
			break
		}
	}
	if !hayL {
		return c
	}
	manos := make([]gestures.Mano, 0, len(c.Manos))
	for _, m := range c.Manos {
		if gestures.PoseDe(m, c.Aspecto) != gestures.PoseL {
			manos = append(manos, m)
		}
	}
	c.Manos = manos
	return c
}

// Encender es el interruptor del panel (lo mueve la L). Al apagar levanta
// los dedos, suelta el botón y olvida las manos, de modo que no queda ningún
// toque suelto; al encender el embrague empieza de cero. Repetir el mismo
// estado no hace nada.
func (p *Panel) Encender(on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if on == !p.apagado {
		return
	}
	p.apagado = !on
	if on {
		p.embrague = nil
		p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: "enciende"})
		return
	}
	p.soltarBoton()
	p.levantarDedos()
	if p.embrague != nil {
		p.embrague.Parar()
	}
	p.dentro = false
	p.fase, p.armadas = gestures.EmbragueFuera, 0
	p.arrastre, p.zoom = "", ""
	p.filtro.Reiniciar()
	p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: "apaga"})
}

// aplicar traduce el resultado del embrague a los efectos reales del sistema.
// Se llama con p.mu tomado.
func (p *Panel) aplicar(r gestures.ResultadoEmbrague, tMs int64, mover bool) {
	p.fase, p.armadas, p.dentro = r.Fase, r.ManosArmadas, r.Dentro
	if r.Arrastre != nil {
		p.arrastre = string(r.Arrastre.Direccion)
	}
	if r.TienePos {
		entrada := gestures.PuntoPanel{X: r.X, Y: r.Y}
		if !r.Dentro && r.Fase == gestures.EmbragueAgarrando {
			// Fuera del panel pero agarrando, la palma puede proyectarse muy
			// lejos: se limita antes del filtro para no arrastrar ese salto.
			entrada.X, entrada.Y = limitar01(entrada.X), limitar01(entrada.Y)
		}
		pos := p.filtro.Filtrar(tMs, entrada)
		// El freno del imán solo actúa mientras ningún dedo está apoyado:
		// así el contacto nace donde estaba el puntero frenado y el
		// arrastre después va con la mano.
		if r.Dentro && !p.presionado && contarDedos(p.dedosAbajo) == 0 {
			pos = p.freno.Aplicar(p.objetivosIman(), pos)
		} else {
			p.freno.Reiniciar()
		}
		if r.Dentro {
			p.x, p.y = pos.X, pos.Y
		}
		// Con el pellizco agarrado el puntero sigue aunque la palma se salga
		// del rectángulo; se limita al borde de la pantalla.
		if mover && p.Puntero != nil && (r.Dentro || r.Fase == gestures.EmbragueAgarrando) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			err := p.Puntero.MoveAbsolute(ctx, limitar01(pos.X), limitar01(pos.Y))
			cancel()
			if err != nil {
				p.err = err.Error()
			} else {
				p.err = ""
			}
		}
	} else {
		p.filtro.Reiniciar()
		p.freno.Reiniciar()
	}
	if !r.Dentro && r.Fase != gestures.EmbragueAgarrando {
		p.filtro.Reiniciar()
	}
	p.Iman.Pedir(mover && (r.Dentro || r.Fase == gestures.EmbragueAgarrando))

	if mover && p.tactilListo {
		if err := p.enviarToques(r, tMs); err != nil {
			p.desactivarTactil(err)
		}
	}

	// Con la pantalla táctil, el puntero solo anticipa dónde caerá el dedo:
	// nunca pulsa el botón. En respaldo conserva el embrague anterior.
	debePresionar := mover && !p.tactilListo && p.Puntero != nil && r.Fase == gestures.EmbragueAgarrando && r.ManosArmadas == 1
	if debePresionar != p.presionado {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		err := p.Puntero.Button(ctx, debePresionar)
		cancel()
		if err != nil {
			p.err = err.Error()
		} else {
			p.presionado, p.err = debePresionar, ""
		}
	}

	if r.Zoom != nil && mover && !p.tactilListo && p.Acciones != nil {
		p.zoom = string(r.Zoom.Direccion)
		dir := platform.ZoomIn
		if r.Zoom.Direccion == gestures.ZoomAlejar {
			dir = platform.ZoomOut
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := p.Acciones.Zoom(ctx, dir)
		cancel()
		if err != nil {
			p.err = "zoom: " + err.Error()
		}
	}
}

func (p *Panel) enviarToques(r gestures.ResultadoEmbrague, tMs int64) error {
	cuadro := p.traductor.Cuadro(r)
	if len(cuadro.Dedos) == 0 {
		return nil
	}
	puntos := make([]platform.TouchPoint, 0, len(cuadro.Dedos))
	nuevoAbajo := p.dedosAbajo
	for _, d := range cuadro.Dedos {
		pos := gestures.PuntoPanel{X: d.X, Y: d.Y}
		if d.Apoyado {
			if !p.dedosAbajo[d.Ranura] {
				p.filtrosDedo[d.Ranura].Reiniciar()
				p.conDesfase[d.Ranura] = false
				// Con un solo dedo, el primer contacto coincide exactamente con
				// el puntero que la palma venía moviendo.
				if r.ManosArmadas == 1 && r.TienePos {
					contacto := gestures.PuntoPanel{X: p.x, Y: p.y}
					if p.Iman.Encendido() {
						// Con el imán, el dedo entero se corre lo que se
						// corrió el contacto: el tap no se vuelve un
						// arrastre y el arrastre no salta.
						final := p.ajustarIman(contacto)
						p.desfase[d.Ranura] = gestures.PuntoPanel{X: final.X - d.X, Y: final.Y - d.Y}
						p.conDesfase[d.Ranura] = true
					} else {
						pos = contacto
					}
				}
			}
			pos = p.filtrosDedo[d.Ranura].Filtrar(tMs, pos)
			if p.conDesfase[d.Ranura] {
				pos.X += p.desfase[d.Ranura].X
				pos.Y += p.desfase[d.Ranura].Y
			}
			p.ultimaDedo[d.Ranura] = pos
			nuevoAbajo[d.Ranura] = true
		} else {
			pos = p.ultimaDedo[d.Ranura]
			nuevoAbajo[d.Ranura] = false
			p.filtrosDedo[d.Ranura].Reiniciar()
			p.conDesfase[d.Ranura] = false
		}
		puntos = append(puntos, platform.TouchPoint{Slot: d.Ranura, X: limitar01(pos.X), Y: limitar01(pos.Y), Down: d.Apoyado})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err := p.Tactil.Frame(ctx, puntos)
	cancel()
	if err != nil {
		return err
	}
	antes, despues := contarDedos(p.dedosAbajo), contarDedos(nuevoAbajo)
	p.dedosAbajo = nuevoAbajo
	if antes != despues {
		resultado := "levanta"
		if despues > antes {
			resultado = "apoya"
		} else {
			p.Iman.Cambio(p.reloj()) // un tap o un arrastre cambian la pantalla
		}
		p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: resultado, Dedos: despues})
	}
	return nil
}

// ajusteHecho recuerda el último toque que el imán corrió, para repetirlo
// en un doble toque aunque la lista ya no valga (se acaba de levantar el
// dedo).
type ajusteHecho struct {
	t            time.Time
	desde, hasta gestures.PuntoPanel
	ancho, alto  float64
	hay          bool
}

func (p *Panel) reloj() time.Time {
	if p.ahora != nil {
		return p.ahora()
	}
	return time.Now()
}

// objetivosIman es la lista vigente del imán o nil. Se llama con p.mu tomado.
func (p *Panel) objetivosIman() *ObjetivosIman {
	if p.Iman == nil {
		return nil
	}
	return p.Iman.Objetivos(p.reloj())
}

// ajustarIman decide dónde se apoya un dedo que nace en contacto. Se llama
// con p.mu tomado.
func (p *Panel) ajustarIman(contacto gestures.PuntoPanel) gestures.PuntoPanel {
	ahora := p.reloj()
	u := p.ultimoAjuste
	if u.hay && ahora.Sub(u.t) <= RepetirToqueIman &&
		math.Hypot((contacto.X-u.desde.X)*u.ancho, (contacto.Y-u.desde.Y)*u.alto) <= RadioIman {
		p.ultimoAjuste.t = ahora
		p.anotarIman(AjusteRepetir)
		return u.hasta
	}
	o := p.objetivosIman()
	final, res := o.AjustarToque(contacto)
	if res == AjusteMovido {
		p.ultimoAjuste = ajusteHecho{t: ahora, desde: contacto, hasta: final, ancho: o.ancho, alto: o.alto, hay: true}
	} else {
		p.ultimoAjuste.hay = false
	}
	p.anotarIman(res)
	return final
}

func (p *Panel) anotarIman(res ResultadoAjuste) {
	p.Iman.Anotar(res)
	switch res {
	case AjusteMovido, AjusteDuda, AjusteRepetir:
		p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: "iman_" + string(res)})
	}
}

func contarDedos(d [2]bool) int {
	n := 0
	for _, abajo := range d {
		if abajo {
			n++
		}
	}
	return n
}

func (p *Panel) desactivarTactil(err error) {
	p.tactilListo = false
	p.avisoTactil = "No se pudo crear el táctil virtual; uso botón del ratón y Ctrl +/-."
	p.err = err.Error()
	p.traductor = gestures.NuevoTraductorToques()
	p.dedosAbajo = [2]bool{}
	p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: "respaldo", Error: ClaseError(err)})
}

func limitar01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// soltarBoton suelta el botón virtual si estaba pulsado. Se llama con p.mu
// tomado; en Parar y al recalibrar, para no dejarlo pulsado al apagar.
func (p *Panel) soltarBoton() {
	if !p.presionado || p.Puntero == nil {
		p.presionado = false
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err := p.Puntero.Button(ctx, false)
	cancel()
	if err != nil {
		p.err = err.Error()
		return
	}
	p.presionado = false
}

// levantarDedos manda el cuadro final antes de apagar o recalibrar. Aunque
// no haya dedos, reinicia el traductor y los filtros para la sesión siguiente.
func (p *Panel) levantarDedos() {
	cuadro := p.traductor.Parar()
	if p.tactilListo && p.Tactil != nil && len(cuadro.Dedos) > 0 {
		puntos := make([]platform.TouchPoint, 0, len(cuadro.Dedos))
		for _, d := range cuadro.Dedos {
			pos := p.ultimaDedo[d.Ranura]
			puntos = append(puntos, platform.TouchPoint{Slot: d.Ranura, X: limitar01(pos.X), Y: limitar01(pos.Y), Down: false})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		err := p.Tactil.Frame(ctx, puntos)
		cancel()
		if err != nil {
			p.desactivarTactil(err)
		} else {
			p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: "levanta", Dedos: 0})
		}
	}
	p.dedosAbajo = [2]bool{}
	p.conDesfase = [2]bool{}
	p.freno.Reiniciar()
	p.Iman.Pedir(false)
	for i := range p.filtrosDedo {
		p.filtrosDedo[i].Reiniciar()
	}
}

func (p *Panel) Preparar() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var errPuntero error
	if p.calibrado && p.Puntero != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		errPuntero = p.Puntero.Prepare(ctx)
		cancel()
		if errPuntero != nil {
			p.err = errPuntero.Error()
		}
	}
	if p.Tactil == nil {
		p.desactivarTactil(platform.ErrNoImplementado)
		return errPuntero
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	errTactil := p.Tactil.Prepare(ctx)
	cancel()
	if errTactil != nil {
		p.desactivarTactil(errTactil)
		return errors.Join(errPuntero, errTactil)
	}
	p.tactilListo, p.avisoTactil = true, ""
	p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: "tactil"})
	return errPuntero
}

func (p *Panel) Parar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.soltarBoton()
	p.levantarDedos()
	if p.embrague != nil {
		p.embrague.Parar()
	}
	p.muestras = nil
	p.dentro = false
	p.fase, p.armadas = gestures.EmbragueFuera, 0
	p.filtro.Reiniciar()
}

func (p *Panel) Estado() EstadoPanel {
	p.mu.Lock()
	defer p.mu.Unlock()
	fase := p.fase
	if fase == "" {
		fase = gestures.EmbragueFuera
	}
	e := EstadoPanel{Calibrado: p.calibrado, Calibrando: p.calibrando, Paso: p.paso,
		Dentro: p.dentro, Fase: string(fase), ManosArmadas: p.armadas,
		Arrastre: p.arrastre, Zoom: p.zoom, X: p.x, Y: p.y, Error: p.err,
		Encendido: !p.apagado, ModoEntrada: "respaldo", AvisoEntrada: p.avisoTactil, AvisoCalibracion: p.avisoCalibracion}
	if p.tactilListo {
		e.ModoEntrada = "tactil"
	}
	switch {
	case p.calibrando && p.paso == 1:
		e.Instruccion = "Pon la palma en la esquina superior izquierda del panel imaginario, mantenla quieta y captura la esquina."
	case p.calibrando && p.paso == 2:
		e.Instruccion = "Ahora pon la palma en la esquina inferior derecha del panel imaginario, mantenla quieta y captura la esquina."
	case p.calibrando:
		e.Instruccion = "Junta los dedos: toca la punta del índice con la del pulgar, como al pellizcar, mantenlo quieto y captura. Así el pellizco se ajusta a tu mano."
	case !p.calibrado:
		e.Instruccion = "Calibra dos esquinas y tu pellizco para crear el panel delante de ti."
	case p.armadas == 2:
		if p.tactilListo {
			e.Instruccion = "Dos manos pellizcando: dos dedos hacen zoom directamente en la app."
		} else {
			e.Instruccion = "Dos manos pellizcando: sepáralas o júntalas para hacer zoom."
		}
	case p.armadas == 1:
		if p.tactilListo {
			e.Instruccion = "Pellizco apoyado como dedo. Abre para levantarlo."
		} else {
			e.Instruccion = "Pellizco agarrado: el botón del ratón está pulsado. Suelta para soltar."
		}
	case p.dentro:
		e.Instruccion = "Mano dentro del panel: el puntero sigue la palma. Pellizca para agarrar."
	default:
		e.Instruccion = "Panel listo. Mete la palma dentro del rectángulo que calibraste."
	}
	if p.calibrando {
		var err error
		if p.paso == 3 {
			_, err = p.pellizcoEstable()
		} else {
			_, err = p.muestraEstable()
		}
		e.ListoCapturar = err == nil
	}
	return e
}

// VistaPanelCamara es lo que dibuja la vista de cámara para comprobar el
// panel: el rectángulo calibrado y, por cada mano, el panel a su escala y por
// qué cuenta o no. Solo para la pantalla: nunca va al registro.
type VistaPanelCamara struct {
	Calibrado [4]float64                  `json:"calibrado"` // x0, y0, x1, y1 en la imagen
	Manos     []gestures.DiagnosticoPanel `json:"manos"`
}

// Diagnostico usa las manos crudas del cuadro (antes de los filtros de torso
// y panel), para que también se vean las que el panel rechaza.
func (p *Panel) Diagnostico(c gestures.Cuadro) *VistaPanelCamara {
	p.mu.Lock()
	cal, ok := p.calibracion, p.calibrado && !p.calibrando
	p.mu.Unlock()
	if !ok || c.Aspecto <= 0 {
		return nil
	}
	a := float64(c.Aspecto)
	v := &VistaPanelCamara{}
	v.Calibrado[0], v.Calibrado[1], v.Calibrado[2], v.Calibrado[3] = cal.RectanguloCalibrado(a)
	for _, m := range c.Manos {
		if mu, ok := gestures.MuestraPanelDe(m, c.Aspecto); ok {
			if d, ok := cal.Diagnosticar(mu, a); ok {
				v.Manos = append(v.Manos, d)
			}
		}
	}
	return v
}
