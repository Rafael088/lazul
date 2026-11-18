//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// touchScreen deja explícita la capacidad aunque InjectTouchInput queda fuera
// de esta versión. Quien llama puede activar su respaldo con errors.Is.
type touchScreen struct{}

func (touchScreen) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return platform.ErrNoImplementado
}

func (touchScreen) Frame(ctx context.Context, _ []platform.TouchPoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return platform.ErrNoImplementado
}
