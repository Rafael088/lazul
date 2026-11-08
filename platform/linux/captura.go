//go:build linux

package linux

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// captura implementa platform.Screenshot. Pendiente: grim (con la salida enfocada de hyprctl).
type captura struct{}

func (captura) Capture(context.Context) (platform.Imagen, error) {
	return platform.Imagen{}, platform.ErrNoImplementado
}
