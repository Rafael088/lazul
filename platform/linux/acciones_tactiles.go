//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Rafael088/lazul/platform"
)

type ejecutarTactil func(context.Context, string, ...string) ([]byte, error)

// accionesTactiles implementa platform.TactileActions con la API Lua de
// Hyprland (0.57+): hl.dsp.send_key_state sin window actúa sobre la ventana
// con foco. Se manda la bajada y la subida por separado, como hace Omarchy,
// porque send_shortcut a veces deja la tecla sintética pegada.
//
// Hyprland no puede inyectar la rueda del ratón (una «tecla» mouse_down se
// acepta sin error pero no hace nada), así que el scroll son flechas y el
// zoom es Ctrl + «+» / «-», que entienden navegadores, visores de PDF y
// LibreOffice.
type accionesTactiles struct {
	ejecutar ejecutarTactil
}

var _ platform.TactileActions = accionesTactiles{}

func ejecutarTactilReal(ctx context.Context, programa string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, programa, args...).CombinedOutput()
}

// ScrollPulsaciones es cuántas flechas manda un gesto de scroll: una sola
// mueve unos 40 px en un navegador, casi nada para un gesto de toda la mano.
const ScrollPulsaciones = 3

// codigoTeclas arma el Lua que pulsa la tecla veces veces: bajada, 30 ms,
// subida, 20 ms y la siguiente. Los temporizadores corren dentro de Hyprland,
// así que hyprctl vuelve enseguida.
func codigoTeclas(mods, tecla string, veces int) string {
	return fmt.Sprintf(`local m, k, n = %q, %q, %d
local function golpe(i)
  if i > n then return end
  hl.dispatch(hl.dsp.send_key_state({ mods = m, key = k, state = "down" }))
  hl.timer(function()
    hl.dispatch(hl.dsp.send_key_state({ mods = m, key = k, state = "up" }))
    hl.timer(function() golpe(i + 1) end, { timeout = 20, type = "oneshot" })
  end, { timeout = 30, type = "oneshot" })
end
golpe(1)`, mods, tecla, veces)
}

func (a accionesTactiles) enviar(ctx context.Context, mods, tecla, accion string) error {
	return a.enviarVeces(ctx, mods, tecla, 1, accion)
}

func (a accionesTactiles) enviarVeces(ctx context.Context, mods, tecla string, veces int, accion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ejecutar := a.ejecutar
	if ejecutar == nil {
		ejecutar = ejecutarTactilReal
	}
	salida, err := ejecutar(ctx, "hyprctl", "eval", codigoTeclas(mods, tecla, veces))
	if err == nil && strings.HasPrefix(strings.TrimSpace(string(salida)), "error") {
		err = errors.New(strings.TrimSpace(string(salida)))
	}
	if err != nil {
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
		return a.enviarVeces(ctx, "", "Up", a.perfil(ctx).scrollVeces, "scroll arriba")
	case platform.ScrollDown:
		return a.enviarVeces(ctx, "", "Down", a.perfil(ctx).scrollVeces, "scroll abajo")
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
		return a.enviar(ctx, "CTRL", "plus", "zoom para acercar")
	case platform.ZoomOut:
		return a.enviar(ctx, "CTRL", "minus", "zoom para alejar")
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
	if mode != platform.NavigationNormal && mode != platform.NavigationSlides {
		return platform.ErrAccionInvalida
	}
	var tecla, accion string
	if p := a.perfil(ctx); p.navAtras != "" {
		switch direction {
		case platform.NavigateLeft:
			tecla, accion = p.navAtras, "atrás ("+p.nombre+")"
		case platform.NavigateRight:
			tecla, accion = p.navAdelante, "adelante ("+p.nombre+")"
		}
		if tecla == "" {
			return platform.ErrAccionInvalida
		}
		return a.enviar(ctx, "", tecla, accion)
	}
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
