//go:build linux

// Prueba de factibilidad: una pantalla táctil virtual (uinput, multitoque
// protocolo B) para que las apps reciban toques de verdad en vez de clics,
// flechas o rueda. Coordenadas normalizadas 0..1 sobre el monitor.
//
//	go run ./prototipos/tactil tap 0.5 0.5
//	go run ./prototipos/tactil doble 0.5 0.5
//	go run ./prototipos/tactil largo 0.5 0.5 [ms]
//	go run ./prototipos/tactil arrastrar 0.5 0.7 0.5 0.3 [ms]
//	go run ./prototipos/tactil pellizco 0.5 0.5 0.1 0.3 [ms]   (separación inicial → final)
//	go run ./prototipos/tactil guion archivo                     (una orden por línea; un solo dispositivo)
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	evSyn = 0x00
	evKey = 0x01
	evAbs = 0x03

	synReport = 0x00
	btnTouch  = 0x14a
	btnTool1  = 0x145 // BTN_TOOL_FINGER
	btnTool2  = 0x14d // BTN_TOOL_DOUBLETAP

	absX          = 0x00
	absY          = 0x01
	absMtSlot     = 0x2f
	absMtX        = 0x35
	absMtY        = 0x36
	absMtTracking = 0x39

	propDirect = 0x01

	uiSetEvbit   = 0x40045564
	uiSetKeybit  = 0x40045565
	uiSetAbsbit  = 0x40045567
	uiSetPropbit = 0x4004556e
	uiAbsSetup   = 0x401c5504 // _IOW('U', 4, struct uinput_abs_setup) (28 bytes)
	uiDevSetup   = 0x405c5503
	uiDevCreate  = 0x5501
	uiDevDestroy = 0x5502

	maxX, maxY = 2880, 1800 // resolución del panel; la escala la pone libinput
	ranuras    = 2
)

type tactil struct {
	f   *os.File
	sig int32 // siguiente tracking id
	buf []byte
}

func ioctl(fd int, req, val uintptr) error {
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, val); e != 0 {
		return e
	}
	return nil
}

func crear() (*tactil, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd())
	for _, p := range []struct{ req, val uintptr }{
		{uiSetEvbit, evKey}, {uiSetKeybit, btnTouch}, {uiSetKeybit, btnTool1}, {uiSetKeybit, btnTool2},
		{uiSetEvbit, evAbs},
		{uiSetAbsbit, absX}, {uiSetAbsbit, absY},
		{uiSetAbsbit, absMtSlot}, {uiSetAbsbit, absMtX}, {uiSetAbsbit, absMtY}, {uiSetAbsbit, absMtTracking},
		{uiSetPropbit, propDirect},
	} {
		if err := ioctl(fd, p.req, p.val); err != nil {
			f.Close()
			return nil, fmt.Errorf("configurar %#x: %w", p.val, err)
		}
	}
	// struct uinput_abs_setup { u16 code; (2 de relleno) struct input_absinfo {value,min,max,fuzz,flat,res s32} }
	eje := func(code uint16, max, res int32) error {
		b := make([]byte, 28)
		binary.LittleEndian.PutUint16(b[0:], code)
		binary.LittleEndian.PutUint32(b[8:], 0)
		binary.LittleEndian.PutUint32(b[12:], uint32(max))
		binary.LittleEndian.PutUint32(b[24:], uint32(res))
		_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiAbsSetup, uintptr(unsafe.Pointer(&b[0])))
		if e != 0 {
			return e
		}
		return nil
	}
	for _, e := range []struct {
		c   uint16
		max int32
	}{{absX, maxX}, {absY, maxY}, {absMtX, maxX}, {absMtY, maxY}, {absMtSlot, ranuras - 1}, {absMtTracking, 65535}} {
		res := int32(10) // unidades por mm: 288 mm de ancho, como una pantalla de 13"
		if e.c == absMtSlot || e.c == absMtTracking {
			res = 0
		}
		if err := eje(e.c, e.max, res); err != nil {
			f.Close()
			return nil, fmt.Errorf("eje %#x: %w", e.c, err)
		}
	}
	setup := make([]byte, 92)
	binary.LittleEndian.PutUint16(setup[0:], 0x06)
	binary.LittleEndian.PutUint16(setup[2:], 0x1209)
	binary.LittleEndian.PutUint16(setup[4:], 0x4c5b)
	binary.LittleEndian.PutUint16(setup[6:], 1)
	copy(setup[8:88], "Lazul tactil virtual")
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uiDevSetup, uintptr(unsafe.Pointer(&setup[0]))); e != 0 {
		f.Close()
		return nil, fmt.Errorf("setup: %w", e)
	}
	if err := ioctl(fd, uiDevCreate, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("crear: %w", err)
	}
	time.Sleep(700 * time.Millisecond) // que el compositor lo reconozca
	return &tactil{f: f, sig: 1}, nil
}

func (t *tactil) ev(tipo, code uint16, v int32) {
	e := make([]byte, 24)
	binary.LittleEndian.PutUint16(e[16:], tipo)
	binary.LittleEndian.PutUint16(e[18:], code)
	binary.LittleEndian.PutUint32(e[20:], uint32(v))
	t.buf = append(t.buf, e...)
}

func (t *tactil) sync() {
	t.ev(evSyn, synReport, 0)
	if _, err := t.f.Write(t.buf); err != nil {
		fmt.Fprintln(os.Stderr, "escribir:", err)
	}
	t.buf = t.buf[:0]
}

type punto struct{ x, y float64 }

func px(v float64, max int) int32 { return int32(v * float64(max)) }

// cuadro manda el estado de los dedos: nil = dedo levantado.
// abajo indica qué ranuras estaban apoyadas en el cuadro anterior.
func (t *tactil) cuadro(dedos []*punto, abajo []bool) {
	n := 0
	for i, d := range dedos {
		t.ev(evAbs, absMtSlot, int32(i))
		switch {
		case d != nil && !abajo[i]:
			t.ev(evAbs, absMtTracking, t.sig)
			t.sig++
			fallthrough
		case d != nil:
			t.ev(evAbs, absMtX, px(d.x, maxX))
			t.ev(evAbs, absMtY, px(d.y, maxY))
		case abajo[i]:
			t.ev(evAbs, absMtTracking, -1)
		}
		if d != nil {
			if n == 0 {
				t.ev(evAbs, absX, px(d.x, maxX))
				t.ev(evAbs, absY, px(d.y, maxY))
			}
			n++
		}
	}
	antes := 0
	for _, a := range abajo {
		if a {
			antes++
		}
	}
	if (n > 0) != (antes > 0) {
		t.ev(evKey, btnTouch, b2i(n > 0))
	}
	t.ev(evKey, btnTool1, b2i(n == 1))
	t.ev(evKey, btnTool2, b2i(n == 2))
	t.sync()
	for i, d := range dedos {
		abajo[i] = d != nil
	}
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

const paso = 8 * time.Millisecond // ~120 Hz

// trazo mueve los dedos de a hacia b en dur, apoyando al inicio y levantando al final.
func (t *tactil) trazo(a, b []punto, dur time.Duration) {
	abajo := make([]bool, ranuras)
	pasos := int(dur / paso)
	if pasos < 1 {
		pasos = 1
	}
	for k := 0; k <= pasos; k++ {
		f := float64(k) / float64(pasos)
		dedos := make([]*punto, ranuras)
		for i := range a {
			dedos[i] = &punto{a[i].x + (b[i].x-a[i].x)*f, a[i].y + (b[i].y-a[i].y)*f}
		}
		t.cuadro(dedos, abajo)
		time.Sleep(paso)
	}
	t.cuadro(make([]*punto, ranuras), abajo)
}

func num(s []string, i int, def float64) float64 {
	if i >= len(s) {
		return def
	}
	v, err := strconv.ParseFloat(s[i], 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "número inválido %q\n", s[i])
		os.Exit(2)
	}
	return v
}

func ms(v float64) time.Duration { return time.Duration(v) * time.Millisecond }

func (t *tactil) orden(a []string) error {
	if len(a) == 0 || strings.HasPrefix(a[0], "#") {
		return nil
	}
	p := punto{num(a, 1, .5), num(a, 2, .5)}
	switch a[0] {
	case "tap":
		t.trazo([]punto{p}, []punto{p}, ms(60))
	case "doble":
		t.trazo([]punto{p}, []punto{p}, ms(60))
		time.Sleep(120 * time.Millisecond)
		t.trazo([]punto{p}, []punto{p}, ms(60))
	case "largo":
		t.trazo([]punto{p}, []punto{p}, ms(num(a, 3, 900)))
	case "arrastrar":
		q := punto{num(a, 3, .5), num(a, 4, .3)}
		t.trazo([]punto{p}, []punto{q}, ms(num(a, 5, 400)))
	case "pellizco":
		d0, d1 := num(a, 3, .1)/2, num(a, 4, .3)/2
		t.trazo(
			[]punto{{p.x - d0, p.y}, {p.x + d0, p.y}},
			[]punto{{p.x - d1, p.y}, {p.x + d1, p.y}},
			ms(num(a, 5, 600)))
	case "esperar":
		time.Sleep(ms(num(a, 1, 500)))
	default:
		return fmt.Errorf("orden desconocida %q", a[0])
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: tactil tap|doble|largo|arrastrar|pellizco|guion ...")
		os.Exit(2)
	}
	t, err := crear()
	if err != nil {
		fmt.Fprintln(os.Stderr, "táctil virtual:", err)
		os.Exit(1)
	}
	defer func() { ioctl(int(t.f.Fd()), uiDevDestroy, 0); t.f.Close() }()
	if os.Args[1] == "guion" {
		f, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if err := t.orden(strings.Fields(sc.Text())); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		return
	}
	if err := t.orden(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	time.Sleep(200 * time.Millisecond)
}
