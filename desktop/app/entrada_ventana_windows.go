//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	iniciarVentanaOculta = true

	gwlExStyle         = ^uintptr(19) // -20
	wsExAppWindow      = 0x00040000
	swpNoZOrderEntrada = 0x0004
	swpFrameChanged    = 0x0020
	regionRedonda      = 52
)

var (
	enumWindowsEntrada              = user32Aura.NewProc("EnumWindows")
	getClassNameWEntrada            = user32Aura.NewProc("GetClassNameW")
	getWindowThreadProcessIDEntrada = user32Aura.NewProc("GetWindowThreadProcessId")
	getWindowLongPtrWEntrada        = user32Aura.NewProc("GetWindowLongPtrW")
	setWindowLongPtrWEntrada        = user32Aura.NewProc("SetWindowLongPtrW")
	setWindowRgnEntrada             = user32Aura.NewProc("SetWindowRgn")
	getClientRectEntrada            = user32Aura.NewProc("GetClientRect")
	getForegroundWindowEntrada      = user32Aura.NewProc("GetForegroundWindow")
	setForegroundWindowEntrada      = user32Aura.NewProc("SetForegroundWindow")
	isWindowEntrada                 = user32Aura.NewProc("IsWindow")
	setLastErrorEntrada             = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetLastError")
	createRoundRectRgnEntrada       = gdi32Aura.NewProc("CreateRoundRectRgn")
	deleteObjectEntrada             = gdi32Aura.NewProc("DeleteObject")
)

type rectWindows struct {
	izquierda, arriba, derecha, abajo int32
}

// entradaVentanaWindows conserva solo la parte del estilo que modifica. El
// HWND se obtiene por PID y por la clase exclusiva configurada en Wails; no
// depende del titulo, que se puede cambiar con -titulo.
type entradaVentanaWindows struct {
	mu              sync.Mutex
	hwnd            uintptr
	estiloOriginal  uintptr
	preparada       bool
	ventanaAnterior uintptr
}

func nuevaEntradaVentana() entradaVentana { return &entradaVentanaWindows{} }

func buscarVentanaLazul(pid uint32) uintptr {
	var encontrada uintptr
	callback := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		var ventanaPID uint32
		getWindowThreadProcessIDEntrada.Call(hwnd, uintptr(unsafe.Pointer(&ventanaPID)))
		if ventanaPID != pid {
			return 1
		}
		var nombre [64]uint16
		n, _, _ := getClassNameWEntrada.Call(hwnd, uintptr(unsafe.Pointer(&nombre[0])), uintptr(len(nombre)))
		if n > 0 && windows.UTF16ToString(nombre[:n]) == claseVentanaLazul {
			encontrada = hwnd
			return 0
		}
		return 1
	})
	enumWindowsEntrada.Call(callback, 0)
	return encontrada
}

func (v *entradaVentanaWindows) Preparar() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.preparada {
		return nil
	}
	v.hwnd = buscarVentanaLazul(uint32(os.Getpid()))
	if v.hwnd == 0 {
		return fmt.Errorf("no se encontro la ventana %s del proceso", claseVentanaLazul)
	}
	v.estiloOriginal, _, _ = getWindowLongPtrWEntrada.Call(v.hwnd, gwlExStyle)
	if err := v.indicadorBloqueado(); err != nil {
		setWindowRgnEntrada.Call(v.hwnd, 0, 0)
		_ = v.ponerEstilo(v.estiloOriginal)
		v.hwnd = 0
		return err
	}
	v.preparada = true
	return nil
}

func (v *entradaVentanaWindows) Mostrar(context.Context) error {
	// OnStartup y OnDomReady corren en goroutines distintas en Wails. Repetir
	// Preparar aqui cierra la carrera: nunca se muestra antes de tener estilo y
	// region, aunque la pagina termine de cargar de inmediato.
	if err := v.Preparar(); err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	showWindow.Call(v.hwnd, swShowNoActivate)
	return nil
}

func (v *entradaVentanaWindows) Camara(abierta bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.preparada {
		return nil
	}
	if abierta {
		v.ventanaAnterior, _, _ = getForegroundWindowEntrada.Call()
		if r, _, e := setWindowRgnEntrada.Call(v.hwnd, 0, 1); r == 0 {
			return fmt.Errorf("quitar la region del indicador: %v", e)
		}
		if err := v.ponerEstilo(v.estiloOriginal); err != nil {
			return err
		}
		showWindow.Call(v.hwnd, swShowNoActivate)
		setForegroundWindowEntrada.Call(v.hwnd)
		return nil
	}
	if err := v.indicadorBloqueado(); err != nil {
		return err
	}
	showWindow.Call(v.hwnd, swShowNoActivate)
	if v.ventanaAnterior != 0 && v.ventanaAnterior != v.hwnd {
		if ok, _, _ := isWindowEntrada.Call(v.ventanaAnterior); ok != 0 {
			setForegroundWindowEntrada.Call(v.ventanaAnterior)
		}
	}
	v.ventanaAnterior = 0
	return nil
}

func (v *entradaVentanaWindows) indicadorBloqueado() error {
	// NOACTIVATE evita que un clic en el boton de camara cambie la ventana con
	// foco; TOOLWINDOW evita que la pildora auxiliar aparezca en Alt+Tab. La
	// vista de camara restaura exactamente el estilo que puso Wails.
	estilo := (v.estiloOriginal | wsExNoActivate | wsExToolWindow) &^ wsExAppWindow
	if err := v.ponerEstilo(estilo); err != nil {
		return err
	}
	var r rectWindows
	if ok, _, e := getClientRectEntrada.Call(v.hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return fmt.Errorf("medir la ventana del indicador: %v", e)
	}
	x0, y0, x1, y1 := rectPildora(int(r.derecha-r.izquierda), int(r.abajo-r.arriba))
	region, _, e := createRoundRectRgnEntrada.Call(uintptr(x0), uintptr(y0), uintptr(x1+1), uintptr(y1+1), regionRedonda, regionRedonda)
	if region == 0 {
		return fmt.Errorf("crear la region del indicador: %v", e)
	}
	if ok, _, e := setWindowRgnEntrada.Call(v.hwnd, region, 1); ok == 0 {
		deleteObjectEntrada.Call(region)
		return fmt.Errorf("aplicar la region del indicador: %v", e)
	}
	// Al tener exito, Windows pasa a ser dueno de region.
	return nil
}

func (v *entradaVentanaWindows) ponerEstilo(estilo uintptr) error {
	setLastErrorEntrada.Call(0)
	anterior, _, e := setWindowLongPtrWEntrada.Call(v.hwnd, gwlExStyle, estilo)
	if anterior == 0 && e != windows.ERROR_SUCCESS {
		return fmt.Errorf("cambiar el estilo del indicador: %v", e)
	}
	if ok, _, e := setWindowPos.Call(v.hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrderEntrada|swpNoActivate|swpFrameChanged); ok == 0 {
		return fmt.Errorf("actualizar el estilo del indicador: %v", e)
	}
	return nil
}

func (v *entradaVentanaWindows) Cerrar() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.preparada {
		return
	}
	setWindowRgnEntrada.Call(v.hwnd, 0, 0)
	_ = v.ponerEstilo(v.estiloOriginal)
	v.preparada = false
}
