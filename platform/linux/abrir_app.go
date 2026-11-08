//go:build linux

package linux

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// abrirApp implementa platform.LaunchApp. Pendiente: archivos .desktop y xdg-open.
type abrirApp struct{}

func (abrirApp) Launch(context.Context, platform.AppRef) error { return platform.ErrNoImplementado }
