//go:build linux

package linux

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ruedaVirtual es un ratón virtual (uinput) que solo mueve la rueda. Así el
// scroll continuo es la rueda de verdad, en alta resolución, y no flechas:
// cada app lo desplaza con su propia suavidad y velocidad.
//
// /dev/uinput lo puede escribir el usuario de la sesión (ACL de logind). Si
// no se puede abrir, ScrollContinuo devuelve el error y la app vuelve al
// scroll por pasos con flechas.
type ruedaVirtual struct {
	mu     sync.Mutex
	f      *os.File
	err    error
	abrir  sync.Once
	restos int // unidades de alta resolución que aún no suman un paso entero
}

// Constantes de linux/input-event-codes.h y linux/uinput.h.
const (
	evSyn         = 0x00
	evKey         = 0x01
	evRel         = 0x02
	synReport     = 0x00
	relX          = 0x00
	relY          = 0x01
	relWheel      = 0x08
	relWheelHiRes = 0x0b
	btnLeft       = 0x110
	busVirtual    = 0x06

	uiSetEvbit  = 0x40045564 // _IOW('U', 100, int)
	uiSetKeybit = 0x40045565 // _IOW('U', 101, int)
	uiSetRelbit = 0x40045566 // _IOW('U', 102, int)
	uiDevSetup  = 0x405c5503 // _IOW('U', 3, struct uinput_setup) (92 bytes)
	uiDevCreate = 0x5501     // _IO('U', 1)

	unidadesPorPaso = 120
)

func (r *ruedaVirtual) abrirDispositivo() {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		r.err = fmt.Errorf("rueda virtual: %w", err)
		return
	}
	fd := int(f.Fd())
	ioctl := func(req, val uintptr) error {
		if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, val); e != 0 {
			return e
		}
		return nil
	}
	// libinput solo trata como puntero a un dispositivo con botón y
	// movimiento relativo; la rueda sola no basta.
	pasos := []struct{ req, val uintptr }{
		{uiSetEvbit, evKey}, {uiSetKeybit, btnLeft},
		{uiSetEvbit, evRel}, {uiSetRelbit, relX}, {uiSetRelbit, relY},
		{uiSetRelbit, relWheel}, {uiSetRelbit, relWheelHiRes},
	}
	for _, p := range pasos {
		if err := ioctl(p.req, p.val); err != nil {
			f.Close()
			r.err = fmt.Errorf("rueda virtual: configurar: %w", err)
			return
		}
	}
	// struct uinput_setup { struct input_id {bustype, vendor, product, version u16}; char name[80]; u32 ff_effects_max; }
	setup := make([]byte, 92)
	binary.LittleEndian.PutUint16(setup[0:], busVirtual)
	binary.LittleEndian.PutUint16(setup[2:], 0x1209) // id de pruebas de pid.codes
	binary.LittleEndian.PutUint16(setup[4:], 0x4c5a) // «LZ»
	binary.LittleEndian.PutUint16(setup[6:], 1)
	copy(setup[8:88], "Lazul rueda virtual")
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiDevSetup, uintptr(unsafe.Pointer(&setup[0]))); e != 0 {
		f.Close()
		r.err = fmt.Errorf("rueda virtual: setup: %w", e)
		return
	}
	if err := ioctl(uiDevCreate, 0); err != nil {
		f.Close()
		r.err = fmt.Errorf("rueda virtual: crear: %w", err)
		return
	}
	r.f = f
}

// Preparar crea el dispositivo antes de necesitarlo: el compositor tarda un
// momento en reconocerlo y los primeros eventos se perderían.
func (r *ruedaVirtual) Preparar() error {
	r.abrir.Do(r.abrirDispositivo)
	return r.err
}

// eventos arma los input_event (24 bytes en 64 bits: timeval, tipo, código,
// valor) de un desplazamiento. REL_WHEEL lleva los pasos enteros para las
// apps antiguas; REL_WHEEL_HI_RES, el detalle. La rueda positiva sube, así
// que unidades positivas (hacia abajo) van con signo negativo.
func (r *ruedaVirtual) eventos(unidades int) []byte {
	r.restos += unidades
	pasos := r.restos / unidadesPorPaso
	r.restos -= pasos * unidadesPorPaso
	var b []byte
	ev := func(tipo, codigo uint16, valor int32) {
		e := make([]byte, 24)
		binary.LittleEndian.PutUint16(e[16:], tipo)
		binary.LittleEndian.PutUint16(e[18:], codigo)
		binary.LittleEndian.PutUint32(e[20:], uint32(valor))
		b = append(b, e...)
	}
	ev(evRel, relWheelHiRes, int32(-unidades))
	if pasos != 0 {
		ev(evRel, relWheel, int32(-pasos))
	}
	ev(evSyn, synReport, 0)
	return b
}

func (r *ruedaVirtual) ScrollContinuo(ctx context.Context, unidades int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.Preparar(); err != nil {
		return err
	}
	if unidades == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.f.Write(r.eventos(unidades)); err != nil {
		return fmt.Errorf("rueda virtual: escribir: %w", err)
	}
	return nil
}
