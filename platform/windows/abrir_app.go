//go:build windows

package windows

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/windows"
)

type abrirApp struct{}

var (
	shell32AbrirApp = windows.NewLazySystemDLL("shell32.dll")
	shellExecute    = shell32AbrirApp.NewProc("ShellExecuteW")
)

func ejecutarShell(objetivo string) error {
	operacion, _ := windows.UTF16PtrFromString("open")
	destino, err := windows.UTF16PtrFromString(objetivo)
	if err != nil {
		return fmt.Errorf("referencia contiene NUL: %w", err)
	}
	r, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(operacion)), uintptr(unsafe.Pointer(destino)), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("ShellExecute(%q) falló con código %d", objetivo, r)
	}
	return nil
}

func (abrirApp) Launch(ctx context.Context, ref platform.AppRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch {
	case ref.URL != "":
		return ejecutarShell(ref.URL)
	case ref.Archivo != "":
		if _, err := os.Stat(ref.Archivo); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("archivo %q: %w", ref.Archivo, platform.ErrNoEncontrada)
			}
			return fmt.Errorf("consultar archivo %q: %w", ref.Archivo, err)
		}
		return ejecutarShell(ref.Archivo)
	case ref.ID != "":
		objetivo := ref.ID
		if strings.Contains(ref.ID, "!") { // AUMID de una app empaquetada.
			objetivo = `shell:AppsFolder\` + ref.ID
		} else {
			ruta, err := exec.LookPath(ref.ID)
			if err != nil {
				return fmt.Errorf("app %q: %w", ref.ID, platform.ErrNoEncontrada)
			}
			objetivo = ruta
		}
		return ejecutarShell(objetivo)
	default:
		return platform.ErrRefInvalida
	}
}
