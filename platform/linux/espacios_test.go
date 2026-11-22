//go:build linux

package linux

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

func TestEspaciosHyprland(t *testing.T) {
	casos := []struct {
		dir    platform.NavigationDirection
		quiere string
	}{
		{platform.NavigateRight, `hl.dispatch(hl.dsp.focus({ workspace = "e+1" }))`},
		{platform.NavigateLeft, `hl.dispatch(hl.dsp.focus({ workspace = "e-1" }))`},
	}
	for _, c := range casos {
		var visto []string
		e := espaciosHyprland{ejecutar: func(_ context.Context, programa string, args ...string) ([]byte, error) {
			visto = append([]string{programa}, args...)
			return []byte("ok"), nil
		}}
		if err := e.Switch(context.Background(), c.dir); err != nil {
			t.Fatal(err)
		}
		if strings.Join(visto, " ") != "hyprctl eval "+c.quiere {
			t.Errorf("ejecutó %q", visto)
		}
	}
}

func TestEspaciosHyprlandErrores(t *testing.T) {
	e := espaciosHyprland{ejecutar: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("error: bad lua"), nil
	}}
	if err := e.Switch(context.Background(), platform.NavigateRight); err == nil {
		t.Error("un error de Lua no se devolvió")
	}
	if err := e.Switch(context.Background(), 0); !errors.Is(err, platform.ErrAccionInvalida) {
		t.Errorf("dirección inválida: %v", err)
	}
}
