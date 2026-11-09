//go:build linux

package linux

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
)

// xdgFalso pone un xdg-open de mentira primero en el PATH.
func xdgFalso(t *testing.T, cuerpo string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte("#!/bin/sh\n"+cuerpo+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestLaunchNoEsperaALaVentana(t *testing.T) {
	xdgFalso(t, "sleep 2")
	t0 := time.Now()
	err := abrirApp{}.Launch(context.Background(), platform.AppRef{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if d := time.Since(t0); d > 200*time.Millisecond {
		t.Fatalf("Launch tardó %v; no debe esperar a que abra la ventana", d)
	}
}

func TestLaunchDevuelveElFalloInmediato(t *testing.T) {
	xdgFalso(t, "exit 3")
	err := abrirApp{}.Launch(context.Background(), platform.AppRef{URL: "https://example.com"})
	if err == nil {
		t.Fatal("un xdg-open que falla de inmediato debe devolver error")
	}
}

func TestLaunchComandoInexistente(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := abrirApp{}.Launch(context.Background(), platform.AppRef{URL: "https://example.com"})
	if err == nil {
		t.Fatal("sin xdg-open debe devolver error")
	}
}
