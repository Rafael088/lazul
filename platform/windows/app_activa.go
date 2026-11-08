//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// appActiva implementa platform.ActiveApp. Pendiente: GetForegroundWindow: ejecutable y AUMID.
type appActiva struct{}

func (appActiva) Active(context.Context) (platform.App, error) {
	return platform.App{}, platform.ErrNoImplementado
}
