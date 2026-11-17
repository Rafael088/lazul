//go:build linux

package linux

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sync"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/unix"
)

// punteroVirtual es un dispositivo uinput con ejes absolutos. El compositor
// convierte su rango 0..65535 al escritorio completo, sin acumular errores
// como ocurriría con movimientos relativos.
type punteroVirtual struct {
	mu    sync.Mutex
	f     *os.File
	err   error
	abrir sync.Once
}

const (
	evAbs            = 0x03
	absX             = 0x00
	absY             = 0x01
	uiSetAbsbit      = 0x40045567 // _IOW('U', 103, int)
	uiAbsSetup       = 0x401c5504 // _IOW('U', 4, struct uinput_abs_setup) (28 bytes)
	uiSetPropbit     = 0x4004556e // _IOW('U', 110, int)
	inputPropPointer = 0x00       // los ejes absolutos controlan un puntero, no una pantalla táctil
	maxAbs           = 65535
)

var _ platform.AbsolutePointer = (*punteroVirtual)(nil)

func ioctlUinput(fd int, req, val uintptr) error {
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, val); e != 0 {
		return e
	}
	return nil
}

func configurarEjeAbsoluto(fd int, codigo uint16) error {
	// struct uinput_abs_setup: code u16, 2 bytes de relleno, y después
	// input_absinfo { value, minimum, maximum, fuzz, flat, resolution } i32.
	a := make([]byte, 28)
	binary.LittleEndian.PutUint16(a[0:], codigo)
	binary.LittleEndian.PutUint32(a[8:], 0)
	binary.LittleEndian.PutUint32(a[12:], maxAbs)
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiAbsSetup, uintptr(unsafe.Pointer(&a[0]))); e != 0 {
		return e
	}
	return nil
}

func (p *punteroVirtual) abrirDispositivo() {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		p.err = fmt.Errorf("puntero virtual: %w", err)
		return
	}
	fd := int(f.Fd())
	pasos := []struct{ req, val uintptr }{
		{uiSetPropbit, inputPropPointer},
		{uiSetEvbit, evKey}, {uiSetKeybit, btnLeft},
		{uiSetEvbit, evAbs}, {uiSetAbsbit, absX}, {uiSetAbsbit, absY},
	}
	for _, paso := range pasos {
		if err := ioctlUinput(fd, paso.req, paso.val); err != nil {
			f.Close()
			p.err = fmt.Errorf("puntero virtual: configurar: %w", err)
			return
		}
	}
	for _, eje := range []uint16{absX, absY} {
		if err := configurarEjeAbsoluto(fd, eje); err != nil {
			f.Close()
			p.err = fmt.Errorf("puntero virtual: configurar eje: %w", err)
			return
		}
	}
	setup := make([]byte, 92)
	binary.LittleEndian.PutUint16(setup[0:], busVirtual)
	binary.LittleEndian.PutUint16(setup[2:], 0x1209)
	binary.LittleEndian.PutUint16(setup[4:], 0x4c5b)
	binary.LittleEndian.PutUint16(setup[6:], 1)
	copy(setup[8:88], "Lazul puntero absoluto")
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiDevSetup, uintptr(unsafe.Pointer(&setup[0]))); e != 0 {
		f.Close()
		p.err = fmt.Errorf("puntero virtual: setup: %w", e)
		return
	}
	if err := ioctlUinput(fd, uiDevCreate, 0); err != nil {
		f.Close()
		p.err = fmt.Errorf("puntero virtual: crear: %w", err)
		return
	}
	p.f = f
}

func (p *punteroVirtual) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.abrir.Do(p.abrirDispositivo)
	return p.err
}

func eventosPuntero(x, y float64) ([]byte, error) {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < 0 || x > 1 || y < 0 || y > 1 {
		return nil, platform.ErrAccionInvalida
	}
	var b []byte
	evento := func(tipo, codigo uint16, valor int32) {
		e := make([]byte, 24)
		binary.LittleEndian.PutUint16(e[16:], tipo)
		binary.LittleEndian.PutUint16(e[18:], codigo)
		binary.LittleEndian.PutUint32(e[20:], uint32(valor))
		b = append(b, e...)
	}
	evento(evAbs, absX, int32(math.Round(x*maxAbs)))
	evento(evAbs, absY, int32(math.Round(y*maxAbs)))
	evento(evSyn, synReport, 0)
	return b, nil
}

// eventosBoton arma los input_event de BTN_LEFT. El botón es el embrague del
// pellizco: se pulsa al agarrar y se suelta al soltar.
func eventosBoton(down bool) []byte {
	valor := int32(0)
	if down {
		valor = 1
	}
	e := make([]byte, 24)
	binary.LittleEndian.PutUint16(e[16:], evKey)
	binary.LittleEndian.PutUint16(e[18:], btnLeft)
	binary.LittleEndian.PutUint32(e[20:], uint32(valor))
	syn := make([]byte, 24)
	binary.LittleEndian.PutUint16(syn[16:], evSyn)
	binary.LittleEndian.PutUint16(syn[18:], synReport)
	return append(e, syn...)
}

// Button pulsa o suelta el botón principal del puntero virtual.
func (p *punteroVirtual) Button(ctx context.Context, down bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.Prepare(ctx); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.f.Write(eventosBoton(down)); err != nil {
		return fmt.Errorf("puntero virtual: botón: %w", err)
	}
	return nil
}

func (p *punteroVirtual) MoveAbsolute(ctx context.Context, x, y float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := eventosPuntero(x, y)
	if err != nil {
		return err
	}
	if err := p.Prepare(ctx); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.f.Write(b); err != nil {
		return fmt.Errorf("puntero virtual: escribir: %w", err)
	}
	return nil
}
