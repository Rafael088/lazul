//go:build linux

package linux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os/exec"

	"github.com/Rafael088/lazul/platform"
)

// captura implementa platform.Screenshot con grim.
type captura struct{}

func (captura) Capture(ctx context.Context) (platform.Imagen, error) {
	if err := ctx.Err(); err != nil {
		return platform.Imagen{}, err
	}

	argumentos := []string{"-"}
	if salida := salidaEnfocada(ctx); salida != "" {
		argumentos = []string{"-o", salida, "-"}
	}
	contenido, err := exec.CommandContext(ctx, "grim", argumentos...).Output()
	if err != nil {
		if ctx.Err() != nil {
			return platform.Imagen{}, ctx.Err()
		}
		return platform.Imagen{}, fmt.Errorf("capturar la pantalla con grim: %w", err)
	}
	configuracion, err := png.DecodeConfig(bytes.NewReader(contenido))
	if err != nil {
		return platform.Imagen{}, fmt.Errorf("leer la captura de grim: %w", err)
	}
	return platform.Imagen{
		PNG: contenido, Ancho: configuracion.Width, Alto: configuracion.Height,
	}, nil
}

// salidaEnfocada intenta limitar la captura al monitor de la ventana activa.
// Si Hyprland no está disponible, grim conserva su comportamiento normal.
func salidaEnfocada(ctx context.Context) string {
	var activa struct {
		Monitor int `json:"monitor"`
	}
	datos, err := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j").Output()
	if err != nil || json.Unmarshal(datos, &activa) != nil {
		return ""
	}

	var monitores []struct {
		ID     int    `json:"id"`
		Nombre string `json:"name"`
		EnFoco bool   `json:"focused"`
	}
	datos, err = exec.CommandContext(ctx, "hyprctl", "monitors", "-j").Output()
	if err != nil || json.Unmarshal(datos, &monitores) != nil {
		return ""
	}
	for _, monitor := range monitores {
		if monitor.ID == activa.Monitor {
			return monitor.Nombre
		}
	}
	for _, monitor := range monitores {
		if monitor.EnFoco {
			return monitor.Nombre
		}
	}
	return ""
}
