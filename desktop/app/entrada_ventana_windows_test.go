//go:build windows

package main

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestEntradaVentanaViva reproduce el fallo: una ventana transparente ocupa
// la pantalla completa aunque solo se vea la pildora. Comprueba con
// WindowFromPoint que centro, cuatro bordes y cuatro esquinas de la pantalla
// pasan por debajo, que mostrar el indicador no cambia el foco y que la
// pildora conserva su boton. Necesita el escritorio real de Windows.
func TestEntradaVentanaViva(t *testing.T) {
	if os.Getenv("LAZUL_VENTANA_VIVO") != "1" {
		t.Skip("abre una ventana en el escritorio: LAZUL_VENTANA_VIVO=1")
	}
	instancia, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	clase := windows.StringToUTF16Ptr("LazulEntradaPrueba")
	proc := windows.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
		r, _, _ := defWindowProcW.Call(hwnd, msg, wparam, lparam)
		return r
	})
	c := claseVentana{proc: proc, instancia: instancia, nombre: clase}
	c.tamano = uint32(unsafe.Sizeof(c))
	if r, _, e := registerClassExW.Call(uintptr(unsafe.Pointer(&c))); r == 0 {
		t.Fatalf("registrar clase de prueba: %v", e)
	}

	const x, y = 0, 0
	anchoRaw, _, _ := getSystemMetricsAura.Call(0)
	altoRaw, _, _ := getSystemMetricsAura.Call(1)
	ancho, alto := int(int32(anchoRaw)), int(int32(altoRaw))
	if ancho < 640 || alto < 480 {
		t.Fatalf("pantalla de prueba invalida: %dx%d", ancho, alto)
	}
	hwnd, _, e := createWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(clase)), uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Lazul entrada prueba"))),
		wsPopup, x, y, uintptr(ancho), uintptr(alto), 0, 0, instancia, 0)
	if hwnd == 0 {
		t.Fatalf("crear ventana de prueba: %v", e)
	}
	v := &entradaVentanaWindows{hwnd: hwnd, preparada: true}
	v.estiloOriginal, _, _ = getWindowLongPtrWEntrada.Call(hwnd, gwlExStyle)
	defer func() {
		v.Cerrar()
		destroyWindow.Call(hwnd)
	}()

	focoAntes, _, _ := getForegroundWindowEntrada.Call()
	if err := v.indicadorBloqueado(); err != nil {
		t.Fatal(err)
	}
	showWindow.Call(hwnd, swShowNoActivate)
	focoDespues, _, _ := getForegroundWindowEntrada.Call()
	if focoAntes != 0 && focoDespues != focoAntes {
		t.Fatalf("mostrar el indicador robo el foco: antes=%#x despues=%#x", focoAntes, focoDespues)
	}

	estilo, _, _ := getWindowLongPtrWEntrada.Call(hwnd, gwlExStyle)
	if estilo&wsExNoActivate == 0 || estilo&wsExToolWindow == 0 || estilo&wsExAppWindow != 0 {
		t.Fatalf("estilo del indicador = %#x; falta NOACTIVATE/TOOLWINDOW o sobra APPWINDOW", estilo)
	}

	windowFromPoint := user32Aura.NewProc("WindowFromPoint")
	punto := func(px, py int) uintptr {
		h, _, _ := windowFromPoint.Call(uintptr(uint32(px)) | uintptr(uint32(py))<<32)
		return h
	}
	muestras := [][2]int{
		{x, y}, {x + ancho/2, y}, {x + ancho - 1, y},
		{x, y + alto/2}, {x + ancho/2, y + alto/2}, {x + ancho - 1, y + alto/2},
		{x, y + alto - 1}, {x + ancho/2, y + alto - 1}, {x + ancho - 1, y + alto - 1},
	}
	for _, p := range muestras {
		if got := punto(p[0], p[1]); got == hwnd {
			t.Errorf("la ventana transparente tapa el punto %d,%d", p[0], p[1])
		}
	}
	x0, y0, x1, y1 := rectPildora(ancho, alto)
	centroX, centroY := x+(x0+x1)/2, y+(y0+y1)/2
	if got := punto(centroX, centroY); got != hwnd {
		t.Fatalf("la pildora visible dejo de recibir el raton: WindowFromPoint=%#x", got)
	}

	if err := v.Camara(true); err != nil {
		t.Fatal(err)
	}
	if got := punto(x+ancho/2, y+alto/2); got != hwnd {
		t.Fatalf("la vista de camara no recupero toda la entrada: WindowFromPoint=%#x", got)
	}
	if err := v.Camara(false); err != nil {
		t.Fatal(err)
	}
	if got := punto(x+ancho/2, y+alto/2); got == hwnd {
		t.Fatal("al volver al indicador, la zona transparente volvio a tapar el centro")
	}
}
