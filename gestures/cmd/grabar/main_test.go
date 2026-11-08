package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Rafael088/lazul/gestures"
)

func probar(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	vendor := fstest.MapFS{"vision_bundle.mjs": {Data: []byte("// mediapipe")}}
	srv := httptest.NewServer(nuevoServidor(dir, vendor).rutas())
	t.Cleanup(srv.Close)
	return srv, dir
}

func pedir(t *testing.T, metodo, url, cuerpo string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(metodo, url, strings.NewReader(cuerpo))
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	return r.StatusCode, v
}

func leer(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Una toma sintética de la L entra, se guarda con número correlativo, se
// lee igual que una grabación y el clasificador la reconoce.
func TestGuardarToma(t *testing.T) {
	srv, dir := probar(t)
	toma := leer(t, "../../testdata/sinteticas/activar-buena-001.jsonl")
	for i, quiere := range []string{"activar-buena-001.jsonl", "activar-buena-002.jsonl"} {
		cod, res := pedir(t, "POST", srv.URL+"/guardar", toma)
		if cod != 200 || res["archivo"] != quiere || res["acierto"] != true {
			t.Fatalf("toma %d: %d %v", i, cod, res)
		}
	}
	gs, err := gestures.LeerDirectorio(dir)
	if err != nil || len(gs) != 2 || gs[0].Cabecera.Ancho != 640 {
		t.Fatalf("%v %+v", err, gs)
	}
	_, cuenta := pedir(t, "GET", srv.URL+"/cuenta", "")
	if cuenta["activar-buena"] != 2.0 {
		t.Fatalf("cuenta: %v", cuenta)
	}
}

func TestNombreDeLosNegativos(t *testing.T) {
	srv, _ := probar(t)
	toma := leer(t, "../../testdata/sinteticas/senalar-mala-002.jsonl")
	if _, res := pedir(t, "POST", srv.URL+"/guardar", toma); res["archivo"] != "senalar-mala-001.jsonl" {
		t.Fatalf("%v", res)
	}
	malo := `{"gesto":"NINGUNO","luz":"buena","negativo":"../../x"}` + "\n" + `{"t":0,"manos":[]}`
	if _, res := pedir(t, "POST", srv.URL+"/guardar", malo); res["archivo"] != "x-buena-001.jsonl" {
		t.Fatalf("no limpió el nombre: %v", res)
	}
}

func TestTomasInvalidas(t *testing.T) {
	srv, dir := probar(t)
	for _, cuerpo := range []string{
		"",
		`{"gesto":"SALUDAR","luz":"buena"}` + "\n" + `{"t":0,"manos":[]}`,
		`{"gesto":"ACTIVAR","luz":"buena"}`,
		`{"gesto":"ACTIVAR","luz":"regular"}` + "\n" + `{"t":0,"manos":[]}`,
		`{"gesto":"ACTIVAR","luz":"buena"}` + "\n" + `{"t":0,"manos":[{"p":[[0,0,0]]}]}`,
	} {
		if cod, _ := pedir(t, "POST", srv.URL+"/guardar", cuerpo); cod != 400 {
			t.Errorf("aceptó %q (%d)", cuerpo, cod)
		}
	}
	if rutas, _ := filepath.Glob(filepath.Join(dir, "*")); len(rutas) != 0 {
		t.Errorf("quedaron archivos: %v", rutas)
	}
}

func TestDescartarSoloLasPropias(t *testing.T) {
	srv, dir := probar(t)
	ajena := filepath.Join(dir, "activar-buena-001.jsonl")
	if err := os.WriteFile(ajena, []byte(`{"gesto":"ACTIVAR","luz":"buena"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cod, _ := pedir(t, "POST", srv.URL+"/descartar?archivo=activar-buena-001.jsonl", ""); cod != 403 {
		t.Fatalf("descartó una toma de otra sesión: %d", cod)
	}
	_, res := pedir(t, "POST", srv.URL+"/guardar", leer(t, "../../testdata/sinteticas/activar-buena-001.jsonl"))
	if res["archivo"] != "activar-buena-002.jsonl" {
		t.Fatalf("%v", res)
	}
	if cod, _ := pedir(t, "POST", srv.URL+"/descartar?archivo=activar-buena-002.jsonl", ""); cod != 200 {
		t.Fatalf("no descartó la propia: %d", cod)
	}
	if _, err := os.Stat(ajena); err != nil {
		t.Fatal("borró la ajena")
	}
}

// El clasificador en vivo: arranca en el modo del gesto y avanza cuadro a
// cuadro hasta reconocer la L.
func TestCuadroEnVivo(t *testing.T) {
	srv, _ := probar(t)
	if _, res := pedir(t, "POST", srv.URL+"/reiniciar?gesto=SOLTAR", ""); res["modo"] != "agarrando" {
		t.Fatalf("%v", res)
	}
	pedir(t, "POST", srv.URL+"/reiniciar?gesto=ACTIVAR", "")
	lineas := strings.Split(strings.TrimSpace(leer(t, "../../testdata/sinteticas/activar-buena-001.jsonl")), "\n")[1:]
	vio := map[string]bool{}
	for _, l := range lineas {
		cuerpo := strings.TrimSuffix(l, "}") + `,"ancho":640,"alto":480}`
		cod, res := pedir(t, "POST", srv.URL+"/cuadro", cuerpo)
		if cod != 200 {
			t.Fatalf("%d", cod)
		}
		vio[res["pose"].(string)] = true
		if g, ok := res["gesto"]; ok {
			vio["gesto "+g.(string)] = true
		}
	}
	if !vio["L"] || !vio["gesto ACTIVAR"] {
		t.Fatalf("vio %v", vio)
	}
}

func TestSirveLaPagina(t *testing.T) {
	srv, _ := probar(t)
	for _, ruta := range []string{"/", "/app.js", "/vendor/vision_bundle.mjs"} {
		r, err := http.Get(srv.URL + ruta)
		if err != nil || r.StatusCode != 200 {
			t.Errorf("%s: %v %v", ruta, err, r.StatusCode)
		}
		r.Body.Close()
	}
}
