//go:build linux

package linux

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// portapapeles implementa platform.Clipboard con wl-paste y wl-copy.
type portapapeles struct{}

func (portapapeles) ReadText(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	contenido, err := exec.CommandContext(ctx, "wl-paste", "--no-newline").Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("leer el portapapeles con wl-paste: %w", err)
	}
	return string(contenido), nil
}

func (portapapeles) WriteText(ctx context.Context, texto string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	comando := exec.CommandContext(ctx, "wl-copy", "--type", "text/plain;charset=utf-8")
	comando.Stdin = bytes.NewBufferString(texto)
	if err := comando.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("escribir el portapapeles con wl-copy: %w", err)
	}
	return nil
}
