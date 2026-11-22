//go:build windows

package windows

import (
	"testing"

	"github.com/Rafael088/lazul/platform"
)

func TestTeclasEspacio(t *testing.T) {
	es, err := teclasEspacio(platform.NavigateRight)
	if err != nil {
		t.Fatal(err)
	}
	// Alt abajo, Tab abajo y arriba, Alt arriba.
	if len(es) != 4 || es[0].vk != vkMenu || es[1].vk != vkTab || es[3].vk != vkMenu || es[3].flags&teclaSoltar == 0 {
		t.Fatalf("Alt+Tab: %+v", es)
	}
	es, err = teclasEspacio(platform.NavigateLeft)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 6 || es[0].vk != vkMenu || es[1].vk != vkShift || es[2].vk != vkTab {
		t.Fatalf("Alt+Mayús+Tab: %+v", es)
	}
	if _, err := teclasEspacio(0); err == nil {
		t.Error("dirección inválida aceptada")
	}
}
