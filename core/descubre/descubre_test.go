package descubre

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

var huella = strings.Repeat("ab", 32)

func TestTxtIdaYVuelta(t *testing.T) {
	a := Anuncio{EquipoID: "e1", Nombre: "PC = estudio", Huella: huella, Dir: "192.168.1.5:4100"}
	b, err := deTxt(txt(a), "192.168.1.5", 4100)
	if err != nil || b != a {
		t.Fatalf("%+v %v", b, err)
	}
}

func TestTxtIncompletoSeDescarta(t *testing.T) {
	for _, campos := range [][]string{
		{"id=e1", "huella=" + huella},        // sin versión
		{"v=2", "id=e1", "huella=" + huella}, // versión futura
		{"v=1", "huella=" + huella},          // sin id
		{"v=1", "id=e1", "huella=abc"},       // huella corta
	} {
		if _, err := deTxt(campos, "10.0.0.1", 1); err == nil {
			t.Errorf("%q debía descartarse", campos)
		}
	}
	if _, err := deTxt(txt(Anuncio{EquipoID: "e", Huella: huella}), "", 1); err == nil {
		t.Error("sin dirección debía descartarse")
	}
}

func TestMemoriaAnunciaYDetiene(t *testing.T) {
	var r Memoria
	d1, err := r.Anunciar(Anuncio{EquipoID: "a", Nombre: "A", Huella: huella, Dir: "127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Anunciar(Anuncio{EquipoID: "b", Nombre: "B", Huella: huella, Dir: "127.0.0.1:2"}); err != nil {
		t.Fatal(err)
	}
	if l, _ := r.Buscar(context.Background()); len(l) != 2 || l[0].Nombre != "A" {
		t.Fatalf("%v", l)
	}
	d1()
	if l, _ := r.Buscar(context.Background()); len(l) != 1 || l[0].Nombre != "B" {
		t.Fatalf("tras detener: %v", l)
	}
	if _, err := r.Anunciar(Anuncio{EquipoID: "c", Huella: "x", Dir: "127.0.0.1:3"}); err == nil {
		t.Fatal("aceptó un anuncio inválido")
	}
}

// Con red de verdad: LAZUL_MDNS=1 go test ./core/descubre -run MDNS
func TestMDNSDeVerdad(t *testing.T) {
	if os.Getenv("LAZUL_MDNS") == "" {
		t.Skip("necesita multicast en la red local; LAZUL_MDNS=1 para correrla")
	}
	var r Red = MDNS{}
	a := Anuncio{EquipoID: "prueba-" + time.Now().Format("150405"), Nombre: "Prueba", Huella: huella, Dir: "127.0.0.1:4100"}
	detener, err := r.Anunciar(a)
	if err != nil {
		t.Fatal(err)
	}
	defer detener()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l, _ := r.Buscar(ctx)
	for _, b := range l {
		if b.EquipoID == a.EquipoID && b.Huella == huella {
			t.Logf("encontrado en %s", b.Dir)
			return
		}
	}
	t.Fatalf("no se encontró el propio anuncio entre %v", l)
}
