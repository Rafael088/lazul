package apps

import (
	"os"
	"strings"
	"testing"
)

func TestCatalogoIncluido(t *testing.T) {
	catalogo, err := Cargar()
	if err != nil {
		t.Fatalf("el catalogo incluido no es valido: %v", err)
	}
	if len(catalogo.Apps) < 30 {
		t.Fatalf("el catalogo tiene %d apps; se esperaban al menos 30", len(catalogo.Apps))
	}
}

func TestCatalogoMalFormadoFalla(t *testing.T) {
	contenido, err := os.ReadFile("testdata/apps_malo.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Validar(contenido)
	if err == nil {
		t.Fatal("Validar acepto una app sin respaldo web")
	}
	if !strings.Contains(err.Error(), "web") {
		t.Fatalf("el error no identifica el campo web: %v", err)
	}
}
