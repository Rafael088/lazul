//go:build windows

package windows

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type portapapeles struct{}

var (
	user32Portapapeles   = windows.NewLazySystemDLL("user32.dll")
	kernel32Portapapeles = windows.NewLazySystemDLL("kernel32.dll")
	openClipboard        = user32Portapapeles.NewProc("OpenClipboard")
	closeClipboard       = user32Portapapeles.NewProc("CloseClipboard")
	emptyClipboard       = user32Portapapeles.NewProc("EmptyClipboard")
	getClipboardData     = user32Portapapeles.NewProc("GetClipboardData")
	setClipboardData     = user32Portapapeles.NewProc("SetClipboardData")
	isClipboardFormat    = user32Portapapeles.NewProc("IsClipboardFormatAvailable")
	globalAlloc          = kernel32Portapapeles.NewProc("GlobalAlloc")
	globalFree           = kernel32Portapapeles.NewProc("GlobalFree")
	globalLock           = kernel32Portapapeles.NewProc("GlobalLock")
	globalUnlock         = kernel32Portapapeles.NewProc("GlobalUnlock")
	globalSize           = kernel32Portapapeles.NewProc("GlobalSize")
	rtlMoveMemory        = kernel32Portapapeles.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

func abrirPortapapeles(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ok, _, err := openClipboard.Call(0)
		if ok != 0 {
			return nil
		}
		t := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
			_ = err
		}
	}
}

func (portapapeles) ReadText(ctx context.Context) (string, error) {
	if err := abrirPortapapeles(ctx); err != nil {
		return "", err
	}
	defer closeClipboard.Call()
	disponible, _, _ := isClipboardFormat.Call(cfUnicodeText)
	if disponible == 0 {
		return "", nil
	}
	manejador, _, err := getClipboardData.Call(cfUnicodeText)
	if manejador == 0 {
		return "", fmt.Errorf("leer texto del portapapeles: %w", err)
	}
	puntero, _, err := globalLock.Call(manejador)
	if puntero == 0 {
		return "", fmt.Errorf("bloquear memoria del portapapeles: %w", err)
	}
	defer globalUnlock.Call(manejador)
	tamano, _, _ := globalSize.Call(manejador)
	if tamano < 2 {
		return "", nil
	}
	u16 := make([]uint16, int(tamano/2))
	rtlMoveMemory.Call(uintptr(unsafe.Pointer(&u16[0])), puntero, tamano)
	for i, v := range u16 {
		if v == 0 {
			u16 = u16[:i]
			break
		}
	}
	texto := windows.UTF16ToString(u16)
	return strings.ReplaceAll(texto, "\r\n", "\n"), nil
}

func (portapapeles) WriteText(ctx context.Context, texto string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	u16, err := windows.UTF16FromString(strings.ReplaceAll(texto, "\n", "\r\n"))
	if err != nil {
		return fmt.Errorf("convertir texto del portapapeles: %w", err)
	}
	bytes := uintptr(len(u16) * 2)
	manejador, _, err := globalAlloc.Call(gmemMoveable, bytes)
	if manejador == 0 {
		return fmt.Errorf("reservar memoria para el portapapeles: %w", err)
	}
	liberar := true
	defer func() {
		if liberar {
			globalFree.Call(manejador)
		}
	}()
	puntero, _, err := globalLock.Call(manejador)
	if puntero == 0 {
		return fmt.Errorf("bloquear memoria para el portapapeles: %w", err)
	}
	rtlMoveMemory.Call(puntero, uintptr(unsafe.Pointer(&u16[0])), bytes)
	globalUnlock.Call(manejador)

	if err := abrirPortapapeles(ctx); err != nil {
		return err
	}
	defer closeClipboard.Call()
	if ok, _, err := emptyClipboard.Call(); ok == 0 {
		return fmt.Errorf("vaciar portapapeles: %w", err)
	}
	if ok, _, err := setClipboardData.Call(cfUnicodeText, manejador); ok == 0 {
		return fmt.Errorf("escribir texto en el portapapeles: %w", err)
	}
	liberar = false // SetClipboardData transfirió la memoria al sistema.
	return nil
}
