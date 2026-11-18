package gestures

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// --- manos sintéticas -------------------------------------------------------

// manoEnPanel construye una mano cuyo MuestraPanelDe da la posición y el
// tamaño pedidos, con el pellizco abierto o cerrado. El panel de prueba es el
// cuadrado proyectado [-1,1]², así que (x,y) en [-1,1] cae dentro.
func manoEnPanel(x, y, tam float64, cerrado bool) Mano {
	var m Mano
	medio := tam / math.Sqrt2 / 2
	cx, cy := x*tam+0.5, y*tam+0.5
	m.Puntos[Muneca] = Punto{float32(cx), float32(cy - medio), 0}
	m.Puntos[IndiceMCP] = Punto{float32(cx - medio), float32(cy), 0}
	m.Puntos[MedioMCP] = Punto{float32(cx), float32(cy + medio), 0}
	m.Puntos[AnularMCP] = Punto{float32(cx), float32(cy), 0}
	m.Puntos[MeniqueMCP] = Punto{float32(cx + medio), float32(cy), 0}
	apertura := 0.80
	if cerrado {
		apertura = 0.10
	}
	m.Puntos[PulgarPunta] = Punto{float32(cx), float32(cy), 0}
	m.Puntos[IndicePunta] = Punto{float32(cx + apertura*tam/math.Sqrt2), float32(cy), 0}
	return m
}

func cuadroPanel(t int64, manos ...Mano) Cuadro {
	return Cuadro{TMs: t, Aspecto: 1, Manos: manos}
}

func panelCuadrado(t *testing.T) CalibracionPanel {
	t.Helper()
	c, err := NuevaCalibracionPanel(muestraProyectada(-1, -1, .2), muestraProyectada(1, 1, .2))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// --- pruebas del embrague ---------------------------------------------------

func TestEmbragueIgnoraPellizcoQueEmpiezaFuera(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	// Empieza cerrado fuera (x=1.4 en el panel [-1,1]) y entra cerrado.
	var r ResultadoEmbrague
	for i := 0; i < 12; i++ {
		x := 1.4 - float64(i)*0.12
		r = med.Cuadro(cuadroPanel(int64(i)*67, manoEnPanel(x, 0, .2, true)))
	}
	if r.Fase != EmbragueDentro {
		t.Fatalf("la mano entró cerrada pero no debe agarrar: %+v", r)
	}
	// Al soltar dentro no hay arrastre: el pellizco nació fuera.
	for i := 0; i < 6; i++ {
		r = med.Cuadro(cuadroPanel(int64(12+i)*67, manoEnPanel(-0.2, 0, .2, false)))
		if r.Arrastre != nil {
			t.Fatalf("contó un arrastre que empezó fuera: %+v", r.Arrastre)
		}
	}
	if r.Fase != EmbragueDentro {
		t.Fatalf("fase tras soltar dentro: %+v", r)
	}
}

func TestEmbragueArrastraYClasificaLaDireccion(t *testing.T) {
	casos := []struct {
		nombre    string
		dx, dy    float64
		direccion DireccionArrastre
	}{
		{"arriba", 0, -0.8, ArrastreArriba},
		{"abajo", 0, 0.8, ArrastreAbajo},
		{"izquierda", -0.8, 0, ArrastreIzquierda},
		{"derecha", 0.8, 0, ArrastreDerecha},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			med := NuevoMedidorEmbrague(panelCuadrado(t))
			x, y := -0.5, 0.0
			var r ResultadoEmbrague
			for i := 0; i < 5; i++ {
				r = med.Cuadro(cuadroPanel(int64(i)*67, manoEnPanel(x, y, .2, true)))
			}
			if r.Fase != EmbragueAgarrando {
				t.Fatalf("no agarró: %+v", r)
			}
			for i := 1; i <= 10; i++ {
				px := x + caso.dx*float64(i)/10
				py := y + caso.dy*float64(i)/10
				r = med.Cuadro(cuadroPanel(int64(5+i)*67, manoEnPanel(px, py, .2, true)))
			}
			if r.Arrastre != nil {
				t.Fatalf("solté antes de tiempo: %+v", r.Arrastre)
			}
			var arrastres []Arrastre
			for i := 0; i < 5; i++ {
				r = med.Cuadro(cuadroPanel(int64(15+i)*67, manoEnPanel(x+caso.dx, y+caso.dy, .2, false)))
				if r.Arrastre != nil {
					arrastres = append(arrastres, *r.Arrastre)
				}
			}
			if len(arrastres) != 1 || arrastres[0].Direccion != caso.direccion {
				t.Fatalf("arrastre %q: %+v", caso.direccion, arrastres)
			}
			// El panel de prueba es [-1,1]², así que 0,8 en sus coordenadas
			// son 0,4 en unidades del panel.
			if arrastres[0].Distancia < 0.35 {
				t.Fatalf("distancia corta: %+v", arrastres[0])
			}
		})
	}
}

func TestEmbragueElParpadeoNoCortaElArrastre(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	// Agarra dentro y arrastra.
	for i := 0; i < 5; i++ {
		med.Cuadro(cuadroPanel(int64(i)*67, manoEnPanel(-0.5, 0, .2, true)))
	}
	var r ResultadoEmbrague
	for i := 1; i <= 8; i++ {
		r = med.Cuadro(cuadroPanel(int64(5+i)*67, manoEnPanel(-0.5+float64(i)*0.1, 0, .2, true)))
	}
	// Dos cuadros abiertos en medio (134 ms): no suelta.
	for i := 0; i < 2; i++ {
		r = med.Cuadro(cuadroPanel(int64(14+i)*67, manoEnPanel(0.3, 0, .2, false)))
		if r.Arrastre != nil || r.Fase != EmbragueAgarrando {
			t.Fatalf("el parpadeo cortó el arrastre: %+v", r)
		}
	}
	// Vuelve a cerrar y sigue arrastrando hasta el final.
	for i := 0; i < 5; i++ {
		r = med.Cuadro(cuadroPanel(int64(16+i)*67, manoEnPanel(0.3, 0, .2, true)))
	}
	var arrastres []Arrastre
	for i := 0; i < 5; i++ {
		r = med.Cuadro(cuadroPanel(int64(21+i)*67, manoEnPanel(0.8, 0, .2, false)))
		if r.Arrastre != nil {
			arrastres = append(arrastres, *r.Arrastre)
		}
	}
	if len(arrastres) != 1 || arrastres[0].Direccion != ArrastreDerecha {
		t.Fatalf("un solo arrastre al final: %+v", arrastres)
	}
}

func TestEmbragueDosManosHacenZoom(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	var r ResultadoEmbrague
	var zooms []Zoom
	var arrastres []Arrastre
	recoger := func(r ResultadoEmbrague) {
		if r.Zoom != nil {
			zooms = append(zooms, *r.Zoom)
		}
		if r.Arrastre != nil {
			arrastres = append(arrastres, *r.Arrastre)
		}
	}
	for i := 0; i < 4; i++ {
		r = med.Cuadro(cuadroPanel(int64(i)*67,
			manoEnPanel(-0.3, 0, .2, true), manoEnPanel(0.3, 0, .2, true)))
		recoger(r)
	}
	if r.Fase != EmbragueAgarrando || r.ManosArmadas != 2 {
		t.Fatalf("no agarraron: %+v", r)
	}
	for i := 1; i <= 10; i++ {
		sep := 0.3 + float64(i)*0.05
		r = med.Cuadro(cuadroPanel(int64(4+i)*67,
			manoEnPanel(-sep, 0, .2, true), manoEnPanel(sep, 0, .2, true)))
		recoger(r)
	}
	if len(zooms) == 0 {
		t.Fatal("separar las manos no emitió zoom")
	}
	for _, z := range zooms {
		if z.Direccion != ZoomAcercar {
			t.Fatalf("zoom al separar: %+v", zooms)
		}
	}
	// Al juntarlas, el zoom va al revés.
	zooms = nil
	for i := 10; i >= 1; i-- {
		sep := 0.3 + float64(i)*0.05
		r = med.Cuadro(cuadroPanel(int64(15+10-i)*67,
			manoEnPanel(-sep, 0, .2, true), manoEnPanel(sep, 0, .2, true)))
		recoger(r)
	}
	if len(zooms) == 0 {
		t.Fatal("juntar las manos no emitió zoom")
	}
	for _, z := range zooms {
		if z.Direccion != ZoomAlejar {
			t.Fatalf("zoom al juntar: %+v", zooms)
		}
	}
	if len(arrastres) != 0 {
		t.Fatalf("el zoom también contó arrastre: %+v", arrastres)
	}
}

// --- medida contra las tomas reales ----------------------------------------

// El panel de la cabecera está en coordenadas normalizadas de la imagen y no
// guarda el tamaño de la palma con que se calibró. Todas las tomas comparten
// el mismo panel, así que la prueba reconstruye la calibración con el tamaño
// mediano del corpus: la pertenencia al rectángulo no depende de ese valor y
// la tolerancia de profundidad sigue siendo realista.
func calibracionDeToma(g Grabacion, tamano float64) CalibracionPanel {
	cal, err := CalibracionDeToma(g, tamano)
	if err != nil {
		panic(err)
	}
	return cal
}

func tamanoMedianoPellizco(gs []Grabacion) float64 { return TamanoMedianoPellizco(gs) }

type medidaToma struct {
	arrastres []Arrastre
	zooms     []Zoom
}

func medirToma(g Grabacion, tamano float64) medidaToma {
	med := NuevoMedidorEmbrague(calibracionDeToma(g, tamano))
	var m medidaToma
	for _, c := range g.Cuadros {
		r := med.Cuadro(c)
		if r.Arrastre != nil {
			m.arrastres = append(m.arrastres, *r.Arrastre)
		}
		if r.Zoom != nil {
			m.zooms = append(m.zooms, *r.Zoom)
		}
	}
	// Si la toma termina con el pellizco cerrado, cuatro cuadros sin manos
	// cumplen los umbrales de soltar y cierran el embrague con la última
	// posición agarrada.
	if len(g.Cuadros) > 0 {
		fin := g.Cuadros[len(g.Cuadros)-1].TMs
		for i := 0; i < 4; i++ {
			r := med.Cuadro(Cuadro{TMs: fin + 1000 + int64(i)*67, Aspecto: g.Cuadros[0].Aspecto})
			if r.Arrastre != nil {
				m.arrastres = append(m.arrastres, *r.Arrastre)
			}
			if r.Zoom != nil {
				m.zooms = append(m.zooms, *r.Zoom)
			}
		}
	}
	return m
}

// direccionCruda mide la toma sin regla de panel: el pellizco más largo de
// cada toma y su desplazamiento neto. Es la línea base de 79/82 en datos
// crudos: si esto no sale, la toma está mal y no se le puede pedir al
// embrague más que a los datos.
func direccionCruda(g Grabacion, cal CalibracionPanel) (DireccionArrastre, float64) {
	type corrida struct {
		primero, ultimo PuntoPanel
		hay             bool
		abiertos        int
	}
	actual := map[ClaveMano]*corrida{}
	var corridas []corrida
	cerrar := func(c *corrida) {
		if c.hay {
			corridas = append(corridas, *c)
		}
	}
	for _, c := range g.Cuadros {
		vistos := map[bool]int{}
		presentes := make(map[ClaveMano]bool, len(c.Manos))
		for _, m := range c.Manos {
			clave := ClaveMano{Derecha: m.Derecha, N: vistos[m.Derecha]}
			vistos[m.Derecha]++
			presentes[clave] = true
			abierto := m.Pellizco == nil || !m.Pellizco.Cerrado
			if abierto {
				if r := actual[clave]; r != nil {
					r.abiertos++
					if r.abiertos >= 3 {
						cerrar(r)
						delete(actual, clave)
					}
				}
				continue
			}
			r := actual[clave]
			if r == nil {
				r = &corrida{}
				actual[clave] = r
			}
			r.abiertos = 0
			mu, ok := MuestraPanelDe(m, c.Aspecto)
			if !ok {
				continue
			}
			p, _ := cal.Mapear(mu)
			if !r.hay {
				r.primero, r.hay = p, true
			}
			r.ultimo = p
		}
		for clave, r := range actual {
			if !presentes[clave] {
				r.abiertos++
				if r.abiertos >= 3 {
					cerrar(r)
					delete(actual, clave)
				}
			}
		}
	}
	for _, r := range actual {
		cerrar(r)
	}
	var mejor corrida
	mayor := -1.0
	for _, r := range corridas {
		d := math.Hypot(r.ultimo.X-r.primero.X, r.ultimo.Y-r.primero.Y)
		if d > mayor {
			mayor, mejor = d, r
		}
	}
	if !mejor.hay {
		return "", 0
	}
	dx, dy := mejor.ultimo.X-mejor.primero.X, mejor.ultimo.Y-mejor.primero.Y
	d := math.Hypot(dx, dy)
	switch {
	case math.Abs(dx) >= math.Abs(dy) && dx > 0:
		return ArrastreDerecha, d
	case math.Abs(dx) >= math.Abs(dy):
		return ArrastreIzquierda, d
	case dy > 0:
		return ArrastreAbajo, d
	default:
		return ArrastreArriba, d
	}
}

func TestEmbragueConTomasRealesDeArrastre(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "pellizco"))
	if err != nil {
		t.Fatal(err)
	}
	tam := tamanoMedianoPellizco(gs)
	total, criollos, correctos := 0, 0, 0
	var malas, malDireccion, sinArrastre []string
	for _, g := range gs {
		if !strings.HasPrefix(g.Cabecera.Toma, "arrastrar-") {
			continue
		}
		total++
		quiere := DireccionArrastre(strings.TrimPrefix(g.Cabecera.Toma, "arrastrar-"))
		cal := calibracionDeToma(g, tam)
		cruda, distCruda := direccionCruda(g, cal)
		buena := cruda == quiere && distCruda >= 0.05
		if buena {
			criollos++
		} else {
			malas = append(malas, fmt.Sprintf("%s: crudo %s %.2f, quiere %s", g.Archivo, cruda, distCruda, quiere))
		}
		m := medirToma(g, tam)
		if len(m.arrastres) == 0 {
			sinArrastre = append(sinArrastre, g.Archivo)
			continue
		}
		correctos++
		if len(m.arrastres) > 1 {
			t.Logf("%s: %d arrastres (la toma tiene varios pellizcos separados)", g.Archivo, len(m.arrastres))
		}
		// Solo se juzga la dirección en las tomas que la propia grabación
		// sostiene; las mal grabadas no sirven de referencia.
		if !buena {
			continue
		}
		for _, a := range m.arrastres {
			if a.Direccion != quiere {
				malDireccion = append(malDireccion, fmt.Sprintf("%s: quiere %s, obtuve %+v", g.Archivo, quiere, a))
			}
		}
	}
	t.Logf("línea base cruda en la dirección del nombre: %d/%d", criollos, total)
	for _, m := range malas {
		t.Logf("toma cruda descartada %s", m)
	}
	t.Logf("tomas con al menos un arrastre del embrague: %d/%d", correctos, total)
	if len(sinArrastre) > 0 {
		t.Logf("ignoradas por empezar fuera del panel (la regla las descarta): %d", len(sinArrastre))
	}
	if correctos+len(sinArrastre) != total {
		t.Fatalf("cuentas incoherentes: %d con arrastre + %d sin = %d, tomas %d", correctos, len(sinArrastre), correctos+len(sinArrastre), total)
	}
	if len(sinArrastre) == 0 {
		t.Fatal("la regla de empezar dentro no descartó ninguna toma: revisa la reconstrucción del panel")
	}
	if len(malDireccion) > 0 {
		t.Fatalf("arrastres en dirección equivocada:\n%s", strings.Join(malDireccion, "\n"))
	}
	// Los datos crudos sostienen 79/82 tomas buenas (las otras 3 son tomas
	// mal grabadas; una tiene un solo cuadro). El embrague descarta además
	// las que empiezan el pellizco fuera del panel, como manda dec-fa72.
	if criollos < 79 {
		t.Fatalf("la línea base cruda bajó a %d/%d: revisa las tomas", criollos, total)
	}
	if correctos < 50 {
		t.Fatalf("el embrague solo reconoció %d tomas: demasiado bajo", correctos)
	}
}

// ambasManosEmpiezanDentro dice si la primera vez que las dos manos están
// pellizcadas las dos están dentro del panel. Una toma que empieza fuera se
// ignora, igual que un arrastre.
func ambasManosEmpiezanDentro(g Grabacion, cal CalibracionPanel) bool {
	for _, c := range g.Cuadros {
		if len(c.Manos) != 2 {
			continue
		}
		dentro, cerradas := 0, 0
		for _, m := range c.Manos {
			if m.Pellizco == nil || !m.Pellizco.Cerrado {
				continue
			}
			cerradas++
			if mu, ok := MuestraPanelDe(m, c.Aspecto); ok {
				if _, ok := cal.Mapear(mu); ok {
					dentro++
				}
			}
		}
		if cerradas == 2 {
			return dentro == 2
		}
	}
	return false
}

func TestEmbragueConTomasRealesDeZoom(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "pellizco"))
	if err != nil {
		t.Fatal(err)
	}
	tam := tamanoMedianoPellizco(gs)
	vistas, validas := 0, 0
	for _, g := range gs {
		if g.Cabecera.Toma != "zoom-dos-manos" {
			continue
		}
		vistas++
		if !ambasManosEmpiezanDentro(g, calibracionDeToma(g, tam)) {
			t.Logf("%s: las manos no empiezan las dos dentro del panel; no es un zoom válido", g.Archivo)
			continue
		}
		validas++
		m := medirToma(g, tam)
		if len(m.arrastres) != 0 {
			t.Fatalf("%s: el zoom contó arrastres %+v", g.Archivo, m.arrastres)
		}
		var acercar, alejar int
		for _, z := range m.zooms {
			switch z.Direccion {
			case ZoomAcercar:
				acercar++
			case ZoomAlejar:
				alejar++
			}
		}
		if acercar < 1 || alejar < 1 {
			t.Fatalf("%s: zooms acercar=%d alejar=%d (las tomas separan y juntan varias veces)", g.Archivo, acercar, alejar)
		}
		t.Logf("%s: %d pasos de zoom (%d acercar, %d alejar)", g.Archivo, acercar+alejar, acercar, alejar)
	}
	if vistas == 0 || validas < 7 {
		t.Fatalf("tomas de zoom: %d, válidas (empiezan dentro): %d", vistas, validas)
	}
}

func TestEmbragueNormalNoArrastra(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "pellizco"))
	if err != nil {
		t.Fatal(err)
	}
	tam := tamanoMedianoPellizco(gs)
	for _, g := range gs {
		if g.Cabecera.Toma != "normal" {
			continue
		}
		m := medirToma(g, tam)
		if len(m.arrastres) != 0 || len(m.zooms) != 0 {
			t.Fatalf("%s: arrastres %+v, zoom %+v", g.Archivo, m.arrastres, m.zooms)
		}
	}
}
