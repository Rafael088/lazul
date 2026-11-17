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
	srv := httptest.NewServer(nuevoServidor(dir, filepath.Join(dir, "pellizco"), vendor).rutas())
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
	gestos, _ := cuenta["gestos"].(map[string]any)
	if gestos["activar-buena"] != 2.0 {
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

// La página conserva las tomas viejas y ofrece las del modelo nuevo.
func TestPaginaTieneLasTomasViejasYNuevas(t *testing.T) {
	srv, _ := probar(t)
	r, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	pagina := string(b)
	for _, valor := range []string{
		"ACTIVAR", "AGARRAR", "SOLTAR", "MANDAR_APP", "NINGUNO:pinza", "NINGUNO:senalar", "NINGUNO:normal",
		"toma:arrastrar-arriba", "toma:arrastrar-abajo", "toma:arrastrar-izquierda", "toma:arrastrar-derecha",
		"toma:entrar-panel", "toma:salir-panel", "toma:zoom-dos-manos", "toma:normal",
	} {
		if !strings.Contains(pagina, `value="`+valor+`"`) {
			t.Errorf("la página no ofrece %q", valor)
		}
	}
}

func debeJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// manoConApertura arma 21 puntos donde la palma mide 0,2 y las puntas del
// pulgar e índice están separadas por apertura*0,2: la apertura pedida.
func manoConApertura(a float64) map[string]any {
	p := make([][3]float64, gestures.NumPuntos)
	p[gestures.Muneca] = [3]float64{0.5, 0.5, 0}
	p[gestures.MedioMCP] = [3]float64{0.5, 0.7, 0}
	p[gestures.PulgarPunta] = [3]float64{0.5, 0.5, 0}
	p[gestures.IndicePunta] = [3]float64{0.5, 0.5 + a*0.2, 0}
	return map[string]any{"derecha": true, "p": p}
}

func tomaDePellizco(t *testing.T, aperturas []float64) string {
	cab := map[string]any{
		"gesto": "NINGUNO", "luz": "buena", "ancho": 640, "alto": 480,
		"toma": "arrastrar-arriba", "panel": map[string]float64{"x0": 0.1, "y0": 0.1, "x1": 0.9, "y1": 0.9},
	}
	lineas := []string{debeJSON(t, cab)}
	for i, a := range aperturas {
		lineas = append(lineas, debeJSON(t, map[string]any{"t": i * 33, "manos": []any{manoConApertura(a)}}))
	}
	return strings.Join(lineas, "\n") + "\n"
}

// Una toma del modelo nuevo va a su carpeta, conserva el panel de la cabecera
// y sale con el estado del pellizco anotado cuadro a cuadro.
func TestGuardarTomaDePellizco(t *testing.T) {
	srv, dir := probar(t)
	cuerpo := tomaDePellizco(t, []float64{0.60, 0.25, 0.35, 0.44, 0.50})
	cod, res := pedir(t, "POST", srv.URL+"/guardar", cuerpo)
	if cod != 200 {
		t.Fatalf("%d", cod)
	}
	if res["archivo"] != "arrastrar-arriba-buena-001.jsonl" || res["toma"] != "arrastrar-arriba" {
		t.Fatalf("%v", res)
	}
	p, _ := res["pellizco"].(map[string]any)
	if p["cierres"] != 1.0 || p["cuadros_con_pellizco"] != 3.0 {
		t.Fatalf("resumen: %v", p)
	}
	ruta := filepath.Join(dir, "pellizco", "arrastrar-arriba-buena-001.jsonl")
	f, err := os.Open(ruta)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gestures.LeerGrabacion(filepath.Base(ruta), f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if g.Cabecera.Toma != "arrastrar-arriba" || g.Cabecera.Panel == nil || g.Cabecera.Panel.X1 != 0.9 {
		t.Fatalf("cabecera: %+v", g.Cabecera)
	}
	quiere := []bool{false, true, true, true, false}
	for i, c := range g.Cuadros {
		if len(c.Manos) != 1 || c.Manos[0].Pellizco == nil {
			t.Fatalf("cuadro %d sin pellizco: %+v", i, c)
		}
		if c.Manos[0].Pellizco.Cerrado != quiere[i] {
			t.Errorf("cuadro %d: cerrado %v, quería %v (apertura %.2f)", i, c.Manos[0].Pellizco.Cerrado, quiere[i], c.Manos[0].Pellizco.Apertura)
		}
	}
	_, cuenta := pedir(t, "GET", srv.URL+"/cuenta", "")
	pellizco, _ := cuenta["pellizco"].(map[string]any)
	if pellizco["arrastrar-arriba-buena"] != 1.0 {
		t.Fatalf("cuenta del pellizco: %v", cuenta)
	}
	if cod, _ := pedir(t, "POST", srv.URL+"/descartar?archivo=arrastrar-arriba-buena-001.jsonl", ""); cod != 200 {
		t.Fatalf("no descartó la toma del pellizco: %d", cod)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatal("la toma del pellizco sigue en su carpeta")
	}
}

func TestTomaDePellizcoInvalida(t *testing.T) {
	srv, _ := probar(t)
	frame := debeJSON(t, map[string]any{"t": 0, "manos": []any{manoConApertura(0.2)}})
	casos := []string{
		`{"gesto":"NINGUNO","luz":"buena","toma":"saludar"}`,
		`{"gesto":"ACTIVAR","luz":"buena","toma":"normal"}`,
		`{"gesto":"NINGUNO","luz":"buena","panel":{"x0":0.1,"y0":0.1,"x1":0.9,"y1":0.9}}`,
		`{"gesto":"NINGUNO","luz":"buena","toma":"normal","panel":{"x0":0.9,"y0":0.1,"x1":0.1,"y1":0.9}}`,
		`{"gesto":"NINGUNO","luz":"buena","toma":"normal","panel":{"x0":1.1,"y0":0.1,"x1":1.2,"y1":0.9}}`,
	}
	for _, cab := range casos {
		if cod, _ := pedir(t, "POST", srv.URL+"/guardar", cab+"\n"+frame+"\n"); cod != 400 {
			t.Errorf("aceptó %s (%d)", cab, cod)
		}
	}
}
