//go:build linux

package linux

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
		{"scroll arriba", func(a platform.TactileActions) error { return a.Scroll(context.Background(), platform.ScrollUp) }, codigoTeclas("", "Up", ScrollPulsaciones)},
		{"scroll abajo", func(a platform.TactileActions) error { return a.Scroll(context.Background(), platform.ScrollDown) }, codigoTeclas("", "Down", ScrollPulsaciones)},
		{"zoom acercar", func(a platform.TactileActions) error { return a.Zoom(context.Background(), platform.ZoomIn) }, codigoTeclas("CTRL", "plus", 1)},
		{"zoom alejar", func(a platform.TactileActions) error { return a.Zoom(context.Background(), platform.ZoomOut) }, codigoTeclas("CTRL", "minus", 1)},
		{"navegar izquierda", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateLeft, platform.NavigationNormal)
		}, codigoTeclas("", "Left", 1)},
		{"navegar derecha", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateRight, platform.NavigationNormal)
		}, codigoTeclas("", "Right", 1)},
		{"diapositiva anterior", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateLeft, platform.NavigationSlides)
		}, codigoTeclas("", "Page_Up", 1)},
		{"diapositiva siguiente", func(a platform.TactileActions) error {
			return a.Navigate(context.Background(), platform.NavigateRight, platform.NavigationSlides)
		}, codigoTeclas("", "Page_Down", 1)},
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

func TestCodigoTeclasUsaLaAPIDeHyprland057(t *testing.T) {
	c := codigoTeclas("CTRL", "plus", 2)
	for _, quiere := range []string{"hl.dispatch(hl.dsp.send_key_state(", `state = "down"`, `state = "up"`, `"CTRL", "plus", 2`} {
		if !strings.Contains(c, quiere) {
			t.Errorf("falta %q en:\n%s", quiere, c)
		}
	}
	if strings.Contains(c, "hl.send_shortcut") || strings.Contains(c, "mouse_") {
		t.Errorf("usa una API que no existe en Hyprland 0.57 o una rueda que no se puede inyectar:\n%s", c)
	}
}

func TestAccionesTactilesFallaSiHyprlandDevuelveError(t *testing.T) {
	adaptador := accionesTactiles{ejecutar: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("error: attempt to call a nil value"), nil
	}}
	if err := adaptador.Scroll(context.Background(), platform.ScrollUp); err == nil {
		t.Fatal("un «error:» de hyprctl con código 0 debe contar como falla")
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
