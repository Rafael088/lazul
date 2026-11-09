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

func TestPorNativoYNativo(t *testing.T) {
	c, err := Cargar()
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct{ so, nativo, id string }{
		{SistemaWindows, "firefox.exe", "firefox"},
		{SistemaWindows, `C:\Program Files\Microsoft VS Code\CODE.EXE`, "vscode"},
		{SistemaWindows, "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App", "calculator"},
		{SistemaLinux, "firefox", "firefox"},
		{SistemaLinux, "Code", "vscode"},
		{SistemaLinux, "google-chrome.desktop", "chrome"},
	}
	for _, k := range casos {
		a, ok := c.PorNativo(k.so, k.nativo)
		if !ok || a.ID != k.id {
			t.Errorf("PorNativo(%s, %q) = %q, %v; esperaba %q", k.so, k.nativo, a.ID, ok, k.id)
		}
	}
	if _, ok := c.PorNativo(SistemaLinux, "no-existe-esta-app"); ok {
		t.Error("encontró una app que no está")
	}
	ff, _ := c.Buscar("firefox")
	if ff.Nativo(SistemaWindows) != "firefox.exe" || ff.Nativo(SistemaLinux) != "firefox.desktop" || ff.Nativo("android") != "" {
		t.Errorf("Nativo de firefox: %q %q", ff.Nativo(SistemaWindows), ff.Nativo(SistemaLinux))
	}
	ol, _ := c.Buscar("outlook")
	if ol.Nativo(SistemaLinux) != "" {
		t.Error("outlook no tiene app de Linux en el catálogo")
	}
}
