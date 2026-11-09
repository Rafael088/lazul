//go:build windows

package windows

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/windows"
)

type abrirApp struct{}

var (
	shell32AbrirApp = windows.NewLazySystemDLL("shell32.dll")
	shellExecute    = shell32AbrirApp.NewProc("ShellExecuteW")
)

// esperaLanzar: lo que se espera a que ShellExecute conteste. Abrir en
// pantalla tarda 260-545 ms; un fallo inmediato (código <= 32) llega mucho
// antes. Si en este tiempo falla se devuelve el error; si no, se da por
// abierto y la llamada termina sola en segundo plano.
var esperaLanzar = 25 * time.Millisecond

// ejecutarShell lanza el objetivo con ShellExecute sin esperar a que la
// ventana aparezca (ver esperaLanzar). La llamada corre en su propio hilo y
// no queda colgada: cuando ShellExecute vuelve, la goroutine termina.
func ejecutarShell(ctx context.Context, objetivo string) error {
	operacion, _ := windows.UTF16PtrFromString("open")
	destino, err := windows.UTF16PtrFromString(objetivo)
	if err != nil {
		return fmt.Errorf("referencia contiene NUL: %w", err)
	}
	fin := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		r, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(operacion)), uintptr(unsafe.Pointer(destino)), 0, 0, 1)
		if r <= 32 {
			fin <- fmt.Errorf("ShellExecute(%q) falló con código %d", objetivo, r)
			return
		}
		fin <- nil
	}()
	temporizador := time.NewTimer(esperaLanzar)
	defer temporizador.Stop()
	select {
	case err := <-fin:
		return err
	case <-temporizador.C:
		return nil
	case <-ctx.Done():
		return nil // ya se lanzó; la goroutine termina sola
	}
}

// rutas recuerda dónde se encontró cada ejecutable: LookPath recorre el PATH
// y, en frío, tarda hasta ~200 ms (más que el propio lanzamiento).
var rutas sync.Map

func buscarEjecutable(nombre string) (string, error) {
	if r, ok := rutas.Load(nombre); ok {
		if _, err := os.Stat(r.(string)); err == nil {
			return r.(string), nil
		}
		rutas.Delete(nombre)
	}
	ruta, err := exec.LookPath(nombre)
	if err == nil {
		rutas.Store(nombre, ruta)
	}
	return ruta, err
}

func (abrirApp) Launch(ctx context.Context, ref platform.AppRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch {
	case ref.URL != "":
		return ejecutarShell(ctx, ref.URL)
	case ref.Archivo != "":
		if _, err := os.Stat(ref.Archivo); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("archivo %q: %w", ref.Archivo, platform.ErrNoEncontrada)
			}
			return fmt.Errorf("consultar archivo %q: %w", ref.Archivo, err)
		}
		return ejecutarShell(ctx, ref.Archivo)
	case ref.ID != "":
		objetivo := ref.ID
		if strings.Contains(ref.ID, "!") { // AUMID de una app empaquetada.
			objetivo = `shell:AppsFolder\` + ref.ID
		} else {
			ruta, err := buscarEjecutable(ref.ID)
			if err != nil {
				return fmt.Errorf("app %q: %w", ref.ID, platform.ErrNoEncontrada)
			}
			objetivo = ruta
		}
		return ejecutarShell(ctx, objetivo)
	default:
		return platform.ErrRefInvalida
	}
}
