//go:build linux

package linux

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/unix"
)

// Constantes de linux/input-event-codes.h y linux/uinput.h para un táctil
// multitoque de protocolo B.
const (
	absMtSlot     = 0x2f
	absMtMayor    = 0x30 // ABS_MT_TOUCH_MAJOR
	absMtMenor    = 0x31 // ABS_MT_TOUCH_MINOR
	absMtX        = 0x35
	absMtY        = 0x36
	absMtTracking = 0x39

	btnTouch = 0x14a
	btnTool1 = 0x145 // BTN_TOOL_FINGER
	btnTool2 = 0x14d // BTN_TOOL_DOUBLETAP

	inputPropDirect = 0x01
	uiDevDestroy    = 0x5502

	maxTouchX  = 2880
	maxTouchY  = 1800
	touchSlots = 2

	// anchoLogicoSupuesto es el ancho lógico de la salida cuando aún no se
	// ha elegido monitor: la principal de 2880 px a escala 2, la misma que
	// da el rango de maxTouchX.
	anchoLogicoSupuesto = 1440
)

// unidadesContacto pasa platform.DiametroContacto (píxeles lógicos) a
// unidades del táctil. Hyprland estira el rango 0..maxTouchX sobre el ancho
// lógico de la salida asignada, así que un píxel lógico vale
// maxTouchX/anchoLogico unidades.
func unidadesContacto(anchoLogico int) int32 {
	if anchoLogico <= 0 {
		anchoLogico = anchoLogicoSupuesto
	}
	u := math.Round(float64(platform.DiametroContacto) * maxTouchX / float64(anchoLogico))
	return int32(min(max(u, 1), maxTouchX))
}

var _ platform.TouchScreen = (*tactilVirtual)(nil)

// estadoTactil contiene solo el estado necesario para convertir cuadros en
// input_event. Está separado del dispositivo para probar los bytes sin abrir
// /dev/uinput.
type estadoTactil struct {
	abajo    [touchSlots]bool
	sig      int32
	contacto int32 // ABS_MT_TOUCH_MAJOR/MINOR de cada dedo, en unidades del táctil
}

func nuevoEstadoTactil() estadoTactil {
	return estadoTactil{sig: 1, contacto: unidadesContacto(0)}
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

func (s *estadoTactil) eventos(points []platform.TouchPoint) ([]byte, error) {
	if err := validarPuntosTactiles(points); err != nil {
		return nil, err
	}
	var dedos [touchSlots]platform.TouchPoint
	for _, p := range points {
		dedos[p.Slot] = p
	}

	var b []byte
	evento := func(tipo, codigo uint16, valor int32) {
		e := make([]byte, 24)
		binary.LittleEndian.PutUint16(e[16:], tipo)
		binary.LittleEndian.PutUint16(e[18:], codigo)
		binary.LittleEndian.PutUint32(e[20:], uint32(valor))
		b = append(b, e...)
	}

	n := 0
	for slot, dedo := range dedos {
		evento(evAbs, absMtSlot, int32(slot))
		switch {
		case dedo.Down && !s.abajo[slot]:
			evento(evAbs, absMtTracking, s.sig)
			s.sig++
			// El área va al apoyar: no cambia mientras el dedo sigue abajo,
			// así el arrastre, el scroll y el zoom ven el mismo contacto.
			evento(evAbs, absMtMayor, s.contacto)
			evento(evAbs, absMtMenor, s.contacto)
			fallthrough
		case dedo.Down:
			evento(evAbs, absMtX, int32(dedo.X*maxTouchX))
			evento(evAbs, absMtY, int32(dedo.Y*maxTouchY))
		case s.abajo[slot]:
			evento(evAbs, absMtTracking, -1)
		}
		if dedo.Down {
			if n == 0 {
				evento(evAbs, absX, int32(dedo.X*maxTouchX))
				evento(evAbs, absY, int32(dedo.Y*maxTouchY))
			}
			n++
		}
	}

	antes := 0
	for _, down := range s.abajo {
		if down {
			antes++
		}
	}
	if (n > 0) != (antes > 0) {
		evento(evKey, btnTouch, boolAInt32(n > 0))
	}
	evento(evKey, btnTool1, boolAInt32(n == 1))
	evento(evKey, btnTool2, boolAInt32(n == 2))
	evento(evSyn, synReport, 0)
	for slot := range dedos {
		s.abajo[slot] = dedos[slot].Down
	}
	return b, nil
}

func boolAInt32(v bool) int32 {
	if v {
		return 1
	}
	return 0
}

type tactilVirtual struct {
	mu     sync.Mutex
	f      *os.File
	err    error
	estado estadoTactil
}

// salida guarda el ancho lógico del monitor que recibe el táctil, para que
// el área de contacto mida lo mismo en píxeles lógicos en cualquier monitor.
// Se llama con t.mu tomado y antes de recrear el dispositivo.
func (t *tactilVirtual) salida(anchoLogico int) {
	t.estado.contacto = unidadesContacto(anchoLogico)
}

func nuevoTactilVirtual() *tactilVirtual {
	t := &tactilVirtual{estado: nuevoEstadoTactil()}
	// Red de seguridad para quien no cierre explícitamente. Close sigue
	// siendo el camino determinista y levanta primero todas las ranuras.
	runtime.SetFinalizer(t, func(t *tactilVirtual) { _ = t.Close() })
	return t
}

func configurarEjeTactil(fd int, codigo uint16, maximo, resolucion int32) error {
	a := make([]byte, 28)
	binary.LittleEndian.PutUint16(a[0:], codigo)
	binary.LittleEndian.PutUint32(a[8:], 0)
	binary.LittleEndian.PutUint32(a[12:], uint32(maximo))
	binary.LittleEndian.PutUint32(a[24:], uint32(resolucion))
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiAbsSetup, uintptr(unsafe.Pointer(&a[0]))); e != 0 {
		return e
	}
	return nil
}

func (t *tactilVirtual) abrirDispositivo() {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.err = fmt.Errorf("táctil virtual: %w", err)
		return
	}
	fallar := func(paso string, err error) {
		_ = f.Close()
		t.err = fmt.Errorf("táctil virtual: %s: %w", paso, err)
	}
	fd := int(f.Fd())
	pasos := []struct{ req, val uintptr }{
		{uiSetEvbit, evKey}, {uiSetKeybit, btnTouch}, {uiSetKeybit, btnTool1}, {uiSetKeybit, btnTool2},
		{uiSetEvbit, evAbs},
		{uiSetAbsbit, absX}, {uiSetAbsbit, absY},
		{uiSetAbsbit, absMtSlot}, {uiSetAbsbit, absMtX}, {uiSetAbsbit, absMtY}, {uiSetAbsbit, absMtTracking},
		{uiSetAbsbit, absMtMayor}, {uiSetAbsbit, absMtMenor},
		{uiSetPropbit, inputPropDirect},
	}
	for _, paso := range pasos {
		if err := ioctlUinput(fd, paso.req, paso.val); err != nil {
			fallar("configurar", err)
			return
		}
	}
	ejes := []struct {
		codigo uint16
		maximo int32
		res    int32
	}{
		{absX, maxTouchX, 10}, {absY, maxTouchY, 10},
		{absMtX, maxTouchX, 10}, {absMtY, maxTouchY, 10},
		{absMtMayor, maxTouchX, 10}, {absMtMenor, maxTouchX, 10},
		{absMtSlot, touchSlots - 1, 0}, {absMtTracking, 65535, 0},
	}
	for _, eje := range ejes {
		if err := configurarEjeTactil(fd, eje.codigo, eje.maximo, eje.res); err != nil {
			fallar("configurar eje", err)
			return
		}
	}

	setup := make([]byte, 92)
	binary.LittleEndian.PutUint16(setup[0:], busVirtual)
	binary.LittleEndian.PutUint16(setup[2:], 0x1209)
	binary.LittleEndian.PutUint16(setup[4:], 0x4c5b)
	binary.LittleEndian.PutUint16(setup[6:], 1)
	copy(setup[8:88], "Lazul tactil virtual")
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiDevSetup, uintptr(unsafe.Pointer(&setup[0]))); e != 0 {
		fallar("setup", e)
		return
	}
	if err := ioctlUinput(fd, uiDevCreate, 0); err != nil {
		fallar("crear", err)
		return
	}
	t.f = f
	t.estado.abajo = [touchSlots]bool{}
	time.Sleep(700 * time.Millisecond)
}

func (t *tactilVirtual) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.preparar()
}

// preparar crea el dispositivo si todavía no existe. Un fallo queda guardado:
// no se reintenta en cada cuadro. Se llama con t.mu tomado.
func (t *tactilVirtual) preparar() error {
	if t.f == nil && t.err == nil {
		t.abrirDispositivo()
	}
	return t.err
}

func (t *tactilVirtual) Frame(ctx context.Context, points []platform.TouchPoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validarPuntosTactiles(points); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.preparar(); err != nil {
		return err
	}
	b, err := t.estado.eventos(points)
	if err != nil {
		return err
	}
	if _, err := t.f.Write(b); err != nil {
		return fmt.Errorf("táctil virtual: escribir: %w", err)
	}
	return nil
}

// Close levanta las dos ranuras antes de destruir el dispositivo. TouchScreen
// mantiene una interfaz mínima; quien posee el ciclo de vida puede cerrar la
// implementación mediante interface{ Close() error }.
func (t *tactilVirtual) Close() error {
	runtime.SetFinalizer(t, nil)
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cerrar()
}

// Recrear destruye el dispositivo (si existe) y lo vuelve a crear, para que
// el compositor lo vea nuevo y le aplique la salida que se le acaba de
// asignar; anchoLogico es el ancho de esa salida (0 si no se sabe). Sin
// dispositivo solo guarda el ancho: el primer Prepare ya verá la salida.
func (t *tactilVirtual) Recrear(anchoLogico int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.salida(anchoLogico)
	if t.f == nil {
		return nil
	}
	if err := t.cerrar(); err != nil {
		return err
	}
	return t.preparar()
}

// cerrar levanta los dedos y destruye el dispositivo. Se llama con t.mu
// tomado.
func (t *tactilVirtual) cerrar() error {
	if t.f == nil {
		return nil
	}
	b, _ := t.estado.eventos(nil)
	var errs []error
	if _, err := t.f.Write(b); err != nil {
		errs = append(errs, fmt.Errorf("levantar dedos: %w", err))
	}
	if err := ioctlUinput(int(t.f.Fd()), uiDevDestroy, 0); err != nil {
		errs = append(errs, fmt.Errorf("destruir: %w", err))
	}
	if err := t.f.Close(); err != nil {
		errs = append(errs, fmt.Errorf("cerrar: %w", err))
	}
	t.f = nil
	return errors.Join(errs...)
}
