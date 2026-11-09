package sim

import (
	"context"
	"io"
	"testing"
)

func TestSimCompleto(t *testing.T) {
	if err := Correr(context.Background(), Opciones{Receptores: 3, Pings: 5, Grabaciones: "../../fixtures/gestos", Intentos: 8}, io.Discard); err != nil {
		t.Fatal(err)
	}
}
