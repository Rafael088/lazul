package desktop

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// ConsultarFocoCada es cada cuánto se mira qué monitor tiene el foco para
// que el panel lo siga (en Hyprland, un hyprctl monitors de ~5 ms).
const ConsultarFocoCada = time.Second

// Pantallas decide qué monitor controla el panel y atiende el gesto de tres
// dedos (índice, medio y anular estirados, deslizando la mano a un lado).
// Arranca en el monitor que tiene el foco; con dos monitores, los tres
// dedos pasan el control al de ese lado, y si el foco cambia de monitor por
// otra vía (ratón, teclado), el control lo sigue. Cada cambio dirige el
// táctil y el puntero (Target) y se avisa a AlCambiar para mover el aura.
// Con un solo monitor (o sin Displays) nunca llama a Target: los tres dedos
// hacen lo del sistema (Spaces: escritorio en Hyprland, app en Windows).
type Pantallas struct {
	mu        sync.Mutex
	Displays  platform.Displays
	Spaces    platform.Spaces
	Reg       *Registro
	AlCambiar func(platform.Display)

	detector   *gestures.DetectorTresDedos
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
	tresDedos
)

type solicitudPantallas struct {
	tipo    tipoSolicitudPantallas
	intento gestures.IntentoTres
}

// Resultados de una línea «pantalla» del registro por los tres dedos: el
// intento disparó (y qué hizo, en Motivo) o no (y por qué, en Motivo).
const (
	PantallaDisparo = "disparo"
	PantallaNo      = "no"
)

// Lo que hizo un disparo (Motivo de la línea con Resultado disparo).
const (
	TresMonitor   = "monitor"    // el control pasó al monitor de ese lado
	TresEspacio   = "espacio"    // un monitor: escritorio (Hyprland) o app (Windows)
	TresSinVecino = "sin_vecino" // dos monitores y ninguno de ese lado
	TresError     = "error"
	TresSinAccion = "sin_accion" // un monitor y el sistema no sabe cambiar de espacio
)

// MotivoApagado es el intento de los tres dedos que no dispara porque el
// interruptor de la L está apagado. Es distinto de ocupado: ocupado es que el
// pellizco agarra o se calibra; apagado, que el panel no mueve nada.
const MotivoApagado = "apagado"

// NuevasPantallas prepara el control sobre d y s (que pueden ser nil).
func NuevasPantallas(d platform.Displays, s platform.Spaces) *Pantallas {
	return &Pantallas{Displays: d, Spaces: s, detector: gestures.NuevoDetectorTresDedos()}
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

// anotarIntento deja una línea por intento de los tres dedos: el recorrido
// relativo en palmas y cuánto duró, nunca posiciones.
func (p *Pantallas) anotarIntento(resultado, motivo string, in gestures.IntentoTres, err error) {
	if p.Reg == nil {
		return
	}
	l := Linea{Tipo: TipoPantalla, Resultado: resultado, Motivo: motivo, Ms: in.Ms, Cuadros: in.Cuadros,
		Palmas: math.Round(in.Palmas*100) / 100}
	if in.Lado != gestures.PantallaNinguna {
		l.Direccion = in.Lado.String()
	}
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
	if p.cambiarA(ds, foco, "inicio") {
		// Iniciar ocurre antes de los cuadros; no es un cambio pendiente que
		// el primer Cuadro tenga que anunciar otra vez.
		p.mu.Lock()
		p.cambio = false
		p.mu.Unlock()
	}
}

// dirigir lleva el táctil y el puntero a d (Target, solo con más de un
// monitor), lo recuerda y avisa a AlCambiar. Puede tardar (en Linux recrea
// el táctil), así que nunca se llama desde Cuadro.
func (p *Pantallas) dirigir(ds []platform.Display, d platform.Display) error {
	if len(ds) > 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := p.Displays.Target(ctx, d.Name)
		cancel()
		if err != nil {
			return err
		}
	}
	p.mu.Lock()
	p.actual, p.hay = d, true
	p.cambio = true
	p.mu.Unlock()
	if p.AlCambiar != nil {
		p.AlCambiar(d)
	}
	return nil
}

// cambiarA dirige a d al iniciar o al seguir al foco y lo anota (solo con
// más de un monitor: con uno no cambia nada).
func (p *Pantallas) cambiarA(ds []platform.Display, d platform.Display, resultado string) bool {
	if err := p.dirigir(ds, d); err != nil {
		p.anotar("error", "", err)
		return false
	}
	if len(ds) > 1 {
		p.anotar(resultado, "", nil)
	}
	return true
}

// solicitar arranca un trabajo si no hay otro. Los tres dedos tienen
// prioridad: si llegan durante una consulta de foco quedan pendientes y se
// atienden al terminar. Se llama con p.mu tomado.
func (p *Pantallas) solicitar(s solicitudPantallas) {
	if p.trabajando {
		if s.tipo == tresDedos {
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
	case tresDedos:
		p.cambiarPorTres(s.intento)
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
		p.cambiarA(ds, foco, "foco")
	}
}

// Cuadro mira los tres dedos en el cuadro (después del filtro del torso,
// antes del panel: la mano puede salir del panel por un lado) y devuelve el
// cuadro sin la mano en esa pose, para que el panel no toque ni arrastre con
// ella ni el clasificador la tome por otro gesto. libre es falso mientras el
// pellizco agarra o se calibra: entonces el gesto no cambia nada y el
// intento se anota como ocupado. apagado es el interruptor de la L: con el
// panel apagado tampoco cambia nada y el intento se anota como apagado. El
// segundo valor es true si el control cambió de monitor.
func (p *Pantallas) Cuadro(c gestures.Cuadro, libre, apagado bool, ahora time.Time) (gestures.Cuadro, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cambio := p.cambio
	p.cambio = false
	r := p.detector.Cuadro(c)
	if r.Mano >= 0 && r.Mano < len(c.Manos) {
		manos := make([]gestures.Mano, 0, len(c.Manos)-1)
		manos = append(manos, c.Manos[:r.Mano]...)
		c.Manos = append(manos, c.Manos[r.Mano+1:]...)
	}
	if in := r.Intento; in != nil {
		switch {
		case in.Motivo != gestures.MotivoDisparo:
			p.anotarIntento(PantallaNo, in.Motivo, *in, nil)
		case apagado:
			p.anotarIntento(PantallaNo, MotivoApagado, *in, nil)
		case !libre:
			p.anotarIntento(PantallaNo, gestures.MotivoOcupado, *in, nil)
		default:
			p.solicitar(solicitudPantallas{tipo: tresDedos, intento: *in})
			return c, cambio
		}
	}
	if p.Displays == nil || ahora.Sub(p.consulta) < ConsultarFocoCada {
		return c, cambio
	}
	p.consulta = ahora
	p.solicitar(solicitudPantallas{tipo: consultarFoco})
	return c, cambio
}

// cambiarPorTres atiende un disparo de los tres dedos: con dos monitores o
// más pasa al vecino de ese lado; con uno, cambia de espacio.
func (p *Pantallas) cambiarPorTres(in gestures.IntentoTres) {
	// Se vuelve a listar fuera de Cuadro: un monitor pudo conectarse desde la
	// última consulta y los adaptadores necesitan la geometría vigente.
	ds, ok := p.listar()
	if !ok || len(ds) < 2 {
		p.cambiarEspacio(in)
		return
	}
	p.mu.Lock()
	actual, hay := p.actual.Name, p.hay
	p.mu.Unlock()
	if _, sigue := platform.PorNombre(ds, actual); !hay || !sigue {
		f, _ := platform.Enfocado(ds)
		actual = f.Name
	}
	vecino, ok := platform.Vecino(ds, actual, in.Lado == gestures.PantallaIzquierda)
	if !ok {
		p.anotarIntento(PantallaDisparo, TresSinVecino, in, nil)
		return
	}
	// En Linux Target también mueve el foco y el seguimiento lo verá ya en
	// el monitor actual; en Windows el foco no cambia y focoVisto tampoco,
	// así que el seguimiento no deshace el cambio.
	if err := p.dirigir(ds, vecino); err != nil {
		p.anotarIntento(PantallaDisparo, TresError, in, err)
		return
	}
	p.anotarIntento(PantallaDisparo, TresMonitor, in, nil)
}

func (p *Pantallas) cambiarEspacio(in gestures.IntentoTres) {
	if p.Spaces == nil {
		p.anotarIntento(PantallaDisparo, TresSinAccion, in, nil)
		return
	}
	dir := platform.NavigateRight
	if in.Lado == gestures.PantallaIzquierda {
		dir = platform.NavigateLeft
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := p.Spaces.Switch(ctx, dir)
	cancel()
	if err != nil {
		p.anotarIntento(PantallaDisparo, TresError, in, err)
		return
	}
	p.anotarIntento(PantallaDisparo, TresEspacio, in, nil)
}

// Actual dice qué monitor controla el panel.
func (p *Pantallas) Actual() (platform.Display, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.actual, p.hay
}
