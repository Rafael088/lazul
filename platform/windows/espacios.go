//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// espaciosWindows implementa platform.Spaces como el gesto de tres dedos de
// Windows hacia un lado: cambia de app. A la derecha, Alt+Tab (la app
// anterior); a la izquierda, Alt+Mayús+Tab.
type espaciosWindows struct{}

var _ platform.Spaces = espaciosWindows{}

const (
	vkTab   = 0x09
	vkShift = 0x10
	vkMenu  = 0x12 // Alt
)

// teclasEspacio son las pulsaciones de cambiar de app hacia direction.
func teclasEspacio(direction platform.NavigationDirection) ([]entradaTeclado, error) {
	switch direction {
	case platform.NavigateRight:
		return pulsaciones([]uint16{vkMenu}, vkTab, 1), nil
	case platform.NavigateLeft:
		return pulsaciones([]uint16{vkMenu, vkShift}, vkTab, 1), nil
	}
	return nil, platform.ErrAccionInvalida
}

func (espaciosWindows) Switch(ctx context.Context, direction platform.NavigationDirection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	es, err := teclasEspacio(direction)
	if err != nil {
		return err
	}
	return enviarTeclado(es)
}
