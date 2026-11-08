//go:build linux

package linux

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// appActiva implementa platform.ActiveApp. Pendiente: hyprctl activewindow -j.
type appActiva struct{}

func (appActiva) Active(context.Context) (platform.App, error) {
	return platform.App{}, platform.ErrNoImplementado
}
