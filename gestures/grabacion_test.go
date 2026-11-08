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

func TestEscribirYLeer(t *testing.T) {
	cab := Cabecera{Gesto: "SOLTAR", Luz: "mala", Ancho: 1280, Alto: 720, Nota: "ida y vuelta"}
	var m Mano
	for i := range m.Puntos {
		m.Puntos[i] = Punto{0.123456, 0.5, -0.00001}
	}
	var b strings.Builder
	if err := EscribirGrabacion(&b, cab, []Cuadro{{TMs: 0, Manos: []Mano{m}}, {TMs: 33}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "-0]") || !strings.Contains(b.String(), `"manos":[]`) {
		t.Errorf("formato: %s", b.String())
	}
	g, err := LeerGrabacion("x", strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if g.Cabecera != cab || g.Esperado != lazulv1.Gesto_GESTO_SOLTAR || g.Inicio != ModoAgarrando || g.Duracion() != 33 {
		t.Fatalf("%+v", g)
	}
	if c := g.Cuadros[0]; c.Aspecto != 1280.0/720 || c.Manos[0].Puntos[3].X != 0.1235 {
		t.Fatalf("cuadro: %+v", c)
	}
}

func TestModoInicial(t *testing.T) {
	casos := map[Cabecera]Modo{
		{Gesto: "ACTIVAR"}:                      ModoReposo,
		{Gesto: "AGARRAR"}:                      ModoActivo,
		{Gesto: "MANDAR_APP"}:                   ModoActivo,
		{Gesto: "SOLTAR"}:                       ModoAgarrando,
		{Gesto: "NINGUNO"}:                      ModoReposo,
		{Gesto: "NINGUNO", Inicio: "agarrando"}: ModoAgarrando,
		{Gesto: "AGARRAR", Inicio: "reposo"}:    ModoReposo,
	}
	for c, m := range casos {
		if got, err := c.ModoInicial(); err != nil || got != m {
			t.Errorf("%+v: %v %v", c, got, err)
		}
	}
	if _, err := LeerGrabacion("x", strings.NewReader(`{"gesto":"ACTIVAR","inicio":"dormido"}`)); err == nil {
		t.Error("aceptó un inicio que no existe")
	}
}

func TestPor30Min(t *testing.T) {
	if got := (Negativos{Falsos: 1, Ms: 10 * 60 * 1000}).Por30Min(); got != 3 {
		t.Errorf("1 en 10 min: %v por 30 min", got)
	}
}
