//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rafael088/lazul/platform"
)

// bordeHyprland implementa platform.BorderResize con hyprctl. Omarchy enciende
// general:resize_on_border y deja 8 px de agarre: un toque táctil cerca del
// borde de una ventana la redimensiona en vez de hacer clic. Mientras el panel
// está encendido Lazul lo apaga con hl.config (el mismo hl.* que hl.dsp) y
// devuelve el valor que había al apagarlo.
type bordeHyprland struct {
	ejecutar func(ctx context.Context, programa string, args ...string) ([]byte, error)
}

var _ platform.BorderResize = bordeHyprland{}

// leerResizeOnBorder entiende las dos formas de hyprctl getoption: la de
// texto («int: 1») y la de -j ({"int":1} o {"bool":true}). Un valor distinto
// de cero está encendido.
func leerResizeOnBorder(datos []byte) (bool, error) {
	s := strings.TrimSpace(string(datos))
	if s == "" {
		return false, errors.New("hyprctl getoption no dijo nada")
	}
	if strings.HasPrefix(s, "{") {
		var r struct {
			Int  *int  `json:"int"`
			Bool *bool `json:"bool"`
		}
		if err := json.Unmarshal([]byte(s), &r); err == nil {
			if r.Bool != nil {
				return *r.Bool, nil
			}
			if r.Int != nil {
				return *r.Int != 0, nil
			}
		}
	}
	for _, linea := range strings.Split(s, "\n") {
		clave, valor, ok := strings.Cut(linea, ":")
		if !ok {
			continue
		}
		valor = strings.TrimSpace(valor)
		switch strings.ToLower(strings.TrimSpace(clave)) {
		case "int":
			n, err := strconv.Atoi(valor)
			if err != nil {
				return false, fmt.Errorf("general:resize_on_border no es un número: %q", valor)
			}
			return n != 0, nil
		case "bool":
			b, err := strconv.ParseBool(valor)
			if err != nil {
				return false, fmt.Errorf("general:resize_on_border no es un booleano: %q", valor)
			}
			return b, nil
		}
	}
	// Otra forma que se ha visto: «general:resize_on_border = 1».
	if _, valor, ok := strings.Cut(s, "="); ok {
		valor = strings.TrimSpace(valor)
		if n, err := strconv.Atoi(valor); err == nil {
			return n != 0, nil
		}
		if b, err := strconv.ParseBool(valor); err == nil {
			return b, nil
		}
	}
	return false, fmt.Errorf("no se entiende la salida de hyprctl getoption: %q", s)
}

// codigoResizeOnBorder es el Lua que cambia la opción dentro de Hyprland.
func codigoResizeOnBorder(on bool) string {
	return fmt.Sprintf("hl.config({ general = { resize_on_border = %t } })", on)
}

func (b bordeHyprland) ResizeOnBorder(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	salida, err := b.ejecutar(ctx, "hyprctl", "getoption", "general:resize_on_border")
	if err != nil {
		return false, fmt.Errorf("leer general:resize_on_border con hyprctl: %w", err)
	}
	return leerResizeOnBorder(salida)
}

func (b bordeHyprland) SetResizeOnBorder(ctx context.Context, on bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	salida, err := b.ejecutar(ctx, "hyprctl", "eval", codigoResizeOnBorder(on))
	if err != nil || respuestaEvalFallida(salida) {
		return fmt.Errorf("cambiar general:resize_on_border con hyprctl eval: %v %s", err, strings.TrimSpace(string(salida)))
	}
	return nil
}
