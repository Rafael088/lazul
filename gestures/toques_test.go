package gestures

import (
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

// --- secuencias inventadas --------------------------------------------------

func izq(clave int) ClaveMano { return ClaveMano{N: clave} }
func der(clave int) ClaveMano { return ClaveMano{Derecha: true, N: clave} }
func armada(c ClaveMano, x, y float64) ManoArmada {
	return ManoArmada{Clave: c, X: x, Y: y}
}

// resultado arma un ResultadoEmbrague como los que devuelve el embrague, sin
// pasar por la cámara: ManosArmadas y Armadas van siempre de la mano.
func resultado(t int64, armadas ...ManoArmada) ResultadoEmbrague {
	return ResultadoEmbrague{TMs: t, ManosArmadas: len(armadas), Armadas: armadas}
}

func TestToquesDeSecuenciasInventadas(t *testing.T) {
	casos := []struct {
		nombre     string
		resultados []ResultadoEmbrague
		quiere     []CuadroToques
	}{
		{
			nombre: "una mano apoya, mueve y levanta",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 0.20, 0.30)),
				resultado(67, armada(izq(0), 0.25, 0.35)),
				resultado(134),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.20, Y: 0.30, Apoyado: true}}},
				{TMs: 67, Dedos: []Dedo{{Ranura: 0, X: 0.25, Y: 0.35, Apoyado: true}}},
				{TMs: 134, Dedos: []Dedo{{Ranura: 0, X: 0.25, Y: 0.35, Apoyado: false}}},
			},
		},
		{
			nombre: "el dedo levantado se queda en la última posición",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(der(0), 0.60, 0.70)),
				resultado(67, armada(der(0), 0.65, 0.75)),
				resultado(134),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.60, Y: 0.70, Apoyado: true}}},
				{TMs: 67, Dedos: []Dedo{{Ranura: 0, X: 0.65, Y: 0.75, Apoyado: true}}},
				{TMs: 134, Dedos: []Dedo{{Ranura: 0, X: 0.65, Y: 0.75, Apoyado: false}}},
			},
		},
		{
			nombre: "dos manos, dos dedos, aunque el resultado cambie de orden",
			resultados: []ResultadoEmbrague{
				// El embrague ya las ordena, pero ToquesDe no depende de eso.
				resultado(0, armada(der(0), 0.80, 0.50), armada(izq(0), 0.20, 0.50)),
				resultado(67, armada(izq(0), 0.20, 0.60), armada(der(0), 0.80, 0.60)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: true},
				}},
				{TMs: 67, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.60, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.60, Apoyado: true},
				}},
			},
		},
		{
			nombre: "si una mano se pierde, su dedo se levanta y la otra conserva ranura",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 0.20, 0.50), armada(der(0), 0.80, 0.50)),
				resultado(67, armada(izq(0), 0.20, 0.60)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: true},
				}},
				{TMs: 67, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.60, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: false},
				}},
			},
		},
		{
			nombre: "la nueva mano ocupa la ranura libre, no la de la que sigue",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 0.20, 0.50)),
				resultado(67, armada(izq(0), 0.20, 0.55), armada(der(0), 0.80, 0.50)),
				resultado(134, armada(der(0), 0.80, 0.55)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: true}}},
				{TMs: 67, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.55, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: true},
				}},
				{TMs: 134, Dedos: []Dedo{
					// La derecha sigue con la ranura 1; la izquierda se
					// levanta con la suya (0).
					{Ranura: 0, X: 0.20, Y: 0.55, Apoyado: false},
					{Ranura: 1, X: 0.80, Y: 0.55, Apoyado: true},
				}},
			},
		},
		{
			nombre: "tras levantar todo, la ranura 0 vuelve a ser la primera",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 0.20, 0.50)),
				resultado(67),
				resultado(134, armada(der(1), 0.70, 0.40)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: true}}},
				{TMs: 67, Dedos: []Dedo{{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: false}}},
				{TMs: 134, Dedos: []Dedo{{Ranura: 0, X: 0.70, Y: 0.40, Apoyado: true}}},
			},
		},
		{
			nombre: "la ranura que se levanta no se reutiliza en el mismo cuadro",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 0.20, 0.50), armada(der(0), 0.80, 0.50)),
				// Se va la derecha y aparece otra derecha en el mismo cuadro.
				resultado(67, armada(izq(0), 0.20, 0.55), armada(der(1), 0.70, 0.50)),
				resultado(134, armada(izq(0), 0.20, 0.60), armada(der(1), 0.70, 0.55)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: true},
				}},
				{TMs: 67, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.55, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: false},
				}},
				{TMs: 134, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.60, Apoyado: true},
					{Ranura: 1, X: 0.70, Y: 0.55, Apoyado: true},
				}},
			},
		},
		{
			nombre: "la mano que se sale del panel toca el borde",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 1.20, -0.10)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{{Ranura: 0, X: 1, Y: 0, Apoyado: true}}},
			},
		},
		{
			nombre: "sin manos no hay dedos",
			resultados: []ResultadoEmbrague{
				resultado(0),
				resultado(67),
			},
			quiere: []CuadroToques{
				{TMs: 0},
				{TMs: 67},
			},
		},
		{
			nombre: "una tercera mano no roba la ranura de las que tocan",
			resultados: []ResultadoEmbrague{
				resultado(0, armada(izq(0), 0.20, 0.50), armada(der(0), 0.80, 0.50)),
				resultado(67, armada(izq(0), 0.20, 0.55), armada(der(0), 0.80, 0.55), armada(der(1), 0.50, 0.50)),
			},
			quiere: []CuadroToques{
				{TMs: 0, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.50, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.50, Apoyado: true},
				}},
				{TMs: 67, Dedos: []Dedo{
					{Ranura: 0, X: 0.20, Y: 0.55, Apoyado: true},
					{Ranura: 1, X: 0.80, Y: 0.55, Apoyado: true},
				}},
			},
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			obtuvo := ToquesDe(caso.resultados)
			if !reflect.DeepEqual(obtuvo, caso.quiere) {
				t.Fatalf("cuadros:\n obtuve %s\n quiero %s", formatear(obtuvo), formatear(caso.quiere))
			}
		})
	}
}

// El prefijo de una secuencia da los mismos cuadros que la secuencia entera:
// así el llamador en vivo puede ir entregando solo lo nuevo.
func TestToquesDeElPrefijoNoCambiaLosCuadros(t *testing.T) {
	resultados := []ResultadoEmbrague{
		resultado(0, armada(izq(0), 0.20, 0.50)),
		resultado(67, armada(der(0), 0.80, 0.50), armada(izq(0), 0.20, 0.55)),
		resultado(134, armada(der(0), 0.80, 0.55)),
		resultado(201),
	}
	entero := ToquesDe(resultados)
	for n := 1; n <= len(resultados); n++ {
		prefijo := ToquesDe(resultados[:n])
		if !reflect.DeepEqual(prefijo, entero[:n]) {
			t.Fatalf("el prefijo de %d no coincide:\n obtuve %s\n quiero %s",
				n, formatear(prefijo), formatear(entero[:n]))
		}
	}
}

func formatear(cuadros []CuadroToques) string {
	s := ""
	for _, c := range cuadros {
		s += fmt.Sprintf("{t=%d %v} ", c.TMs, c.Dedos)
	}
	return s
}

// --- el embrague publica las manos armadas ---------------------------------

func TestEmbraguePublicaCadaManoArmada(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	var r ResultadoEmbrague
	for i := 0; i < 4; i++ {
		r = med.Cuadro(cuadroPanel(int64(i)*67, manoEnPanel(-0.5, 0.2, .2, true)))
	}
	if r.ManosArmadas != 1 || len(r.Armadas) != 1 {
		t.Fatalf("una mano armada y una publicada: %+v", r)
	}
	m := r.Armadas[0]
	if m.Clave != izq(0) {
		t.Fatalf("clave %v, quiere %v", m.Clave, izq(0))
	}
	// El panel de prueba es [-1,1]² y la posición de la mano va de -1 a 1:
	// -0,5 y 0,2 quedan en 0,25 y 0,6 del panel.
	if math.Abs(m.X-0.25) > 1e-6 || math.Abs(m.Y-0.6) > 1e-6 {
		t.Fatalf("posición (%v,%v), quiere (0.25,0.6)", m.X, m.Y)
	}
	if r.TMs != 3*67 {
		t.Fatalf("TMs %d, quiere %d", r.TMs, 3*67)
	}

	// Al soltar, la mano deja de estar armada y no se publica.
	for i := 0; i < 4; i++ {
		r = med.Cuadro(cuadroPanel(int64(4+i)*67, manoEnPanel(-0.5, 0.2, .2, false)))
	}
	if r.ManosArmadas != 0 || len(r.Armadas) != 0 {
		t.Fatalf("tras soltar no queda mano armada: %+v", r)
	}
}

func TestEmbragueOrdenaDosManosPorClave(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	manoIzquierda := manoEnPanel(-0.3, 0, .2, true)
	manoDerecha := manoEnPanel(0.3, 0, .2, true)
	manoDerecha.Derecha = true
	var r ResultadoEmbrague
	for i := 0; i < 4; i++ {
		r = med.Cuadro(cuadroPanel(int64(i)*67, manoDerecha, manoIzquierda))
	}
	if len(r.Armadas) != 2 {
		t.Fatalf("dos manos armadas: %+v", r)
	}
	if r.Armadas[0].Clave != izq(0) || r.Armadas[1].Clave != der(0) {
		t.Fatalf("orden %v, quiere izquierda y luego derecha", []ClaveMano{r.Armadas[0].Clave, r.Armadas[1].Clave})
	}
	cuadros := ToquesDe([]ResultadoEmbrague{r})
	if len(cuadros[0].Dedos) != 2 ||
		cuadros[0].Dedos[0].Ranura != 0 || cuadros[0].Dedos[1].Ranura != 1 {
		t.Fatalf("dos dedos en las ranuras 0 y 1: %v", cuadros[0].Dedos)
	}
}

// --- las tomas reales -------------------------------------------------------

// TestToquesDeTomasRealesLevantanTodoDedo pasa cada toma de fixtures/pellizco
// por el embrague y comprueba la regla que evita un dedo pegado: todo dedo que
// apoya en algún cuadro se levanta después. Las tomas se cortan cuando la
// persona deja de mover la mano, así que a las que terminan con el pellizco
// cerrado se les quitan las manos (unos cuadros vacíos), que es lo que pasa
// de verdad al apartarla.
func TestToquesDeTomasRealesLevantanTodoDedo(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "pellizco"))
	if err != nil {
		t.Fatal(err)
	}
	tam := tamanoMedianoPellizco(gs)
	apoyos, levantes := 0, 0
	conDosDedos := 0
	for _, g := range gs {
		if len(g.Cuadros) == 0 {
			t.Fatalf("%s: toma vacía", g.Archivo)
		}
		med := NuevoMedidorEmbrague(calibracionDeToma(g, tam))
		resultados := make([]ResultadoEmbrague, 0, len(g.Cuadros)+12)
		var tUltimo int64
		var aspecto float32
		for _, c := range g.Cuadros {
			tUltimo, aspecto = c.TMs, c.Aspecto
			resultados = append(resultados, med.Cuadro(c))
		}
		// Se aparta la mano: cuadros sin manos hasta que el embrague suelte.
		for i := 0; i < 12 && resultados[len(resultados)-1].ManosArmadas > 0; i++ {
			tUltimo += 67
			resultados = append(resultados, med.Cuadro(Cuadro{TMs: tUltimo, Aspecto: aspecto}))
		}

		cuadros := ToquesDe(resultados)
		if len(cuadros) != len(resultados) {
			t.Fatalf("%s: %d cuadros de toques para %d resultados", g.Archivo, len(cuadros), len(resultados))
		}
		apoyado := [maxDedos]bool{}
		for i, c := range cuadros {
			if c.TMs != resultados[i].TMs {
				t.Fatalf("%s: cuadro %d con t=%d, quiere %d", g.Archivo, i, c.TMs, resultados[i].TMs)
			}
			vistos := [maxDedos]bool{}
			for _, d := range c.Dedos {
				if d.Ranura < 0 || d.Ranura >= maxDedos {
					t.Fatalf("%s: ranura %d fuera de 0..1", g.Archivo, d.Ranura)
				}
				if vistos[d.Ranura] {
					t.Fatalf("%s: cuadro %d con dos dedos en la ranura %d: %v", g.Archivo, i, d.Ranura, c.Dedos)
				}
				vistos[d.Ranura] = true
				if d.X < 0 || d.X > 1 || d.Y < 0 || d.Y > 1 {
					t.Fatalf("%s: cuadro %d con posición (%v,%v) fuera del panel", g.Archivo, i, d.X, d.Y)
				}
				if d.Apoyado && !apoyado[d.Ranura] {
					apoyos++
				}
				if d.Apoyado {
					apoyado[d.Ranura] = true
					continue
				}
				if !apoyado[d.Ranura] {
					t.Fatalf("%s: cuadro %d levanta la ranura %d, que no estaba apoyada", g.Archivo, i, d.Ranura)
				}
				apoyado[d.Ranura] = false
				levantes++
			}
			dos := 0
			for _, d := range c.Dedos {
				if d.Apoyado {
					dos++
				}
			}
			if dos == 2 {
				conDosDedos++
			}
		}
		for n, sigue := range apoyado {
			if sigue {
				t.Fatalf("%s: la ranura %d sigue apoyada al final", g.Archivo, n)
			}
		}
	}
	if apoyos == 0 {
		t.Fatal("ninguna toma apoyó un dedo: no se está probando nada")
	}
	if apoyos != levantes {
		t.Fatalf("%d apoyos y %d levantes: quedó un dedo pegado", apoyos, levantes)
	}
	if conDosDedos == 0 {
		t.Fatal("ninguna toma tuvo dos dedos a la vez: falta la prueba del zoom")
	}
	t.Logf("%d tomas: %d toques, todos levantados, %d cuadros con dos dedos",
		len(gs), apoyos, conDosDedos)
}
