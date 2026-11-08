//go:build linux

package linux

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// portapapeles implementa platform.Clipboard. Pendiente: wl-paste y wl-copy.
type portapapeles struct{}

func (portapapeles) ReadText(context.Context) (string, error) { return "", platform.ErrNoImplementado }

func (portapapeles) WriteText(context.Context, string) error { return platform.ErrNoImplementado }
