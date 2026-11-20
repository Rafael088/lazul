//go:build windows

package windows

import (
	"context"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/windows"
)

// pantallasWindows implementa platform.Displays con EnumDisplayMonitors.
// Todo va en píxeles físicos de la pantalla virtual (dentro de
// conDPIFisico), los mismos que usa InjectTouchInput. Target no mueve el
// foco: solo dirige el táctil y el puntero al rectángulo de ese monitor.
type pantallasWindows struct{ destino *destinoPantalla }

var _ platform.Displays = pantallasWindows{}

// destinoPantalla es el monitor que reciben el táctil y el puntero. Sin
// destino, los dos usan la pantalla principal como siempre.
type destinoPantalla struct {
	mu      sync.Mutex
	monitor *platform.Display
}

func (d *destinoPantalla) actual() (platform.Display, bool) {
	if d == nil {
		return platform.Display{}, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.monitor == nil {
		return platform.Display{}, false
	}
	return *d.monitor, true
}

func (d *destinoPantalla) poner(m *platform.Display) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.monitor = m
}

var (
	enumDisplayMonitors = user32Entrada.NewProc("EnumDisplayMonitors")
	monitorPrimario     = uint32(1) // MONITORINFOF_PRIMARY

	enumerar          sync.Mutex
	enumerados        []monitorEnumerado
	alEnumerarMonitor = windows.NewCallback(func(hmon, _, _, _ uintptr) uintptr {
		enumerados = append(enumerados, monitorEnumerado{hmon: hmon})
		return 1
	})
)

type monitorEnumerado struct{ hmon uintptr }

// monitorInfoEx es MONITORINFOEXW: MONITORINFO más el nombre del dispositivo.
type monitorInfoEx struct {
	tamano           uint32
	monitor, trabajo rectCaptura
	flags            uint32
	dispositivo      [32]uint16
}

// pantallaDeInfo convierte MONITORINFOEXW en platform.Display.
func pantallaDeInfo(info monitorInfoEx, enFoco bool) platform.Display {
	return platform.Display{
		Name:    windows.UTF16ToString(info.dispositivo[:]),
		X:       int(info.monitor.izquierda),
		Y:       int(info.monitor.arriba),
		Width:   int(info.monitor.derecha - info.monitor.izquierda),
		Height:  int(info.monitor.abajo - info.monitor.arriba),
		Focused: enFoco,
	}
}

func listarMonitores() ([]platform.Display, error) {
	enumerar.Lock()
	enumerados = enumerados[:0]
	ok, _, err := enumDisplayMonitors.Call(0, 0, alEnumerarMonitor, 0)
	hs := append([]monitorEnumerado(nil), enumerados...)
	enumerar.Unlock()
	if ok == 0 {
		return nil, fmt.Errorf("EnumDisplayMonitors: %w", err)
	}
	ventana, _, _ := getForegroundWindowCaptura.Call()
	foco, _, _ := monitorFromWindowCaptura.Call(ventana, monitorDefaultToPrimary)
	var ds []platform.Display
	var primario = -1
	for _, h := range hs {
		info := monitorInfoEx{tamano: uint32(unsafe.Sizeof(monitorInfoEx{}))}
		if ok, _, _ := getMonitorInfoCaptura.Call(h.hmon, uintptr(unsafe.Pointer(&info))); ok == 0 {
			continue
		}
		d := pantallaDeInfo(info, h.hmon == foco)
		if d.Width <= 0 || d.Height <= 0 {
			continue
		}
		if info.flags&monitorPrimario != 0 {
			primario = len(ds)
		}
		ds = append(ds, d)
	}
	// La principal primero: Enfocado la elige si ninguna tiene el foco.
	if primario > 0 {
		ds[0], ds[primario] = ds[primario], ds[0]
	}
	return ds, nil
}

func (pantallasWindows) List(ctx context.Context) ([]platform.Display, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var ds []platform.Display
	err := conDPIFisico(func() error {
		var err error
		ds, err = listarMonitores()
		return err
	})
	return ds, err
}

func (p pantallasWindows) Target(ctx context.Context, name string) error {
	ds, err := p.List(ctx)
	if err != nil {
		return err
	}
	d, ok := platform.PorNombre(ds, name)
	if !ok {
		return fmt.Errorf("no hay monitor %q: %w", name, platform.ErrAccionInvalida)
	}
	if len(ds) == 1 {
		p.destino.poner(nil) // un solo monitor: como siempre
		return nil
	}
	p.destino.poner(&d)
	return nil
}

// zonaTactil es el rectángulo en píxeles físicos que reciben los toques.
type zonaTactil struct{ x, y, ancho, alto int32 }

// zonaDe da la zona del monitor elegido o, sin destino, la de la pantalla
// principal, que en la pantalla virtual empieza siempre en (0,0).
func zonaDe(d *destinoPantalla, principal func() (int32, int32)) zonaTactil {
	if m, ok := d.actual(); ok {
		return zonaTactil{int32(m.X), int32(m.Y), int32(m.Width), int32(m.Height)}
	}
	ancho, alto := principal()
	return zonaTactil{0, 0, ancho, alto}
}

// pixel lleva un punto normalizado (0..1) al píxel de la zona.
func (z zonaTactil) pixel(x, y float64) punto {
	return punto{
		z.x + int32(math.Round(x*float64(z.ancho-1))),
		z.y + int32(math.Round(y*float64(z.alto-1))),
	}
}

// Métricas de la pantalla virtual (todas las pantallas juntas).
const (
	smXVirtual, smYVirtual   = 76, 77
	smCxVirtual, smCyVirtual = 78, 79
	ratonEscritorio          = 0x4000 // MOUSEEVENTF_VIRTUALDESK
)

// absolutoVirtual convierte un píxel físico al rango 0..65535 de
// MOUSEEVENTF_VIRTUALDESK, que cubre la pantalla virtual entera.
func absolutoVirtual(p punto, v zonaTactil) (int32, int32) {
	norm := func(c, origen, lado int32) int32 {
		if lado <= 1 {
			return 0
		}
		r := math.Round(float64(c-origen) * rangoAbsoluto / float64(lado-1))
		return int32(min(max(r, 0), rangoAbsoluto))
	}
	return norm(p.x, v.x, v.ancho), norm(p.y, v.y, v.alto)
}

func pantallaVirtual() zonaTactil {
	x, _, _ := getSystemMetrics.Call(smXVirtual)
	y, _, _ := getSystemMetrics.Call(smYVirtual)
	w, _, _ := getSystemMetrics.Call(smCxVirtual)
	h, _, _ := getSystemMetrics.Call(smCyVirtual)
	return zonaTactil{int32(x), int32(y), int32(w), int32(h)}
}
