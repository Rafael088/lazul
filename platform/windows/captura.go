//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// captura implementa platform.Screenshot. Pendiente: GDI o DXGI sobre el monitor de la ventana enfocada.
type captura struct{}

func (captura) Capture(context.Context) (platform.Imagen, error) {
	return platform.Imagen{}, platform.ErrNoImplementado
}
