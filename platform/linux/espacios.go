//go:build linux

package linux

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rafael088/lazul/platform"
)

// espaciosHyprland implementa platform.Spaces: con un solo monitor, los tres
// dedos cambian al escritorio (workspace) abierto de al lado, como
// hl.dsp.focus({ workspace = "e+1" }) en los atajos de Omarchy. La derecha
// de la persona es el siguiente.
type espaciosHyprland struct {
	ejecutar func(ctx context.Context, programa string, args ...string) ([]byte, error)
}

var _ platform.Spaces = espaciosHyprland{}

// codigoEspacio es el Lua que cambia de escritorio hacia direction.
func codigoEspacio(direction platform.NavigationDirection) (string, error) {
	var destino string
	switch direction {
	case platform.NavigateRight:
		destino = "e+1"
	case platform.NavigateLeft:
		destino = "e-1"
	default:
		return "", platform.ErrAccionInvalida
	}
	return fmt.Sprintf(`hl.dispatch(hl.dsp.focus({ workspace = %q }))`, destino), nil
}

func (e espaciosHyprland) Switch(ctx context.Context, direction platform.NavigationDirection) error {
	codigo, err := codigoEspacio(direction)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ejecutar := e.ejecutar
	if ejecutar == nil {
		ejecutar = ejecutarTactilReal
	}
	salida, err := ejecutar(ctx, "hyprctl", "eval", codigo)
	if err != nil || respuestaEvalFallida(salida) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("cambiar de escritorio con hyprctl eval: %v %s", err, strings.TrimSpace(string(salida)))
	}
	return nil
}
