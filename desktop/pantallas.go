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

	detector   *gestures.DetectorBarridoPantalla
	actual     platform.Display
	hay        bool
	focoVisto  string // último monitor con foco que se vio: el seguimiento es por cambios
	consulta   time.Time
	fallando   bool // la última consulta de monitores falló
	trabajando bool // hay una consulta/cambio fuera del camino del cuadro
	pendiente  *solicitudPantallas
	cambio     bool // un trabajo terminó de cambiar el monitor desde el último Cuadro
}

type tipoSolicitudPantallas uint8

const (
	consultarFoco tipoSolicitudPantallas = iota + 1
	barrerPantalla
)

type solicitudPantallas struct {
	tipo tipoSolicitudPantallas
	lado gestures.LadoPantalla
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
		p.mu.Lock()
		anotar := !p.fallando
		p.fallando = true
		p.mu.Unlock()
		if anotar {
			p.anotar("error_listar", "", err)
		}
		return nil, false
	}
	p.mu.Lock()
	p.fallando = false
	p.mu.Unlock()
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
	p.consulta = ahora
	p.mu.Unlock()
	ds, ok := p.listar()
	if !ok {
		return
	}
	foco, _ := platform.Enfocado(ds)
	p.mu.Lock()
	p.focoVisto = foco.Name
	p.mu.Unlock()
	if p.cambiarA(ds, foco, "inicio", "") {
		// Iniciar ocurre antes de los cuadros; no es un cambio pendiente que
		// el primer Cuadro tenga que anunciar otra vez.
		p.mu.Lock()
		p.cambio = false
		p.mu.Unlock()
	}
}

// cambiarA dirige el táctil y el puntero a d. Con un solo monitor solo lo
// recuerda. Puede tardar (en Linux recrea el táctil), así que nunca se llama
// desde Cuadro.
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
	p.mu.Lock()
	p.actual, p.hay = d, true
	p.cambio = true
	p.mu.Unlock()
	if len(ds) > 1 {
		p.anotar(resultado, dir, nil)
	}
	if p.AlCambiar != nil {
		p.AlCambiar(d)
	}
	return true
}

// solicitar arranca un trabajo si no hay otro. Los barridos tienen prioridad:
// si llegan durante una consulta de foco quedan pendientes y se atienden al
// terminar. Se llama con p.mu tomado.
func (p *Pantallas) solicitar(s solicitudPantallas) {
	if p.trabajando {
		if s.tipo == barrerPantalla {
			copia := s
			p.pendiente = &copia
		}
		return
	}
	p.trabajando = true
	go p.trabajar(s)
}

func (p *Pantallas) trabajar(s solicitudPantallas) {
	switch s.tipo {
	case consultarFoco:
		p.consultarFoco()
	case barrerPantalla:
		p.barrer(s.lado)
	}
	p.mu.Lock()
	if p.pendiente != nil {
		siguiente := *p.pendiente
		p.pendiente = nil
		p.mu.Unlock()
		p.trabajar(siguiente)
		return
	}
	p.trabajando = false
	p.mu.Unlock()
}

func (p *Pantallas) consultarFoco() {
	ds, ok := p.listar()
	if !ok {
		return
	}
	foco, _ := platform.Enfocado(ds)
	p.mu.Lock()
	if foco.Name == p.focoVisto {
		p.mu.Unlock()
		return
	}
	p.focoVisto = foco.Name
	yaEsta := p.hay && foco.Name == p.actual.Name
	p.mu.Unlock()
	if !yaEsta {
		p.cambiarA(ds, foco, "foco", "")
	}
}

// Cuadro mira el barrido en el cuadro (después del filtro del torso, antes
// del panel: el barrido sale del panel por un lado). libre es falso mientras
// el pellizco agarra o se calibra: entonces la mano no cambia de pantalla.
// Devuelve true si el control cambió de monitor.
func (p *Pantallas) Cuadro(c gestures.Cuadro, libre bool, ahora time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	cambio := p.cambio
	p.cambio = false
	if p.Displays == nil {
		return cambio
	}
	if !libre {
		p.detector.Reiniciar()
		return cambio
	}
	if lado := p.detector.Cuadro(c); lado != gestures.PantallaNinguna {
		p.solicitar(solicitudPantallas{tipo: barrerPantalla, lado: lado})
		return cambio
	}
	if ahora.Sub(p.consulta) < ConsultarFocoCada {
		return cambio
	}
	p.consulta = ahora
	p.solicitar(solicitudPantallas{tipo: consultarFoco})
	return cambio
}

func (p *Pantallas) barrer(lado gestures.LadoPantalla) bool {
	// Se vuelve a listar fuera de Cuadro: un monitor pudo conectarse desde la
	// última consulta y los adaptadores necesitan la geometría vigente.
	ds, ok := p.listar()
	if !ok || len(ds) < 2 {
		return false // un solo monitor: el barrido no hace nada
	}
	p.mu.Lock()
	actual, hay := p.actual.Name, p.hay
	p.mu.Unlock()
	if _, sigue := platform.PorNombre(ds, actual); !hay || !sigue {
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
