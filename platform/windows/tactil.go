//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
)

// Táctil virtual con InjectTouchInput (Windows 8+): funciona sin pantalla
// táctil y las apps reciben toques de verdad (WM_POINTER), igual que el
// uinput de Linux. Hasta dos contactos; las coordenadas normalizadas van
// sobre la pantalla principal, en píxeles físicos.

const (
	touchSlots = 2

	ptTouch            = 2
	punteroEnRango     = 0x00000002
	punteroEnContacto  = 0x00000004
	punteroAbajo       = 0x00010000
	punteroActualizar  = 0x00020000
	punteroArriba      = 0x00040000
	mascaraContacto    = 0x00000001
	mascaraOrientacion = 0x00000002
	mascaraPresion     = 0x00000004
	retroDefecto       = 0x1 // TOUCH_FEEDBACK_DEFAULT: el círculo que dibuja Windows

	dpiBase = 96 // DPI de la escala 100 %

	// sostener: un dedo quieto se vuelve a mandar cada tanto para que
	// Windows no dé el contacto por perdido si la cámara tarda en dar el
	// cuadro siguiente.
	sostener = 100 * time.Millisecond
)

type punto struct{ x, y int32 }

type rect struct{ izq, arriba, der, abajo int32 }

// pointerInfo es POINTER_INFO en x64 (96 bytes).
type pointerInfo struct {
	tipo          uint32
	id            uint32
	cuadro        uint32
	flags         uint32
	dispositivo   uintptr
	ventana       uintptr
	pixel         punto
	himetric      punto
	pixelCrudo    punto
	himetricCrudo punto
	tiempo        uint32
	historia      uint32
	datos         int32
	teclas        uint32
	rendimiento   uint64
	cambioBoton   int32
}

// pointerTouchInfo es POINTER_TOUCH_INFO en x64 (144 bytes).
type pointerTouchInfo struct {
	info        pointerInfo
	touchFlags  uint32
	mascara     uint32
	contacto    rect
	contactoRaw rect
	orientacion uint32
	presion     uint32
}

func validarPuntosTactiles(points []platform.TouchPoint) error {
	if len(points) > touchSlots {
		return platform.ErrAccionInvalida
	}
	var vistas [touchSlots]bool
	for _, p := range points {
		if p.Slot < 0 || p.Slot >= touchSlots || vistas[p.Slot] ||
			math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) ||
			p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			return platform.ErrAccionInvalida
		}
		vistas[p.Slot] = true
	}
	return nil
}

// estadoTactil convierte cuadros completos en contactos de InjectTouchInput.
// Está separado del sistema para probar los flags sin inyectar nada.
type estadoTactil struct {
	abajo [touchSlots]bool
	pos   [touchSlots]punto
	medio [touchSlots]int32 // medio lado del área de cada dedo, fijado al apoyar
	// dpi da el DPI del monitor en un píxel físico; nil es la escala 100 %.
	dpi func(punto) uint32
}

// medioLado pasa platform.DiametroContacto (píxeles lógicos) a medio lado en
// píxeles físicos con el DPI del monitor: 18 a 100 %, 27 a 150 %, 36 a 200 %.
func medioLado(dpi uint32) int32 {
	if dpi == 0 {
		dpi = dpiBase
	}
	return int32(max(math.Round(platform.DiametroContacto*float64(dpi)/dpiBase/2), 1))
}

func contactoTactil(slot int, p punto, flags uint32, medio int32) pointerTouchInfo {
	return pointerTouchInfo{
		info:        pointerInfo{tipo: ptTouch, id: uint32(slot), flags: flags, pixel: p},
		mascara:     mascaraContacto | mascaraOrientacion | mascaraPresion,
		contacto:    rect{p.x - medio, p.y - medio, p.x + medio, p.y + medio},
		orientacion: 90,
		presion:     32000,
	}
}

// cuadro devuelve los contactos que hay que inyectar para pasar del estado
// anterior a points sobre una pantalla de ancho×alto píxeles. Un dedo que
// sigue apoyado va como actualización; uno omitido o con Down=false se
// levanta en su última posición.
func (s *estadoTactil) cuadro(points []platform.TouchPoint, z zonaTactil) ([]pointerTouchInfo, error) {
	if err := validarPuntosTactiles(points); err != nil {
		return nil, err
	}
	var dedos [touchSlots]platform.TouchPoint
	for _, p := range points {
		dedos[p.Slot] = p
	}
	var cs []pointerTouchInfo
	for slot, d := range dedos {
		switch {
		case d.Down:
			p := z.pixel(d.X, d.Y)
			flags := uint32(punteroEnRango | punteroEnContacto | punteroActualizar)
			// El área se fija al apoyar y no cambia hasta levantar, aunque
			// el dedo pase a otro monitor: arrastre, scroll y zoom ven el
			// mismo contacto.
			medio := s.medio[slot]
			if !s.abajo[slot] {
				flags = punteroEnRango | punteroEnContacto | punteroAbajo
				var dpi uint32
				if s.dpi != nil {
					dpi = s.dpi(p)
				}
				medio = medioLado(dpi)
			}
			cs = append(cs, contactoTactil(slot, p, flags, medio))
		case s.abajo[slot]:
			cs = append(cs, contactoTactil(slot, s.pos[slot], punteroArriba, s.medio[slot]))
		}
	}
	return cs, nil
}

// aplicar confirma el cuadro ya inyectado.
func (s *estadoTactil) aplicar(cs []pointerTouchInfo) {
	for _, c := range cs {
		slot := c.info.id
		s.abajo[slot] = c.info.flags&punteroArriba == 0
		s.pos[slot] = c.info.pixel
		s.medio[slot] = (c.contacto.der - c.contacto.izq) / 2
	}
}

// sostenidos repite los dedos apoyados como actualización en el mismo sitio.
func (s *estadoTactil) sostenidos() []pointerTouchInfo {
	var cs []pointerTouchInfo
	for slot, abajo := range s.abajo {
		if abajo {
			cs = append(cs, contactoTactil(slot, s.pos[slot], punteroEnRango|punteroEnContacto|punteroActualizar, s.medio[slot]))
		}
	}
	return cs
}

type tactilVirtual struct {
	mu      sync.Mutex
	abrir   sync.Once
	err     error
	estado  estadoTactil
	ultimo  time.Time
	latido  *time.Ticker
	parar   chan struct{}
	cerrado bool
	destino *destinoPantalla // monitor elegido; nil o vacío = la principal
}

var _ platform.TouchScreen = (*tactilVirtual)(nil)

func nuevoTactilVirtual() *tactilVirtual {
	return &tactilVirtual{estado: estadoTactil{dpi: dpiEnPixel}}
}

func inyectarContactos(cs []pointerTouchInfo) error {
	if len(cs) == 0 {
		return nil
	}
	ok, _, err := injectTouchInput.Call(uintptr(len(cs)), uintptr(unsafe.Pointer(&cs[0])))
	if ok == 0 {
		return fmt.Errorf("InjectTouchInput: %w", err)
	}
	return nil
}

func (t *tactilVirtual) iniciar() {
	if err := injectTouchInput.Find(); err != nil {
		t.err = fmt.Errorf("táctil virtual: %w: %v", platform.ErrNoImplementado, err)
		return
	}
	ok, _, err := initializeTouchInjection.Call(touchSlots, retroDefecto)
	if ok == 0 {
		t.err = fmt.Errorf("táctil virtual: InitializeTouchInjection: %w", err)
		return
	}
	t.parar = make(chan struct{})
	t.latido = time.NewTicker(sostener)
	go t.sostener(t.latido, t.parar)
}

func (t *tactilVirtual) sostener(latido *time.Ticker, parar chan struct{}) {
	for {
		select {
		case <-parar:
			return
		case <-latido.C:
		}
		t.mu.Lock()
		if !t.cerrado && time.Since(t.ultimo) >= sostener {
			if cs := t.estado.sostenidos(); len(cs) > 0 {
				if conDPIFisico(func() error { return inyectarContactos(cs) }) == nil {
					t.ultimo = time.Now()
				}
			}
		}
		t.mu.Unlock()
	}
}

func (t *tactilVirtual) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.abrir.Do(t.iniciar)
	return t.err
}

func (t *tactilVirtual) Frame(ctx context.Context, points []platform.TouchPoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validarPuntosTactiles(points); err != nil {
		return err
	}
	if err := t.Prepare(ctx); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cerrado {
		return errors.New("táctil virtual: cerrado")
	}
	return conDPIFisico(func() error {
		cs, err := t.estado.cuadro(points, zonaDe(t.destino, pantallaPrincipal))
		if err != nil {
			return err
		}
		if err := inyectarContactos(cs); err != nil {
			return fmt.Errorf("táctil virtual: %w", err)
		}
		t.estado.aplicar(cs)
		t.ultimo = time.Now()
		return nil
	})
}

// Close levanta los dedos apoyados y para el latido. Quien posee el ciclo de
// vida lo llama mediante interface{ Close() error }.
func (t *tactilVirtual) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cerrado {
		return nil
	}
	t.cerrado = true
	if t.latido != nil {
		t.latido.Stop()
		close(t.parar)
	}
	cs, _ := t.estado.cuadro(nil, zonaTactil{ancho: 1, alto: 1})
	if len(cs) == 0 {
		return nil
	}
	err := conDPIFisico(func() error { return inyectarContactos(cs) })
	t.estado.aplicar(cs)
	return err
}
