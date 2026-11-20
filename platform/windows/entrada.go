//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Entrada sintética común al puntero, las acciones táctiles y el táctil:
// SendInput para ratón y teclado, y el contexto de DPI por hilo para que las
// coordenadas en píxeles sean las físicas aunque el proceso no declare DPI.

var (
	user32Entrada                = windows.NewLazySystemDLL("user32.dll")
	sendInput                    = user32Entrada.NewProc("SendInput")
	getSystemMetrics             = user32Entrada.NewProc("GetSystemMetrics")
	setThreadDpiAwarenessContext = user32Entrada.NewProc("SetThreadDpiAwarenessContext")
	initializeTouchInjection     = user32Entrada.NewProc("InitializeTouchInjection")
	injectTouchInput             = user32Entrada.NewProc("InjectTouchInput")
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	dpiAwarenessPerMonitorV2 = ^uintptr(3)
)

const smCxScreen, smCyScreen = 0, 1

const (
	inputRaton   = 0
	inputTeclado = 1

	ratonMover     = 0x0001
	ratonIzqAbajo  = 0x0002
	ratonIzqArriba = 0x0004
	ratonRueda     = 0x0800
	ratonAbsoluto  = 0x8000
	teclaExtendida = 0x0001
	teclaSoltar    = 0x0002
	rangoAbsoluto  = 65535
)

// entradaRaton es INPUT con MOUSEINPUT en x64 (40 bytes).
type entradaRaton struct {
	tipo   uint32
	_      uint32
	dx, dy int32
	datos  uint32
	flags  uint32
	tiempo uint32
	_      uint32
	extra  uintptr
}

// entradaTeclado es INPUT con KEYBDINPUT en x64 (40 bytes: la unión mide
// lo que MOUSEINPUT).
type entradaTeclado struct {
	tipo   uint32
	_      uint32
	vk     uint16
	scan   uint16
	flags  uint32
	tiempo uint32
	_      uint32
	extra  uintptr
	_      [8]byte
}

func enviarRaton(es []entradaRaton) error {
	if len(es) == 0 {
		return nil
	}
	n, _, err := sendInput.Call(uintptr(len(es)), uintptr(unsafe.Pointer(&es[0])), unsafe.Sizeof(es[0]))
	if int(n) != len(es) {
		return fmt.Errorf("SendInput (ratón): %w", err)
	}
	return nil
}

func enviarTeclado(es []entradaTeclado) error {
	if len(es) == 0 {
		return nil
	}
	n, _, err := sendInput.Call(uintptr(len(es)), uintptr(unsafe.Pointer(&es[0])), unsafe.Sizeof(es[0]))
	if int(n) != len(es) {
		return fmt.Errorf("SendInput (teclado): %w", err)
	}
	return nil
}

// conDPIFisico corre f en un hilo fijo con conciencia de DPI por monitor:
// GetSystemMetrics e InjectTouchInput trabajan entonces en píxeles físicos,
// sin la virtualización que Windows aplica a los procesos sin DPI declarado
// (este PC está al 150 %).
func conDPIFisico(f func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if setThreadDpiAwarenessContext.Find() == nil {
		anterior, _, _ := setThreadDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2)
		if anterior != 0 {
			defer setThreadDpiAwarenessContext.Call(anterior)
		}
	}
	return f()
}

// pantallaPrincipal devuelve el tamaño de la pantalla principal en píxeles
// del contexto de DPI del hilo (físicos dentro de conDPIFisico).
func pantallaPrincipal() (int32, int32) {
	w, _, _ := getSystemMetrics.Call(smCxScreen)
	h, _, _ := getSystemMetrics.Call(smCyScreen)
	return int32(w), int32(h)
}
