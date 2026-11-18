package falso

import (
	"context"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

func TestRegistraCopiasDeLosCuadrosTactiles(t *testing.T) {
	f := New("táctil")
	puntos := []platform.TouchPoint{{Slot: 0, X: .25, Y: .75, Down: true}}
	if err := f.Frame(context.Background(), puntos); err != nil {
		t.Fatal(err)
	}
	puntos[0].X = 1

	cuadros := f.CuadrosTactiles()
	if len(cuadros) != 1 || len(cuadros[0]) != 1 || cuadros[0][0].X != .25 {
		t.Fatalf("cuadros: %+v", cuadros)
	}
	cuadros[0][0].X = 1
	if f.CuadrosTactiles()[0][0].X != .25 {
		t.Fatal("CuadrosTactiles devolvió el almacenamiento interno")
	}
}
