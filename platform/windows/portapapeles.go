//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// portapapeles implementa platform.Clipboard. Pendiente: API de portapapeles (CF_UNICODETEXT, convirtiendo \r\n a \n).
type portapapeles struct{}

func (portapapeles) ReadText(context.Context) (string, error) { return "", platform.ErrNoImplementado }

func (portapapeles) WriteText(context.Context, string) error { return platform.ErrNoImplementado }
