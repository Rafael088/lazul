//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Rafael088/lazul/platform"
)

// appActiva implementa platform.ActiveApp con hyprctl activewindow -j; la URL
// y el documento abierto salen de niveles.go.
type appActiva struct{}

func (appActiva) Active(ctx context.Context) (platform.App, error) {
	if err := ctx.Err(); err != nil {
		return platform.App{}, err
	}
	datos, err := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j").Output()
	if err != nil {
		if ctx.Err() != nil {
			return platform.App{}, ctx.Err()
		}
		return platform.App{}, fmt.Errorf("consultar la ventana activa con hyprctl: %w", err)
	}
	var ventana struct {
		Clase        string `json:"class"`
		ClaseInicial string `json:"initialClass"`
		Titulo       string `json:"title"`
		PID          int    `json:"pid"`
	}
	if err := json.Unmarshal(datos, &ventana); err != nil {
		return platform.App{}, fmt.Errorf("leer la ventana activa de hyprctl: %w", err)
	}
	id := strings.TrimSpace(ventana.Clase)
	if id == "" {
		id = strings.TrimSpace(ventana.ClaseInicial)
	}
	if id == "" {
		return platform.App{}, platform.ErrSinVentana
	}
	app := platform.App{ID: id, Nombre: id, Titulo: ventana.Titulo, PID: ventana.PID}
	app.URL, app.Archivo = niveles(ventana.PID, ventana.Titulo)
	return app, nil
}
