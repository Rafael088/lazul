//go:build windows

package main

import (
	"math"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Aura en Windows: una ventana por capas (WS_EX_LAYERED) encima de todo, que
// no recibe clics, toques ni foco (WS_EX_TRANSPARENT y WS_EX_NOACTIVATE) y no
// sale en la barra de tareas. Sin cgo ni navegador: los dos dibujos (suave y
// agarrando) se pintan una vez con pintarAura y moverla es solo
// UpdateLayeredWindow. Vive en un hilo propio con su bucle de mensajes.

var (
	user32Aura                   = windows.NewLazySystemDLL("user32.dll")
	gdi32Aura                    = windows.NewLazySystemDLL("gdi32.dll")
	registerClassExW             = user32Aura.NewProc("RegisterClassExW")
	createWindowExW              = user32Aura.NewProc("CreateWindowExW")
	defWindowProcW               = user32Aura.NewProc("DefWindowProcW")
	destroyWindow                = user32Aura.NewProc("DestroyWindow")
	getMessageW                  = user32Aura.NewProc("GetMessageW")
	translateMessage             = user32Aura.NewProc("TranslateMessage")
	dispatchMessageW             = user32Aura.NewProc("DispatchMessageW")
	postMessageW                 = user32Aura.NewProc("PostMessageW")
	postQuitMessage              = user32Aura.NewProc("PostQuitMessage")
	showWindow                   = user32Aura.NewProc("ShowWindow")
	setWindowPos                 = user32Aura.NewProc("SetWindowPos")
	updateLayeredWindow          = user32Aura.NewProc("UpdateLayeredWindow")
	getDCAura                    = user32Aura.NewProc("GetDC")
	releaseDCAura                = user32Aura.NewProc("ReleaseDC")
	getSystemMetricsAura         = user32Aura.NewProc("GetSystemMetrics")
	getDpiForWindow              = user32Aura.NewProc("GetDpiForWindow")
	setThreadDpiAwarenessCtxAura = user32Aura.NewProc("SetThreadDpiAwarenessContext")
	createCompatibleDCAura       = gdi32Aura.NewProc("CreateCompatibleDC")
	createDIBSection             = gdi32Aura.NewProc("CreateDIBSection")
	selectObjectAura             = gdi32Aura.NewProc("SelectObject")
	deleteObjectAura             = gdi32Aura.NewProc("DeleteObject")
	deleteDCAura                 = gdi32Aura.NewProc("DeleteDC")
)

const (
	wsPopup            = 0x80000000
	wsExTopmost        = 0x00000008
	wsExTransparent    = 0x00000020
	wsExToolWindow     = 0x00000080
	wsExLayered        = 0x00080000
	wsExNoActivate     = 0x08000000
	wmDestroy          = 0x0002
	wmClose            = 0x0010
	wmNcHitTest        = 0x0084
	wmMouseActivate    = 0x0021
	wmApp              = 0x8000
	htTransparent      = ^uintptr(0) // -1
	maNoActivate       = 3
	swHide             = 0
	swShowNoActivate   = 4
	swpNoSize          = 0x0001
	swpNoMove          = 0x0002
	swpNoActivate      = 0x0010
	ulwAlpha           = 0x2
	acSrcAlpha         = 0x1
	hwndTopmost        = ^uintptr(0) // -1
	dpiPorMonitorV2    = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
	mensajeAplicarAura = wmApp + 1
)

type puntoAura struct{ x, y int32 }

type mensajeAura struct {
	hwnd    uintptr
	mensaje uint32
	wparam  uintptr
	lparam  uintptr
	tiempo  uint32
	punto   puntoAura
	privado uint32
}

type claseVentana struct {
	tamano     uint32
	estilo     uint32
	proc       uintptr
	extraClase int32
	extraVent  int32
	instancia  uintptr
	icono      uintptr
	cursor     uintptr
	fondo      uintptr
	menu       *uint16
	nombre     *uint16
	iconoChico uintptr
}

type cabeceraDIB struct {
	tamano       uint32
	ancho, alto  int32
	planos, bits uint16
	compresion   uint32
	tamanoImagen uint32
	ppmX, ppmY   int32
	usados       uint32
	importantes  uint32
}

// dibujoAura es un mapa de bits seleccionado en un DC de memoria.
type dibujoAura struct {
	dc, bmp uintptr
}

// Aura es la ventana por capas que sigue a la mano. nil si no se pudo crear.
type Aura struct {
	mu        sync.Mutex
	hwnd      uintptr
	pendiente EstadoAura
	hay       bool
	cerrada   bool

	// Solo en el hilo de la ventana.
	lado      int32
	dibujos   [2]dibujoAura // suave, agarrando
	visible   bool
	agarrando bool
	ultimo    pintadoAura // lo último pintado: el aura se apaga desde ahí
}

// pintadoAura es lo necesario para repintar el aura con otra opacidad.
type pintadoAura struct {
	d       dibujoAura
	destino puntoAura
	tamano  puntoAura
}

// Fundido corto al encender y apagar el interruptor de la L: pasos de
// pasoFundidoAura, que suman unos 150 ms (el CSS de Linux usa .16 s).
const (
	pasosFundidoAura = 5
	pasoFundidoAura  = 30 * time.Millisecond
)

// pintar actualiza la ventana por capas con la opacidad constante alfa (0..255).
func (a *Aura) pintar(hwnd uintptr, p pintadoAura, alfa uint32) {
	origen := puntoAura{}
	mezcla := uint32(0) | alfa<<16 | acSrcAlpha<<24 // AC_SRC_OVER, sin flags, alfa constante, alfa por píxel
	updateLayeredWindow.Call(hwnd, 0, uintptr(unsafe.Pointer(&p.destino)), uintptr(unsafe.Pointer(&p.tamano)),
		p.d.dc, uintptr(unsafe.Pointer(&origen)), 0, uintptr(unsafe.Pointer(&mezcla)), ulwAlpha)
}

// fundir lleva la opacidad de 0 a 255 (entrando) o de 255 a 0 (saliendo).
func (a *Aura) fundir(hwnd uintptr, p pintadoAura, entrando bool) {
	for i := 1; i <= pasosFundidoAura; i++ {
		alfa := uint32(255 * i / pasosFundidoAura)
		if !entrando {
			alfa = 255 - alfa
		}
		a.pintar(hwnd, p, alfa)
		if i < pasosFundidoAura {
			time.Sleep(pasoFundidoAura)
		}
	}
}

// La clase de ventana tiene un solo procedimiento; la app abre un aura.
var (
	auraActual    *Aura
	procAura      = windows.NewCallback(procVentanaAura)
	registrarAura sync.Once
	claseAura     = windows.StringToUTF16Ptr("LazulAura")
)

func procVentanaAura(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmNcHitTest:
		return htTransparent
	case wmMouseActivate:
		return maNoActivate
	case mensajeAplicarAura:
		if a := auraActual; a != nil {
			a.aplicar()
		}
		return 0
	case wmClose:
		destroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	r, _, _ := defWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// NuevaAura crea la ventana en su propio hilo y espera a que exista. El HTML
// no se usa en Windows: la figura es la misma, dibujada en Go.
func NuevaAura(string) *Aura {
	a := &Aura{}
	listo := make(chan bool)
	go a.bucle(listo)
	if !<-listo {
		return nil
	}
	return a
}

func (a *Aura) bucle(listo chan<- bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Píxeles físicos: la posición normalizada va sobre la pantalla principal
	// igual que el táctil y el puntero.
	if setThreadDpiAwarenessCtxAura.Find() == nil {
		setThreadDpiAwarenessCtxAura.Call(dpiPorMonitorV2)
	}
	instancia, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	var errClase bool
	registrarAura.Do(func() {
		c := claseVentana{proc: procAura, instancia: instancia, nombre: claseAura}
		c.tamano = uint32(unsafe.Sizeof(c))
		if r, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&c))); r == 0 {
			errClase = true
		}
	})
	if errClase {
		listo <- false
		return
	}
	hwnd, _, _ := createWindowExW.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(claseAura)), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Lazul aura"))),
		wsPopup, 0, 0, 1, 1, 0, 0, instancia, 0)
	if hwnd == 0 {
		listo <- false
		return
	}
	escala := 1.0
	if getDpiForWindow.Find() == nil {
		if dpi, _, _ := getDpiForWindow.Call(hwnd); dpi > 0 {
			escala = float64(dpi) / 96
		}
	}
	a.lado = int32(math.Round(ladoAura * escala))
	for i, agarrando := range []bool{false, true} {
		d, ok := crearDibujo(int(a.lado), escala, agarrando)
		if !ok {
			destroyWindow.Call(hwnd)
			listo <- false
			return
		}
		a.dibujos[i] = d
	}
	a.mu.Lock()
	a.hwnd = hwnd
	a.mu.Unlock()
	auraActual = a
	listo <- true

	var m mensajeAura
	for {
		r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	for _, d := range a.dibujos {
		deleteDCAura.Call(d.dc)
		deleteObjectAura.Call(d.bmp)
	}
	if auraActual == a {
		auraActual = nil
	}
}

func crearDibujo(lado int, escala float64, agarrando bool) (dibujoAura, bool) {
	cab := cabeceraDIB{ancho: int32(lado), alto: -int32(lado), planos: 1, bits: 32}
	cab.tamano = uint32(unsafe.Sizeof(cab))
	pantalla, _, _ := getDCAura.Call(0)
	defer releaseDCAura.Call(0, pantalla)
	var bits unsafe.Pointer
	bmp, _, _ := createDIBSection.Call(pantalla, uintptr(unsafe.Pointer(&cab)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return dibujoAura{}, false
	}
	copy(unsafe.Slice((*byte)(bits), lado*lado*4), pintarAura(lado, escala, agarrando))
	dc, _, _ := createCompatibleDCAura.Call(pantalla)
	if dc == 0 {
		deleteObjectAura.Call(bmp)
		return dibujoAura{}, false
	}
	selectObjectAura.Call(dc, bmp)
	return dibujoAura{dc: dc, bmp: bmp}, true
}

// Actualizar deja el último estado y despierta al hilo de la ventana.
func (a *Aura) Actualizar(e EstadoAura) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cerrada || a.hwnd == 0 {
		return
	}
	avisar := !a.hay
	a.pendiente, a.hay = e, true
	if avisar {
		postMessageW.Call(a.hwnd, mensajeAplicarAura, 0, 0)
	}
}

// aplicar corre en el hilo de la ventana con el último estado pendiente.
func (a *Aura) aplicar() {
	a.mu.Lock()
	e, hay, hwnd := a.pendiente, a.hay, a.hwnd
	a.hay = false
	a.mu.Unlock()
	if !hay {
		return
	}
	mostrar := e.Activa && e.Calibrado && e.Dentro
	if !mostrar {
		if a.visible {
			a.fundir(hwnd, a.ultimo, false)
			showWindow.Call(hwnd, swHide)
			a.visible = false
		}
		return
	}
	ancho, _, _ := getSystemMetricsAura.Call(0)
	alto, _, _ := getSystemMetricsAura.Call(1)
	x := int32(math.Round(e.X*float64(int32(ancho)-1))) - a.lado/2
	y := int32(math.Round(e.Y*float64(int32(alto)-1))) - a.lado/2
	d := a.dibujos[0]
	if e.Agarrando {
		d = a.dibujos[1]
	}
	p := pintadoAura{d: d, destino: puntoAura{x, y}, tamano: puntoAura{a.lado, a.lado}}
	a.ultimo = p
	if !a.visible {
		// Entra con fundido: primero transparente, ya visible, y sube.
		a.pintar(hwnd, p, 0)
		showWindow.Call(hwnd, swShowNoActivate)
		setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		a.visible = true
		a.fundir(hwnd, p, true)
		return
	}
	a.pintar(hwnd, p, 255)
}

// Cerrar destruye la ventana desde su hilo.
func (a *Aura) Cerrar() {
	if a == nil {
		return
	}
	a.mu.Lock()
	hwnd := a.hwnd
	a.cerrada = true
	a.mu.Unlock()
	if hwnd != 0 {
		postMessageW.Call(hwnd, wmClose, 0, 0)
	}
}
