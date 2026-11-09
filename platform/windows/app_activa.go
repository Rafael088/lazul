//go:build windows

package windows

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/windows"
)

// appActiva implementa platform.ActiveApp con la ventana en primer plano; la
// URL y el documento abierto salen de niveles.go.
type appActiva struct{}

var (
	user32AppActiva           = windows.NewLazySystemDLL("user32.dll")
	kernel32AppActiva         = windows.NewLazySystemDLL("kernel32.dll")
	getForegroundWindowApp    = user32AppActiva.NewProc("GetForegroundWindow")
	getWindowThreadProcessID  = user32AppActiva.NewProc("GetWindowThreadProcessId")
	getWindowTextLength       = user32AppActiva.NewProc("GetWindowTextLengthW")
	getWindowText             = user32AppActiva.NewProc("GetWindowTextW")
	queryFullProcessImageName = kernel32AppActiva.NewProc("QueryFullProcessImageNameW")
	getApplicationUserModelID = kernel32AppActiva.NewProc("GetApplicationUserModelId")
)

func tituloVentana(ventana uintptr) (string, error) {
	longitud, _, _ := getWindowTextLength.Call(ventana)
	buf := make([]uint16, int(longitud)+1)
	if longitud == 0 {
		return "", nil
	}
	escritos, _, err := getWindowText.Call(ventana, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if escritos == 0 {
		if err == windows.ERROR_SUCCESS {
			return "", nil // El título pudo quedar vacío entre las dos llamadas.
		}
		return "", fmt.Errorf("leer título de la ventana: %w", err)
	}
	return windows.UTF16ToString(buf[:escritos]), nil
}

func rutaEjecutable(proceso windows.Handle) (string, error) {
	buf := make([]uint16, 32768)
	tamano := uint32(len(buf))
	ok, _, err := queryFullProcessImageName.Call(
		uintptr(proceso), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&tamano)),
	)
	if ok == 0 {
		return "", fmt.Errorf("consultar ejecutable de la ventana: %w", err)
	}
	return windows.UTF16ToString(buf[:tamano]), nil
}

func aumidProceso(proceso windows.Handle) string {
	var tamano uint32
	r, _, _ := getApplicationUserModelID.Call(uintptr(proceso), uintptr(unsafe.Pointer(&tamano)), 0)
	if r != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || tamano == 0 {
		return ""
	}
	buf := make([]uint16, tamano)
	r, _, _ = getApplicationUserModelID.Call(
		uintptr(proceso), uintptr(unsafe.Pointer(&tamano)), uintptr(unsafe.Pointer(&buf[0])),
	)
	if r != 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func (appActiva) Active(ctx context.Context) (platform.App, error) {
	if err := ctx.Err(); err != nil {
		return platform.App{}, err
	}
	ventana, _, _ := getForegroundWindowApp.Call()
	if ventana == 0 {
		return platform.App{}, platform.ErrSinVentana
	}
	var pid uint32
	getWindowThreadProcessID.Call(ventana, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return platform.App{}, fmt.Errorf("consultar proceso de la ventana enfocada")
	}
	proceso, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return platform.App{}, fmt.Errorf("abrir proceso %d de la ventana enfocada: %w", pid, err)
	}
	defer windows.CloseHandle(proceso)
	ruta, err := rutaEjecutable(proceso)
	if err != nil {
		return platform.App{}, err
	}
	titulo, err := tituloVentana(ventana)
	if err != nil {
		return platform.App{}, err
	}
	ejecutable := filepath.Base(ruta)
	id := ejecutable
	if aumid := aumidProceso(proceso); aumid != "" {
		id = aumid
	}
	nombre := strings.TrimSuffix(ejecutable, filepath.Ext(ejecutable))
	app := platform.App{ID: id, Nombre: nombre, Titulo: titulo, PID: int(pid)}
	app.URL, app.Archivo = niveles(ventana, proceso, ejecutable, titulo)
	return app, nil
}
