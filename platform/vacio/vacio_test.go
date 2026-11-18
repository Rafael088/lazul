package vacio

import (
	"context"
	"errors"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

func TestTactilDevuelveNoImplementado(t *testing.T) {
	ts := New().TouchScreen
	if err := ts.Prepare(context.Background()); !errors.Is(err, platform.ErrNoImplementado) {
		t.Fatalf("Prepare: %v", err)
	}
	if err := ts.Frame(context.Background(), nil); !errors.Is(err, platform.ErrNoImplementado) {
		t.Fatalf("Frame: %v", err)
	}
}
