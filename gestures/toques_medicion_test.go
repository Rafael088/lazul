package gestures

import (
	"math"
	"path/filepath"
	"testing"
)

func TestClasificarToque(t *testing.T) {
	casos := []struct {
		nombre    string
		duracion  int64
		recorrido float64
		quiere    TipoToque
	}{
		{"corto y quieto es tap", 180, 0.01, ToqueTap},
		{"justo en el límite del tap", ToqueTapMs - 1, 0.0, ToqueTap},
		{"en el límite ya no es tap", ToqueTapMs, 0.0, ToqueSinClase},
		{"mover lo hace arrastre aunque sea corto", 180, 0.20, ToqueArrastre},
		{"justo en el límite de quieto sigue tap", 180, ToqueQuieto, ToqueTap},
		{"pasarse de quieto es arrastre", 180, ToqueQuieto + 0.01, ToqueArrastre},
		{"largo y quieto es pulsación larga", 900, 0.01, ToqueLarga},
		{"en el límite de larga no llega", ToqueLargaMs, 0.0, ToqueSinClase},
		{"pasado el límite es larga", ToqueLargaMs + 1, 0.0, ToqueLarga},
		{"largo pero movido es arrastre", 900, 0.30, ToqueArrastre},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if obtuvo := clasificarToque(caso.duracion, caso.recorrido); obtuvo != caso.quiere {
				t.Fatalf("%d ms y %.2f: %s, quiere %s", caso.duracion, caso.recorrido, obtuvo, caso.quiere)
			}
		})
	}
}

func TestToquesDeCuadrosEmparejaApoyoYLevantar(t *testing.T) {
	cuadros := []CuadroToques{
		{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.2, Y: 0.3, Apoyado: true}}},
		{TMs: 100, Dedos: []Dedo{{Ranura: 0, X: 0.3, Y: 0.3, Apoyado: true}}},
		{TMs: 300, Dedos: []Dedo{{Ranura: 0, X: 0.3, Y: 0.3, Apoyado: false}}},
	}
	toques := toquesDeCuadros(cuadros)
	if len(toques) != 1 {
		t.Fatalf("%d toques, quiere 1: %+v", len(toques), toques)
	}
	tq := toques[0]
	if tq.Ranura != 0 || tq.InicioMs != 0 || tq.FinMs != 300 {
		t.Fatalf("toque %+v, quiere ranura 0 de 0 a 300", tq)
	}
	if math.Abs(tq.Recorrido-0.1) > 1e-9 {
		t.Fatalf("recorrido %v, quiere 0.1", tq.Recorrido)
	}
	if tq.Tipo != ToqueArrastre {
		t.Fatalf("tipo %s, quiere arrastre (se movió 0.1 del panel)", tq.Tipo)
	}
}

func TestToquesDeCuadrosDosDedos(t *testing.T) {
	cuadros := []CuadroToques{
		{TMs: 0, Dedos: []Dedo{
			{Ranura: 0, X: 0.2, Y: 0.5, Apoyado: true},
			{Ranura: 1, X: 0.8, Y: 0.5, Apoyado: true},
		}},
		{TMs: 600, Dedos: []Dedo{
			{Ranura: 0, X: 0.2, Y: 0.5, Apoyado: false},
			{Ranura: 1, X: 0.7, Y: 0.5, Apoyado: false},
		}},
	}
	toques := toquesDeCuadros(cuadros)
	if len(toques) != 2 {
		t.Fatalf("%d toques, quiere 2: %+v", len(toques), toques)
	}
	if toques[0].Ranura != 0 || toques[1].Ranura != 1 {
		t.Fatalf("ranuras %d y %d, quiere 0 y 1", toques[0].Ranura, toques[1].Ranura)
	}
	if toques[0].Tipo != ToqueLarga || toques[1].Tipo != ToqueLarga {
		t.Fatalf("tipos %s y %s, quieren larga", toques[0].Tipo, toques[1].Tipo)
	}
}

func TestToquesDeCuadrosCierraElQueQuedaApoyado(t *testing.T) {
	cuadros := []CuadroToques{
		{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.2, Y: 0.5, Apoyado: true}}},
		{TMs: 1000, Dedos: []Dedo{{Ranura: 0, X: 0.2, Y: 0.5, Apoyado: true}}},
	}
	toques := toquesDeCuadros(cuadros)
	if len(toques) != 1 || toques[0].FinMs != 1000 {
		t.Fatalf("toques %+v, quiere uno cerrado en 1000", toques)
	}
}

func TestToquesDeCuadrosIgnoraLevantarSinApoyo(t *testing.T) {
	cuadros := []CuadroToques{{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.2, Y: 0.5, Apoyado: false}}}}
	if toques := toquesDeCuadros(cuadros); len(toques) != 0 {
		t.Fatalf("un levantar sin apoyo no es toque: %+v", toques)
	}
}

// TestCalibracionDeTomaSinPanel: una toma sin panel calibrado no se puede
// medir; se dice en vez de inventarse la calibración.
func TestCalibracionDeTomaSinPanel(t *testing.T) {
	g := Grabacion{Archivo: "toma.jsonl", Cabecera: Cabecera{Ancho: 640, Alto: 480}}
	if _, err := CalibracionDeToma(g, 0.2); err == nil {
		t.Fatal("una toma sin panel debería dar error")
	}
}

// TestMedirToquesConTomasReales pasa todas las tomas de fixtures/pellizco y
// comprueba la coherencia de lo medido: cada dedo apoyado se levanta, cada
// toque se cierra y las cuentas suman. Las cifras concretas se ven con
// ./dev replay -toques; aquí solo se fija lo que no debe romperse.
func TestMedirToquesConTomasReales(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "pellizco"))
	if err != nil {
		t.Fatal(err)
	}
	tomas, err := MedirToques(gs)
	if err != nil {
		t.Fatal(err)
	}
	if len(tomas) != len(gs) {
		t.Fatalf("%d medidas para %d tomas", len(tomas), len(gs))
	}
	total := ContarToques(tomas)
	suma := ContarToques(nil)
	for _, tm := range tomas {
		if tm.Archivo == "" || tm.Toma == "" {
			t.Fatalf("toma sin nombre: %+v", tm)
		}
		if tm.FueraDelPanel < 0 {
			t.Fatalf("%s: pellizcos fuera %d", tm.Archivo, tm.FueraDelPanel)
		}
		for i, tq := range tm.Toques {
			if tq.Ranura < 0 || tq.Ranura >= maxDedos {
				t.Fatalf("%s: ranura %d fuera de 0..1", tm.Archivo, tq.Ranura)
			}
			if tq.FinMs < tq.InicioMs {
				t.Fatalf("%s: toque al revés %+v", tm.Archivo, tq)
			}
			for _, p := range []PuntoPanel{tq.Inicio, tq.Fin} {
				if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
					t.Fatalf("%s: posición %+v fuera del panel", tm.Archivo, p)
				}
			}
			if tq.Recorrido < 0 {
				t.Fatalf("%s: recorrido %v", tm.Archivo, tq.Recorrido)
			}
			if quiere := clasificarToque(tq.DuracionMs(), tq.Recorrido); tq.Tipo != quiere {
				t.Fatalf("%s: tipo %s, quiere %s", tm.Archivo, tq.Tipo, quiere)
			}
			if i > 0 && tm.Toques[i-1].InicioMs > tq.InicioMs {
				t.Fatalf("%s: toques desordenados", tm.Archivo)
			}
		}
		c := ContarToques([]TomaToques{tm})
		suma.Tomas++
		suma.TomasConToques += c.TomasConToques
		suma.Toques += c.Toques
		suma.Tap += c.Tap
		suma.Larga += c.Larga
		suma.Arrastre += c.Arrastre
		suma.SinClase += c.SinClase
		suma.FueraDelPanel += c.FueraDelPanel
	}
	if suma != total {
		t.Fatalf("suma por toma %+v, total %+v", suma, total)
	}
	// Las tomas reales dan toques de sobra; si bajan a cero es que el
	// embrague o ToquesDe cambiaron de comportamiento.
	if total.Toques < 50 || total.TomasConToques < 40 {
		t.Fatalf("pocos toques con las tomas reales: %+v", total)
	}
	if total.Arrastre == 0 {
		t.Fatalf("ningún arrastre en las tomas de arrastre: %+v", total)
	}
	if total.FueraDelPanel == 0 {
		t.Fatalf("la regla de empezar dentro del panel no descartó ningún pellizco: %+v", total)
	}
}
