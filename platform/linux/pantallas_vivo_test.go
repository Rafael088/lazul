//go:build linux

package linux

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
)

// TestPantallasVivo pasa el táctil, el puntero y el foco por cada monitor de
// verdad y vuelve al que tenía el foco. Necesita el escritorio (Hyprland) y
// /dev/uinput: LAZUL_PANTALLAS_VIVO=1 go test ./platform/linux -run PantallasVivo -v
func TestPantallasVivo(t *testing.T) {
	if os.Getenv("LAZUL_PANTALLAS_VIVO") != "1" {
		t.Skip("mueve el foco del escritorio: LAZUL_PANTALLAS_VIVO=1")
	}
	sis := New()
	ctx := context.Background()
	ds, err := sis.Displays.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("monitores: %+v", ds)
	inicial, _ := platform.Enfocado(ds)
	if err := sis.TouchScreen.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sis.Displays.Target(ctx, inicial.Name); err != nil {
			t.Errorf("volver a %s: %v", inicial.Name, err)
		}
		if c, ok := sis.TouchScreen.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}()
	for _, d := range ds {
		inicio := time.Now()
		if err := sis.Displays.Target(ctx, d.Name); err != nil {
			t.Fatalf("Target(%s): %v", d.Name, err)
		}
		ahora, err := sis.Displays.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if f, _ := platform.Enfocado(ahora); f.Name != d.Name {
			t.Errorf("tras Target(%s) el foco está en %s", d.Name, f.Name)
		}
		if err := sis.AbsolutePointer.MoveAbsolute(ctx, .5, .5); err != nil {
			t.Errorf("puntero en %s: %v", d.Name, err)
		}
		t.Logf("%s: dirigido en %v", d.Name, time.Since(inicio).Round(time.Millisecond))
	}
}
