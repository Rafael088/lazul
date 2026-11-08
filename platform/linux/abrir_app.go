//go:build linux

package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Rafael088/lazul/platform"
)

// abrirApp implementa platform.LaunchApp con archivos .desktop y xdg-open.
type abrirApp struct{}

func (abrirApp) Launch(ctx context.Context, ref platform.AppRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch {
	case ref.URL != "":
		return abrirConXDG(ctx, ref.URL, "URL")
	case ref.Archivo != "":
		if _, err := os.Stat(ref.Archivo); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("archivo %q: %w", ref.Archivo, platform.ErrNoEncontrada)
			}
			return fmt.Errorf("comprobar archivo %q: %w", ref.Archivo, err)
		}
		return abrirConXDG(ctx, ref.Archivo, "archivo")
	case ref.ID != "":
		ruta, ok := archivoDesktop(ref.ID)
		if !ok {
			return fmt.Errorf("app %q: %w", ref.ID, platform.ErrNoEncontrada)
		}
		comando := exec.CommandContext(ctx, "gio", "launch", ruta)
		if err := comando.Run(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("abrir app %q: %w", ref.ID, err)
		}
		return nil
	default:
		return platform.ErrRefInvalida
	}
}

func abrirConXDG(ctx context.Context, destino, tipo string) error {
	if err := exec.CommandContext(ctx, "xdg-open", destino).Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("abrir %s %q con xdg-open: %w", tipo, destino, err)
	}
	return nil
}

// archivoDesktop busca un ID según los directorios de datos de escritorio de
// freedesktop. El ID puede venir con o sin el sufijo .desktop.
func archivoDesktop(id string) (string, bool) {
	id = strings.TrimSuffix(strings.TrimSpace(id), ".desktop")
	if id == "" || filepath.Base(id) != id {
		return "", false
	}
	nombre := id + ".desktop"
	for _, directorio := range directoriosAplicaciones() {
		ruta := filepath.Join(directorio, nombre)
		if info, err := os.Stat(ruta); err == nil && !info.IsDir() {
			return ruta, true
		}
	}
	return "", false
}

func directoriosAplicaciones() []string {
	datosUsuario := os.Getenv("XDG_DATA_HOME")
	if datosUsuario == "" {
		if inicio, err := os.UserHomeDir(); err == nil {
			datosUsuario = filepath.Join(inicio, ".local", "share")
		}
	}
	datosSistema := os.Getenv("XDG_DATA_DIRS")
	if datosSistema == "" {
		datosSistema = "/usr/local/share:/usr/share"
	}
	directorios := []string{filepath.Join(datosUsuario, "applications")}
	for _, datos := range filepath.SplitList(datosSistema) {
		if datos != "" {
			directorios = append(directorios, filepath.Join(datos, "applications"))
		}
	}
	return directorios
}
