package desktop

import (
	"context"
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// ConsultarFocoCada es cada cuánto se mira qué monitor tiene el foco para
// que el panel lo siga (en Hyprland, un hyprctl monitors de ~5 ms).
const ConsultarFocoCada = time.Second

// Pantallas decide qué monitor controla el panel. Arranca en el que tiene el
// foco; un barrido con la mano abierta pasa el control al monitor de ese
// lado, y si el foco cambia de monitor por otra vía (ratón, teclado), el
// control lo sigue. Cada cambio dirige el táctil y el puntero (Target) y se
// avisa a AlCambiar para mover el aura. Con un solo monitor (o sin Displays)
// nunca llama a Target: todo queda como estaba.
type Pantallas struct {
	mu        sync.Mutex
	Displays  platform.Displays
	Reg       *Registro
	AlCambiar func(platform.Display)

	detector  *gestures.DetectorBarridoPantalla
	actual    platform.Display
	hay       bool
	focoVisto string // último monitor con foco que se vio: el seguimiento es por cambios
	consulta  time.Time
	fallando  bool // la última consulta de monitores falló
}

// NuevasPantallas prepara el control sobre d (que puede ser nil).
func NuevasPantallas(d platform.Displays) *Pantallas {
	return &Pantallas{Displays: d, detector: gestures.NuevoDetectorBarridoPantalla()}
}

func (p *Pantallas) listar() ([]platform.Display, bool) {
	if p.Displays == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ds, err := p.Displays.List(ctx)
	if err != nil {
		// Sin hyprctl (otro compositor) fallaría cada segundo: se anota una vez.
		if !p.fallando {
			p.anotar("error_listar", "", err)
		}
		p.fallando = true
		return nil, false
	}
	p.fallando = false
	return ds, len(ds) > 0
}

func (p *Pantallas) anotar(resultado, dir string, err error) {
	if p.Reg == nil {
		return
	}
	l := Linea{Tipo: TipoPantalla, Resultado: resultado, Direccion: dir}
	if err != nil {
		l.Error = err.Error()
	}
	p.Reg.Anotar(l)
}

// Iniciar elige el monitor con el foco. Se puede repetir (al encender la
// cámara): vuelve a mirar el foco.
func (p *Pantallas) Iniciar(ahora time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.consulta = ahora
	ds, ok := p.listar()
	if !ok {
		return
	}
	foco, _ := platform.Enfocado(ds)
	p.focoVisto = foco.Name
	p.cambiarA(ds, foco, "inicio", "")
}

// cambiarA dirige el táctil y el puntero a d. Con un solo monitor solo lo
// recuerda. Se llama con p.mu tomado.
func (p *Pantallas) cambiarA(ds []platform.Display, d platform.Display, resultado, dir string) bool {
	if len(ds) > 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := p.Displays.Target(ctx, d.Name)
		cancel()
		if err != nil {
			p.anotar("error", dir, err)
			return false
		}
	}
	p.actual, p.hay = d, true
	if len(ds) > 1 {
		p.anotar(resultado, dir, nil)
	}
	if p.AlCambiar != nil {
		p.AlCambiar(d)
	}
	return true
}

// Cuadro mira el barrido en el cuadro (después del filtro del torso, antes
// del panel: el barrido sale del panel por un lado). libre es falso mientras
// el pellizco agarra o se calibra: entonces la mano no cambia de pantalla.
// Devuelve true si el control cambió de monitor.
func (p *Pantallas) Cuadro(c gestures.Cuadro, libre bool, ahora time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Displays == nil {
		return false
	}
	if !libre {
		p.detector.Reiniciar()
		return false
	}
	if lado := p.detector.Cuadro(c); lado != gestures.PantallaNinguna {
		return p.barrer(lado)
	}
	if ahora.Sub(p.consulta) < ConsultarFocoCada {
		return false
	}
	p.consulta = ahora
	ds, ok := p.listar()
	if !ok {
		return false
	}
	foco, _ := platform.Enfocado(ds)
	if foco.Name == p.focoVisto {
		return false
	}
	p.focoVisto = foco.Name
	if p.hay && foco.Name == p.actual.Name {
		return false
	}
	return p.cambiarA(ds, foco, "foco", "")
}

func (p *Pantallas) barrer(lado gestures.LadoPantalla) bool {
	ds, ok := p.listar()
	if !ok || len(ds) < 2 {
		return false // un solo monitor: el barrido no hace nada
	}
	actual := p.actual.Name
	if _, sigue := platform.PorNombre(ds, actual); !p.hay || !sigue {
		f, _ := platform.Enfocado(ds)
		actual = f.Name
	}
	vecino, ok := platform.Vecino(ds, actual, lado == gestures.PantallaIzquierda)
	if !ok {
		p.anotar("sin_vecino", lado.String(), nil)
		return false
	}
	// En Linux Target también mueve el foco y el seguimiento lo verá ya en
	// el monitor actual; en Windows el foco no cambia y focoVisto tampoco,
	// así que el seguimiento no deshace el barrido.
	return p.cambiarA(ds, vecino, "barrido", lado.String())
}

// Actual dice qué monitor controla el panel.
func (p *Pantallas) Actual() (platform.Display, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.actual, p.hay
}
