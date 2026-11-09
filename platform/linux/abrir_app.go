//go:build linux

package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rafael088/lazul/platform"
)

// abrirApp implementa platform.LaunchApp con archivos .desktop y xdg-open.
// No espera a que se cierre lo abierto (ver esperaLanzar).
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
		if err := lanzar(ctx, "gio", "launch", ruta); err != nil {
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

// esperaLanzar: lo que se espera a que xdg-open o gio terminen. xdg-open
// tarda 260-545 ms en volver y algunos manejadores (imv para las imágenes)
// no vuelven hasta cerrar la ventana; Launch no espera a eso. Un comando que
// no existe falla en Start, y uno que rechaza el destino sale con error en
// pocos ms: si en este tiempo falla se devuelve su error; si sigue corriendo
// se da por abierto y Wait lo recoge en segundo plano.
var esperaLanzar = 50 * time.Millisecond

// lanzar corre el comando sin atarlo a ctx (cancelar no debe cerrar lo que
// se abrió) y espera como mucho esperaLanzar.
func lanzar(ctx context.Context, nombre string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.Command(nombre, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	fin := make(chan error, 1)
	go func() { fin <- cmd.Wait() }()
	select {
	case err := <-fin:
		return err
	case <-time.After(esperaLanzar):
		return nil
	case <-ctx.Done():
		return nil // ya se lanzó; Wait lo recoge cuando termine
	}
}

func abrirConXDG(ctx context.Context, destino, tipo string) error {
	if err := lanzar(ctx, "xdg-open", destino); err != nil {
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
