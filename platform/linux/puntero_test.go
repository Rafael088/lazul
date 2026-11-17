//go:build linux

package linux

import (
	"errors"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

func TestPunteroCodificaPosicionAbsoluta(t *testing.T) {
	b, err := eventosPuntero(.25, .75)
	if err != nil {
		t.Fatal(err)
	}
	ev := leerEventos(b)
	if len(ev) != 3 || ev[0] != (eventoLeido{evAbs, absX, 16384}) ||
		ev[1] != (eventoLeido{evAbs, absY, 49151}) || ev[2].tipo != evSyn {
		t.Fatalf("eventos absolutos: %+v", ev)
	}
}

func TestPunteroRechazaCoordenadasFueraDePantalla(t *testing.T) {
	for _, p := range [][2]float64{{-.01, .5}, {.5, 1.01}} {
		if _, err := eventosPuntero(p[0], p[1]); !errors.Is(err, platform.ErrAccionInvalida) {
			t.Errorf("%v: error %v", p, err)
		}
	}
}
