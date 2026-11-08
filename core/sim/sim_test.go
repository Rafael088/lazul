package sim

import (
	"context"
	"io"
	"testing"
)

func TestSimCompleto(t *testing.T) {
	if err := Correr(context.Background(), 3, 5, io.Discard); err != nil {
		t.Fatal(err)
	}
}
