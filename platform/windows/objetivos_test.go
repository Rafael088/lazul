//go:build windows

package windows

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
)

func TestNormalizarObjetivosALaZonaDelTactil(t *testing.T) {
	// Segundo monitor de 1920×1080 físicos a la derecha, al 150 %.
	z := zonaTactil{x: 1920, y: 0, ancho: 1920, alto: 1080}
	s := normalizarObjetivos([]rectFisico{
		{x: 1920 + 959.5, y: 539.5, ancho: 36, alto: 36, rol: platform.TargetRoleButton}, // en el centro
		{x: 100, y: 100, ancho: 30, alto: 30},                                            // en el otro monitor
		{x: 2000, y: 10, ancho: 0, alto: 20},                                             // sin tamaño
	}, z, 144)
	if math.Abs(s.LogicalWidth-1280) > .01 || math.Abs(s.LogicalHeight-720) > .01 {
		t.Fatalf("tamaño lógico %vx%v, quería 1280x720", s.LogicalWidth, s.LogicalHeight)
	}
	if len(s.Targets) != 1 {
		t.Fatalf("objetivos %+v, quería solo el del centro", s.Targets)
	}
	o := s.Targets[0]
	// Misma escala que zonaTactil.pixel: x*(ancho-1).
	if p := z.pixel(o.X, o.Y); p.x != 1920+960 || p.y != 540 {
		t.Fatalf("el objetivo vuelve al píxel %+v", p)
	}
	if math.Abs(o.Width*float64(z.ancho-1)-36) > .01 {
		t.Fatalf("ancho %v", o.Width)
	}
	if o.Role != platform.TargetRoleButton {
		t.Fatalf("rol %q", o.Role)
	}
	b := normalizarLimitesVentana(rect{izq: 2000, arriba: 20, der: 3000, abajo: 1020}, z)
	if math.Abs(b.X-80.0/1919) > .01 || math.Abs(b.Width-1000.0/1919) > .01 {
		t.Fatalf("límites de ventana %+v", b)
	}
}

func TestRolesUIASeVuelvenGenericos(t *testing.T) {
	casos := map[int32]platform.TargetRole{
		50000: platform.TargetRoleButton,
		50005: platform.TargetRoleLink,
		50019: platform.TargetRoleTab,
		50014: platform.TargetRoleScrollBar,
		50038: platform.TargetRoleSeparator,
	}
	for tipo, quiero := range casos {
		if got := rolControlUIA(tipo); got != quiero {
			t.Errorf("tipo %d: %q, quiero %q", tipo, got, quiero)
		}
	}
}

// TestObjetivosVivo lee los objetivos de la ventana con el foco del
// escritorio (LAZUL_IMAN_VIVO=1, con un navegador delante) y dice cuánto
// tarda: es la medida del CPU que el imán le pide a la app de la ventana.
func TestObjetivosVivo(t *testing.T) {
	if os.Getenv("LAZUL_IMAN_VIVO") == "" {
		t.Skip("LAZUL_IMAN_VIVO=1 para leer la ventana con el foco")
	}
	b := objetivosWindows{destino: &destinoPantalla{}}
	var total time.Duration
	for i := range 10 {
		inicio := time.Now()
		s, err := b.Targets(context.Background())
		d := time.Since(inicio)
		if err != nil {
			t.Fatal(err)
		}
		total += d
		if i == 0 {
			t.Logf("%d objetivos, zona %.0fx%.0f lógicos, %v", len(s.Targets), s.LogicalWidth, s.LogicalHeight, d)
		}
	}
	t.Logf("media de 10 lecturas: %v", total/10)
}
