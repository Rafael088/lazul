package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIdentidadGuardarYCargar(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "identidad.pem")
	a, err := CargarOCrearIdentidad(ruta, "PC de prueba")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CargarOCrearIdentidad(ruta, "otro nombre")
	if err != nil {
		t.Fatal(err)
	}
	if a.Huella != b.Huella {
		t.Fatalf("la huella cambió al recargar: %v → %v", a.Huella, b.Huella)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(ruta); fi.Mode().Perm() != 0o600 {
			t.Fatalf("permisos %v; la clave privada debe quedar en 0600", fi.Mode().Perm())
		}
	}
	c, _ := NuevaIdentidad("x")
	if c.Huella == a.Huella {
		t.Fatal("dos identidades nuevas con la misma huella")
	}
}

func TestHuellaTextoIdaYVuelta(t *testing.T) {
	id, _ := NuevaIdentidad("x")
	h, err := ParsearHuella(id.Huella.String())
	if err != nil || h != id.Huella {
		t.Fatalf("%v %v", h, err)
	}
	if len(id.Huella.Corta()) != 19 {
		t.Fatalf("corta: %q", id.Huella.Corta())
	}
	for _, malo := range []string{"", "abc", id.Huella.String() + "00"} {
		if _, err := ParsearHuella(malo); err == nil {
			t.Errorf("%q debería fallar", malo)
		}
	}
}

func TestParesPersisten(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "pares.json")
	p, err := AbrirPares(ruta)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := NuevaIdentidad("x")
	if err := p.Fijar(Par{Huella: id.Huella, EquipoID: "e1", Nombre: "PC"}); err != nil {
		t.Fatal(err)
	}
	q, err := AbrirPares(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if par, ok := q.Buscar(id.Huella); !ok || par.Nombre != "PC" || par.EquipoID != "e1" {
		t.Fatalf("tras recargar: %v %v", par, ok)
	}
	if err := q.Olvidar(id.Huella); err != nil {
		t.Fatal(err)
	}
	r, _ := AbrirPares(ruta)
	if _, ok := r.Buscar(id.Huella); ok {
		t.Fatal("olvidado y sigue")
	}
}
