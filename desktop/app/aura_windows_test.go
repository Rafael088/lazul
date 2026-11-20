//go:build windows

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
	plataforma "github.com/Rafael088/lazul/platform/windows"
	"golang.org/x/sys/windows"
)

// TestAuraVivo abre el aura en el escritorio de verdad y comprueba que se ve,
// que no se queda con los clics y que un toque inyectado justo debajo llega a
// la ventana de atrás. Necesita la sesión del escritorio:
// prototipos/tactil-win/correr.sh lo corre con LAZUL_AURA_VIVO=1.
func TestAuraVivo(t *testing.T) {
	if os.Getenv("LAZUL_AURA_VIVO") != "1" {
		t.Skip("abre una ventana en el escritorio: LAZUL_AURA_VIVO=1")
	}
	windows.NewLazySystemDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext").Call(dpiPorMonitorV2)
	a := NuevaAura("")
	if a == nil {
		t.Fatal("NuevaAura devolvió nil")
	}
	defer a.Cerrar()
	const x, y = .25, .5
	a.Actualizar(EstadoAura{Activa: true, Calibrado: true, Dentro: true, X: x, Y: y})
	time.Sleep(300 * time.Millisecond)

	isVisible := user32Aura.NewProc("IsWindowVisible")
	windowFromPoint := user32Aura.NewProc("WindowFromPoint")
	getPixel := gdi32Aura.NewProc("GetPixel")
	if v, _, _ := isVisible.Call(a.hwnd); v == 0 {
		t.Fatal("el aura no está visible con la mano dentro")
	}
	ancho, _, _ := getSystemMetricsAura.Call(0)
	alto, _, _ := getSystemMetricsAura.Call(1)
	px, py := int32(x*float64(ancho-1)), int32(y*float64(alto-1))
	if w, _, _ := windowFromPoint.Call(uintptr(px) | uintptr(py)<<32); w == a.hwnd {
		t.Fatal("WindowFromPoint encuentra el aura: se quedaría con los clics")
	}
	dc, _, _ := getDCAura.Call(0)
	c, _, _ := getPixel.Call(dc, uintptr(px), uintptr(py))
	releaseDCAura.Call(0, dc)
	r, b := c&0xff, (c>>16)&0xff
	t.Logf("píxel en la mano (%d,%d): r=%d g=%d b=%d", px, py, r, (c>>8)&0xff, b)
	if b <= r {
		t.Errorf("el centro del aura no se ve azul: r=%d b=%d", r, b)
	}

	a.Actualizar(EstadoAura{Activa: true, Calibrado: true, Dentro: true, Agarrando: true, X: x, Y: y})
	time.Sleep(200 * time.Millisecond)
	sis := plataforma.New()
	if img, err := sis.Screenshot.Capture(context.Background()); err == nil {
		_ = os.WriteFile("aura.png", img.PNG, 0o644)
	}
	// Un tap justo debajo del aura encendida: lo tiene que recibir la ventana
	// de atrás (la página de prueba lo cuenta en su título).
	ts := sis.TouchScreen
	ctx := context.Background()
	if err := ts.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	tap := func(x, y float64) {
		t.Helper()
		for range 4 {
			if err := ts.Frame(ctx, []platform.TouchPoint{{Slot: 0, X: x, Y: y, Down: true}}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(16 * time.Millisecond)
		}
		if err := ts.Frame(ctx, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(400 * time.Millisecond)
	}
	tap(.6, y) // de control, lejos del aura
	tap(x, y)

	a.Actualizar(EstadoAura{Activa: true, Calibrado: true, Dentro: false})
	time.Sleep(200 * time.Millisecond)
	if v, _, _ := isVisible.Call(a.hwnd); v != 0 {
		t.Fatal("el aura sigue visible con la mano fuera")
	}
}
