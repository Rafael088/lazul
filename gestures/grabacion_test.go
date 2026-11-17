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

func TestPellizcoDesconocidoEsError(t *testing.T) {
	for _, cabecera := range []string{
		`{"gesto":"NINGUNO","toma":"saludar"}`,
		`{"gesto":"ACTIVAR","toma":"normal"}`,
		`{"gesto":"NINGUNO","panel":{"x0":0.1,"y0":0.1,"x1":0.9,"y1":0.9}}`,
		`{"gesto":"NINGUNO","toma":"normal","panel":{"x0":0.9,"y0":0.1,"x1":0.1,"y1":0.9}}`,
		`{"gesto":"NINGUNO","toma":"normal","panel":{"x0":0,"y0":0,"x1":1.2,"y1":1}}`,
	} {
		if _, err := LeerGrabacion("x", strings.NewReader(cabecera)); err == nil {
			t.Errorf("aceptó %s", cabecera)
		}
	}
	puntos := strings.TrimSuffix(strings.Repeat("[0,0,0],", NumPuntos), ",")
	if _, err := LeerGrabacion("x", strings.NewReader(
		`{"gesto":"NINGUNO","toma":"normal"}`+"\n"+`{"t":0,"manos":[{"p":[`+puntos+`],"pellizco":{"apertura":-1}}]}`)); err == nil {
		t.Error("aceptó una apertura negativa")
	}
}

func TestEscribirYLeerTomaDePellizco(t *testing.T) {
	cab := Cabecera{
		Gesto: "NINGUNO", Luz: "mala", Ancho: 640, Alto: 480,
		Toma: "zoom-dos-manos", Panel: &Panel{X0: 0.1, Y0: 0.2, X1: 0.8, Y1: 0.9},
		Nota: "dos manos",
	}
	var m Mano
	for i := range m.Puntos {
		m.Puntos[i] = Punto{0.25, 0.5, 0}
	}
	derecha := EstadoPellizco{Cerrado: true, Apertura: 0.23456}
	izquierda := EstadoPellizco{Cerrado: false, Apertura: 0.61234}
	cuadros := []Cuadro{{
		TMs: 0,
		Manos: []Mano{
			{Derecha: true, Puntos: m.Puntos, Pellizco: &derecha},
			{Derecha: false, Puntos: m.Puntos, Pellizco: &izquierda},
		},
	}}
	var b strings.Builder
	if err := EscribirGrabacion(&b, cab, cuadros); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	for _, trozo := range []string{
		`"toma":"zoom-dos-manos"`, `"panel":{"x0":0.1,"y0":0.2,"x1":0.8,"y1":0.9}`,
		`"pellizco":{"cerrado":true,"apertura":0.2346}`, `"pellizco":{"cerrado":false,"apertura":0.6123}`,
	} {
		if !strings.Contains(s, trozo) {
			t.Errorf("falta %s en:\n%s", trozo, s)
		}
	}
	g, err := LeerGrabacion("x", strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	if g.Cabecera.Toma != cab.Toma || g.Cabecera.Panel == nil || *g.Cabecera.Panel != *cab.Panel {
		t.Fatalf("cabecera: %+v", g.Cabecera)
	}
	if len(g.Cuadros) != 1 || len(g.Cuadros[0].Manos) != 2 {
		t.Fatalf("cuadro: %+v", g.Cuadros)
	}
	if p := g.Cuadros[0].Manos[0].Pellizco; p == nil || !p.Cerrado || p.Apertura != 0.2346 {
		t.Fatalf("pellizco derecho: %+v", p)
	}
	if p := g.Cuadros[0].Manos[1].Pellizco; p == nil || p.Cerrado || p.Apertura != 0.6123 {
		t.Fatalf("pellizco izquierdo: %+v", p)
	}
}

func TestTactilDesconocidoEsError(t *testing.T) {
	for _, cabecera := range []string{
		`{"gesto":"NINGUNO","tactil":"CLIC"}`,
		`{"gesto":"NINGUNO","tactil":"SCROLL","direccion_tactil":"CENTRO"}`,
		`{"gesto":"NINGUNO","inicio_tactil":"dormido"}`,
	} {
		if _, err := LeerGrabacion("x", strings.NewReader(cabecera)); err == nil {
			t.Errorf("aceptó %s", cabecera)
		}
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
