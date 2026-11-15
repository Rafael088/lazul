//go:build linux

package linux

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/Rafael088/lazul/platform"
)

type ejecutarTactil func(context.Context, string, ...string) ([]byte, error)

// accionesTactiles implementa platform.TactileActions con la API Lua de
// Hyprland. Al omitir window, hl.send_shortcut usa la ventana con foco.
type accionesTactiles struct {
	ejecutar ejecutarTactil
}

var _ platform.TactileActions = accionesTactiles{}

func ejecutarTactilReal(ctx context.Context, programa string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, programa, args...).CombinedOutput()
}

func (a accionesTactiles) enviar(ctx context.Context, mods, tecla, accion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ejecutar := a.ejecutar
	if ejecutar == nil {
		ejecutar = ejecutarTactilReal
	}
	codigo := fmt.Sprintf(`hl.send_shortcut({ mods = %q, key = %q })`, mods, tecla)
	if _, err := ejecutar(ctx, "hyprctl", "eval", codigo); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("enviar %s a la ventana con foco mediante Hyprland: %w", accion, err)
	}
	return nil
}

func (a accionesTactiles) Scroll(ctx context.Context, direction platform.ScrollDirection) error {
	switch direction {
	case platform.ScrollUp:
		return a.enviar(ctx, "", "mouse_up", "scroll arriba")
	case platform.ScrollDown:
		return a.enviar(ctx, "", "mouse_down", "scroll abajo")
	default:
		if err := ctx.Err(); err != nil {
			return err
		}
		return platform.ErrAccionInvalida
	}
}

func (a accionesTactiles) Zoom(ctx context.Context, direction platform.ZoomDirection) error {
	switch direction {
	case platform.ZoomIn:
		return a.enviar(ctx, "CTRL", "mouse_up", "zoom para acercar")
	case platform.ZoomOut:
		return a.enviar(ctx, "CTRL", "mouse_down", "zoom para alejar")
	default:
		if err := ctx.Err(); err != nil {
			return err
		}
		return platform.ErrAccionInvalida
	}
}

func (a accionesTactiles) Navigate(ctx context.Context, direction platform.NavigationDirection, mode platform.NavigationMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var tecla, accion string
	switch mode {
	case platform.NavigationNormal:
		switch direction {
		case platform.NavigateLeft:
			tecla, accion = "Left", "flecha izquierda"
		case platform.NavigateRight:
			tecla, accion = "Right", "flecha derecha"
		}
	case platform.NavigationSlides:
		switch direction {
		case platform.NavigateLeft:
			tecla, accion = "Page_Up", "diapositiva anterior"
		case platform.NavigateRight:
			tecla, accion = "Page_Down", "diapositiva siguiente"
		}
	}
	if tecla == "" {
		return platform.ErrAccionInvalida
	}
	return a.enviar(ctx, "", tecla, accion)
}
