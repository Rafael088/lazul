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
)

// EstadoPanel es la parte del panel calibrado que pinta la ventana y lee el
// aura. Fase es fuera, dentro o agarrando; mientras está agarrando, el botón
// virtual del ratón está pulsado y el pellizco actúa de embrague.
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
	X             float64 `json:"x,omitempty"`
	Y             float64 `json:"y,omitempty"`
	Error         string  `json:"error,omitempty"`
}

// Panel toma las muestras de la mano, guía la calibración, mueve el puntero
// absoluto y traduce el pellizco en el botón del ratón (el embrague). El
// archivo solo guarda las dos muestras agregadas, nunca cuadros ni puntos de
// MediaPipe.
type Panel struct {
	mu      sync.Mutex
	Puntero platform.AbsolutePointer
	ruta    string
	// Acciones ejecuta el zoom de dos manos (Ctrl+ más/menos). Es opcional.
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
	x, y        float64
	err         string
}

// NuevoPanel carga la calibración anterior. Si el archivo está dañado,
// devuelve un panel utilizable (sin calibrar) junto con el error para que la
// app lo muestre y permita repetirla.
func NuevoPanel(ruta string, puntero platform.AbsolutePointer) (*Panel, error) {
	p := &Panel{Puntero: puntero, ruta: ruta, filtro: gestures.NuevoFiltroOneEuroPanel()}
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
	qx, qy := media.X/media.Tamano, media.Y/media.Tamano
	for _, m := range ultimas {
		if math.Abs(m.X/m.Tamano-qx) > .07 || math.Abs(m.Y/m.Tamano-qy) > .07 ||
			math.Abs(m.Tamano/media.Tamano-1) > .08 {
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

// Cuadro pasa el cuadro por el embrague: mueve el puntero, pulsa o suelta el
// botón y ejecuta el zoom. mover permite a la prueba de trabajo normal
// observar el panel sin tocar el ratón real.
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

	// Dos manos pellizcando son zoom; una sola pulsa el botón. Nunca las dos
	// cosas a la vez.
	debePresionar := mover && p.Puntero != nil && r.Fase == gestures.EmbragueAgarrando && r.ManosArmadas == 1
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

	if r.Zoom != nil && mover && p.Acciones != nil {
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

func (p *Panel) Preparar() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.calibrado || p.Puntero == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Puntero.Prepare(ctx); err != nil {
		p.err = err.Error()
		return err
	}
	return nil
}

func (p *Panel) Parar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.soltarBoton()
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
		Arrastre: p.arrastre, Zoom: p.zoom, X: p.x, Y: p.y, Error: p.err}
	switch {
	case p.calibrando && p.paso == 1:
		e.Instruccion = "Pon la palma en la esquina superior izquierda del panel imaginario, mantenla quieta y captura la esquina."
	case p.calibrando:
		e.Instruccion = "Ahora pon la palma en la esquina inferior derecha del panel imaginario, mantenla quieta y captura la esquina."
	case !p.calibrado:
		e.Instruccion = "Calibra dos esquinas para crear el panel delante de ti."
	case p.armadas == 2:
		e.Instruccion = "Dos manos pellizcando: sepáralas o júntalas para hacer zoom."
	case p.armadas == 1:
		e.Instruccion = "Pellizco agarrado: el botón del ratón está pulsado. Suelta para soltar."
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
