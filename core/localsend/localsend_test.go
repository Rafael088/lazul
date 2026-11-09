package localsend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func servidor(t *testing.T, permitir bool) (*Servidor, *httptest.Server) {
	t.Helper()
	s := &Servidor{Bandeja: t.TempDir(), Info: Dispositivo{Alias: "prueba", Version: "2.0"},
		Permitir: func(*http.Request) bool { return permitir }}
	mux := http.NewServeMux()
	mux.Handle(Ruta, s)
	ts := httptest.NewTLSServer(mux)
	t.Cleanup(ts.Close)
	return s, ts
}

func TestEnviarYBuscar(t *testing.T) {
	s, ts := servidor(t, true)
	a := Archivo{Nombre: "../../captura 1.png", Mime: "image/png", Datos: bytes.Repeat([]byte("lazul"), 1000)}
	ses, err := Enviar(context.Background(), ts.Client(), ts.URL, Dispositivo{Alias: "yo", Version: "2.0"}, a)
	if err != nil {
		t.Fatal(err)
	}
	suma := a.SHA256()
	rec, err := s.Buscar(ses, suma[:])
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(rec.Ruta) != s.Bandeja || rec.Nombre != "captura 1.png" {
		t.Fatalf("quedó en %q como %q", rec.Ruta, rec.Nombre)
	}
	if b, _ := os.ReadFile(rec.Ruta); !bytes.Equal(b, a.Datos) {
		t.Fatal("los bytes no coinciden")
	}
	// Otro con el mismo nombre no lo pisa.
	ses2, err := Enviar(context.Background(), ts.Client(), ts.URL, Dispositivo{}, a)
	if err != nil {
		t.Fatal(err)
	}
	rec2, _ := s.Buscar(ses2, suma[:])
	if rec2.Ruta == rec.Ruta {
		t.Fatal("pisó el archivo anterior")
	}
	if _, err := s.Buscar(ses2, make([]byte, 32)); err != ErrNoRecibido {
		t.Fatalf("sha256 ajeno: %v", err)
	}
	if _, err := s.Buscar(ses, suma[:]); err != ErrNoRecibido {
		t.Fatalf("se pudo entregar dos veces: %v", err)
	}
	// Las sesiones entregadas no ocupan lugar: caben más de MaxSesiones seguidas.
	for range MaxSesiones + 2 {
		ses, err := Enviar(context.Background(), ts.Client(), ts.URL, Dispositivo{}, a)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Buscar(ses, suma[:]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNoPermitido(t *testing.T) {
	_, ts := servidor(t, false)
	_, err := Enviar(context.Background(), ts.Client(), ts.URL, Dispositivo{}, Archivo{Nombre: "a", Datos: []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("debía rechazar: %v", err)
	}
	// info sí responde (es pública en LocalSend).
	resp, err := ts.Client().Get(ts.URL + Ruta + "info")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("info: %v %v", resp, err)
	}
}

// prepara una sesión a mano y devuelve sessionId y token.
func preparar(t *testing.T, ts *httptest.Server, cuerpo string) (string, string) {
	t.Helper()
	resp, err := pedir(context.Background(), ts.Client(), ts.URL+Ruta+"prepare-upload", "application/json", []byte(cuerpo))
	if err != nil {
		t.Fatal(err)
	}
	var rp respuestaPreparar
	if err := jsonDecode(resp, &rp); err != nil {
		t.Fatal(err)
	}
	return rp.SessionID, rp.Files["f"]
}

func subir(ts *httptest.Server, ses, token string, datos []byte) error {
	q := url.Values{"sessionId": {ses}, "fileId": {"f"}, "token": {token}}
	_, err := pedir(context.Background(), ts.Client(), ts.URL+Ruta+"upload?"+q.Encode(), "application/octet-stream", datos)
	return err
}

func TestSubidaMala(t *testing.T) {
	s, ts := servidor(t, true)
	// sha256 de "abc"
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	ses, tok := preparar(t, ts, `{"info":{"alias":"x","version":"2.0"},"files":{"f":{"id":"f","fileName":"a.txt","size":3,"fileType":"text/plain","sha256":"`+abc+`"}}}`)
	if err := subir(ts, ses, "otro", []byte("abc")); err == nil {
		t.Error("aceptó un token ajeno")
	}
	if err := subir(ts, ses, tok, []byte("abd")); err == nil {
		t.Error("aceptó un sha256 que no coincide")
	}
	if err := subir(ts, ses, tok, []byte("abcd")); err == nil {
		t.Error("aceptó más bytes de los anunciados")
	}
	if err := subir(ts, ses, tok, []byte("abc")); err != nil {
		t.Fatalf("la buena: %v", err)
	}
	if err := subir(ts, ses, tok, []byte("abc")); err == nil {
		t.Error("aceptó subir dos veces el mismo archivo")
	}
	entradas, _ := os.ReadDir(s.Bandeja)
	if len(entradas) != 1 {
		t.Errorf("en la bandeja quedaron %d archivos (sin temporales debía quedar 1)", len(entradas))
	}
	if _, err := pedir(context.Background(), ts.Client(), ts.URL+Ruta+"prepare-upload", "application/json",
		[]byte(`{"files":{"f":{"fileName":"x","size":`+"999999999999"+`}}}`)); err == nil {
		t.Error("aceptó un archivo más grande que MaxArchivo")
	}
}

func TestNombreLimpio(t *testing.T) {
	for in, want := range map[string]string{`..\..\win.ini`: "win.ini", "a/b/c.png": "c.png", "..": "", "x\x00y": "x_y", `con:"a"`: "con__a_"} {
		if got := nombreLimpio(in); got != want {
			t.Errorf("nombreLimpio(%q) = %q, esperaba %q", in, got, want)
		}
	}
}

func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }
