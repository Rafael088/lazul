//go:build linux

package linux

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

type llamadaTactil struct {
	programa string
	args     []string
}

type ejecutorTactilFalso struct {
	llamadas []llamadaTactil
	err      error
}

func (e *ejecutorTactilFalso) ejecutar(_ context.Context, programa string, args ...string) ([]byte, error) {
	e.llamadas = append(e.llamadas, llamadaTactil{programa: programa, args: append([]string(nil), args...)})
	return nil, e.err
}

func TestAccionesTactilesEnviaAtajosALaVentanaConFoco(t *testing.T) {
	casos := []struct {
		nombre string
		llamar func(platform.TactileActions) error
		codigo string
	}{
		{"scroll arriba", func(a platform.TactileActions) error { return a.Scroll(context.Background(), platform.ScrollUp) }, `hl.send_shortcut({ mods = "", key = "mouse_up" })`},
		{"scroll abajo", func(a platform.TactileActions) error { return a.Scroll(context.Background(), platform.ScrollDown) }, `hl.send_shortcut({ mods = "", key = "mouse_down" })`},
		{"zoom acercar", func(a platform.TactileActions) error { return a.Zoom(context.Background(), platform.ZoomIn) }, `hl.send_shortcut({ mods = "CTRL", key = "mouse_up" })`},
		{"zoom alejar", func(a platform.TactileActions) error { return a.Zoom(context.Background(), platform.ZoomOut) }, `hl.send_shortcut({ mods = "CTRL", key = "mouse_down" })`},
		{"navegar izquierda", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateLeft, platform.NavigationNormal)
		}, `hl.send_shortcut({ mods = "", key = "Left" })`},
		{"navegar derecha", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateRight, platform.NavigationNormal)
		}, `hl.send_shortcut({ mods = "", key = "Right" })`},
		{"diapositiva anterior", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateLeft, platform.NavigationSlides)
		}, `hl.send_shortcut({ mods = "", key = "Page_Up" })`},
		{"diapositiva siguiente", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateRight, platform.NavigationSlides)
		}, `hl.send_shortcut({ mods = "", key = "Page_Down" })`},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			ejecutor := &ejecutorTactilFalso{}
			adaptador := accionesTactiles{ejecutar: ejecutor.ejecutar}
			if err := caso.llamar(adaptador); err != nil {
				t.Fatal(err)
			}
			quiere := []llamadaTactil{{programa: "hyprctl", args: []string{"eval", caso.codigo}}}
			if !reflect.DeepEqual(ejecutor.llamadas, quiere) {
				t.Fatalf("llamadas %+v, quiere %+v", ejecutor.llamadas, quiere)
			}
		})
	}
}

func TestAccionesTactilesNoEjecutaConContextoCancelado(t *testing.T) {
	ejecutor := &ejecutorTactilFalso{}
	adaptador := accionesTactiles{ejecutar: ejecutor.ejecutar}
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	err := adaptador.Scroll(ctx, platform.ScrollUp)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, quiere context.Canceled", err)
	}
	if len(ejecutor.llamadas) != 0 {
		t.Fatalf("ejecutó con contexto cancelado: %+v", ejecutor.llamadas)
	}
}

func TestAccionesTactilesPropagaFalloDeHyprland(t *testing.T) {
	fallo := errors.New("sin socket")
	ejecutor := &ejecutorTactilFalso{err: fallo}
	err := (accionesTactiles{ejecutar: ejecutor.ejecutar}).Zoom(context.Background(), platform.ZoomIn)
	if !errors.Is(err, fallo) {
		t.Fatalf("error %v no envuelve %v", err, fallo)
	}
}
