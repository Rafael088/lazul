package gestures

import (
	"strings"
	"testing"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

func TestLeerDirectorio(t *testing.T) {
	gs, err := LeerDirectorio("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 2 {
		t.Fatalf("%d grabaciones", len(gs))
	}
	l := gs[0]
	if l.Esperado != lazulv1.Gesto_GESTO_ACTIVAR || len(l.Cuadros) != 2 || len(l.Cuadros[0].Manos) != 1 || l.Cuadros[1].TMs != 33 {
		t.Fatalf("leída mal: %+v", l)
	}
	if p := l.Cuadros[0].Manos[0].Puntos[IndicePunta]; p.X != 0.58 {
		t.Fatalf("punta del índice: %+v", p)
	}
	m, _ := Medir(gs)
	if m.Positivas != 1 || m.Negativas != 1 {
		t.Fatalf("%+v", m)
	}
}

func TestManoIncompletaEsError(t *testing.T) {
	in := `{"gesto":"AGARRAR","luz":"buena"}` + "\n" + `{"t":0,"manos":[{"p":[[0,0,0]]}]}`
	if _, err := LeerGrabacion("x", strings.NewReader(in)); err == nil {
		t.Fatal("aceptó una mano de 1 punto")
	}
}

func TestGestoDesconocidoEsError(t *testing.T) {
	if _, err := LeerGrabacion("x", strings.NewReader(`{"gesto":"SALUDAR"}`)); err == nil {
		t.Fatal("aceptó un gesto que no existe")
	}
}

func TestAcierto(t *testing.T) {
	a := lazulv1.Gesto_GESTO_AGARRAR
	casos := []struct {
		r    Resultado
		want bool
	}{
		{Resultado{Esperado: a, Emitidos: []lazulv1.Gesto{a}}, true},
		{Resultado{Esperado: a}, false},
		{Resultado{Esperado: a, Emitidos: []lazulv1.Gesto{a, a}}, false},
		{Resultado{Esperado: lazulv1.Gesto_GESTO_NINGUNO}, true},
		{Resultado{Esperado: lazulv1.Gesto_GESTO_NINGUNO, Emitidos: []lazulv1.Gesto{a}}, false},
	}
	for i, c := range casos {
		if c.r.Acierto() != c.want {
			t.Errorf("caso %d: %v", i, c.r)
		}
	}
}
