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
)

var _ platform.TouchScreen = (*tactilVirtual)(nil)

// estadoTactil contiene solo el estado necesario para convertir cuadros en
// input_event. Está separado del dispositivo para probar los bytes sin abrir
// /dev/uinput.
type estadoTactil struct {
	abajo [touchSlots]bool
	sig   int32
}

func nuevoEstadoTactil() estadoTactil { return estadoTactil{sig: 1} }

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
	abrir  sync.Once
	estado estadoTactil
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
	time.Sleep(700 * time.Millisecond)
}

func (t *tactilVirtual) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.abrir.Do(t.abrirDispositivo)
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
