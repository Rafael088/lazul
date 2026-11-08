//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// abrirApp implementa platform.LaunchApp. Pendiente: ShellExecute y AUMID para apps de la tienda.
type abrirApp struct{}

func (abrirApp) Launch(context.Context, platform.AppRef) error { return platform.ErrNoImplementado }
