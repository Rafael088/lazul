package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
)

// EstadoPanel es la parte del panel calibrado que pinta la ventana y lee el
// aura. Fase es fuera, dentro o agarrando; mientras está agarrando, el
// pellizco apoya un dedo táctil (o pulsa el botón en modo de respaldo).
type EstadoPanel struct {
	Calibrado     bool    `json:"calibrado"`
	Calibrando    bool    `json:"calibrando"`
	Paso          int     `json:"paso,omitempty"`
	ListoCapturar bool    `json:"listo_capturar"`
	Instruccion   string  `json:"instruccion"`
	Dentro        bool    `json:"dentro"`
	Fase          string  `json:"fase"` // fuera | dentro | agarrando
	ManosArmadas  int     `json:"manos_armadas,omitempty"`
	Arrastre      string  `json:"arrastre,omitempty"`
	Zoom          string  `json:"zoom,omitempty"`
	ModoEntrada   string  `json:"modo_entrada"` // tactil | respaldo
	AvisoEntrada  string  `json:"aviso_entrada,omitempty"`
	X             float64 `json:"x,omitempty"`
	Y             float64 `json:"y,omitempty"`
	Error         string  `json:"error,omitempty"`
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

	calibracion gestures.CalibracionPanel
	calibrado   bool
	calibrando  bool
	paso        int
	primera     gestures.MuestraPanel
	muestras    []gestures.MuestraPanel
	filtro      gestures.FiltroOneEuroPanel
	embrague    *gestures.MedidorEmbrague
	dentro      bool
	fase        gestures.FaseEmbrague
	armadas     int
	arrastre    string
	zoom        string
	presionado  bool
	tactilListo bool
	avisoTactil string
	traductor   *gestures.TraductorToques
	filtrosDedo [2]gestures.FiltroOneEuroPanel
	dedosAbajo  [2]bool
	ultimaDedo  [2]gestures.PuntoPanel
	x, y        float64
	err         string
}

// NuevoPanel carga la calibración anterior. Si el archivo está dañado,
// devuelve un panel utilizable (sin calibrar) junto con el error para que la
// app lo muestre y permita repetirla.
func NuevoPanel(ruta string, puntero platform.AbsolutePointer) (*Panel, error) {
	p := &Panel{Puntero: puntero, ruta: ruta, filtro: gestures.NuevoFiltroOneEuroPanel(), traductor: gestures.NuevoTraductorToques()}
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
	p.muestras = nil
	p.dentro = false
	p.fase = gestures.EmbragueFuera
	p.armadas = 0
	p.err = ""
	p.filtro.Reiniciar()
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

// CapturarEsquina toma la muestra estable del paso actual. En el segundo paso
// valida y persiste la calibración completa.
func (p *Panel) CapturarEsquina() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.calibrando {
		return errors.New("empieza la calibración primero")
	}
	m, err := p.muestraEstable()
	if err != nil {
		p.err = err.Error()
		return err
	}
	if p.paso == 1 {
		p.primera, p.paso, p.muestras, p.err = m, 2, nil, ""
		return nil
	}
	c, err := gestures.NuevaCalibracionPanel(p.primera, m)
	if err != nil {
		p.err = err.Error()
		return err
	}
	if err := GuardarCalibracionPanel(p.ruta, c); err != nil {
		p.err = "no se guardó la calibración: " + err.Error()
		return err
	}
	p.calibracion, p.calibrado = c, true
	p.embrague = gestures.NuevoMedidorEmbrague(c)
	p.calibrando, p.paso, p.muestras, p.err = false, 0, nil, ""
	p.filtro.Reiniciar()
	return nil
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

// Filtrar conserva solo las manos que caen dentro del panel calibrado. Sin
// calibración no deja pasar ninguna; durante el asistente sí devuelve las
// muestras que van a definir el rectángulo, pero la app las entrega solo al
// Panel y no al flujo de gestos. La zona corporal se aplica antes.
func (p *Panel) Filtrar(c gestures.Cuadro) (gestures.Cuadro, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calibrando {
		return c, 0
	}
	if !p.calibrado {
		filtrado := c
		filtrado.Manos = nil
		return filtrado, len(c.Manos)
	}
	filtrado := c
	filtrado.Manos = make([]gestures.Mano, 0, len(c.Manos))
	for _, mano := range c.Manos {
		muestra, ok := gestures.MuestraPanelDe(mano, c.Aspecto)
		if ok {
			_, ok = p.calibracion.Mapear(muestra)
		}
		if ok {
			filtrado.Manos = append(filtrado.Manos, mano)
		}
	}
	return filtrado, len(c.Manos) - len(filtrado.Manos)
}

// Cuadro pasa el cuadro por el embrague: mueve el puntero y manda los dedos
// al táctil (o el botón y zoom de respaldo). mover permite a la prueba de
// trabajo normal observar el panel sin tocar el sistema real.
func (p *Panel) Cuadro(c gestures.Cuadro, mover bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calibrando {
		if len(c.Manos) == 0 {
			p.muestras = nil
			return
		}
		if m, ok := gestures.MuestraPanelDe(c.Manos[0], c.Aspecto); ok {
			p.observarMuestra(m, c.TMs, false)
		}
		return
	}
	if !p.calibrado {
		return
	}
	if p.embrague == nil {
		p.embrague = gestures.NuevoMedidorEmbrague(p.calibracion)
	}
	p.aplicar(p.embrague.Cuadro(c), c.TMs, mover)
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
	}
	if !r.Dentro && r.Fase != gestures.EmbragueAgarrando {
		p.filtro.Reiniciar()
	}

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
				// Con un solo dedo, el primer contacto coincide exactamente con
				// el puntero que la palma venía moviendo.
				if r.ManosArmadas == 1 && r.TienePos {
					pos = gestures.PuntoPanel{X: p.x, Y: p.y}
				}
			}
			pos = p.filtrosDedo[d.Ranura].Filtrar(tMs, pos)
			p.ultimaDedo[d.Ranura] = pos
			nuevoAbajo[d.Ranura] = true
		} else {
			pos = p.ultimaDedo[d.Ranura]
			nuevoAbajo[d.Ranura] = false
			p.filtrosDedo[d.Ranura].Reiniciar()
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
		}
		p.Reg.Anotar(Linea{Tipo: TipoPanel, Resultado: resultado, Dedos: despues})
	}
	return nil
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
		ModoEntrada: "respaldo", AvisoEntrada: p.avisoTactil}
	if p.tactilListo {
		e.ModoEntrada = "tactil"
	}
	switch {
	case p.calibrando && p.paso == 1:
		e.Instruccion = "Pon la palma en la esquina superior izquierda del panel imaginario, mantenla quieta y captura la esquina."
	case p.calibrando:
		e.Instruccion = "Ahora pon la palma en la esquina inferior derecha del panel imaginario, mantenla quieta y captura la esquina."
	case !p.calibrado:
		e.Instruccion = "Calibra dos esquinas para crear el panel delante de ti."
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
		_, err := p.muestraEstable()
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
