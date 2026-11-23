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
// cuadrado proyectado [-1,1]², así que (x,y) en [-1,1] cae dentro. Cerrado es
// un pellizco tibio (0,16: entre UmbralFirme y UmbralCerrar), que espera
// CuadrosParaApoyar; manoEnPanelApertura da cualquier otra apertura.
func manoEnPanel(x, y, tam float64, cerrado bool) Mano {
	apertura := 0.80
	if cerrado {
		apertura = 0.16
	}
	return manoEnPanelApertura(x, y, tam, apertura)
}

func manoEnPanelApertura(x, y, tam, apertura float64) Mano {
	var m Mano
	medio := tam / math.Sqrt2 / 2
	cx, cy := x*tam+0.5, y*tam+0.5
	m.Puntos[Muneca] = Punto{float32(cx), float32(cy - medio), 0}
	m.Puntos[IndiceMCP] = Punto{float32(cx - medio), float32(cy), 0}
	m.Puntos[MedioMCP] = Punto{float32(cx), float32(cy + medio), 0}
	m.Puntos[AnularMCP] = Punto{float32(cx), float32(cy), 0}
	m.Puntos[MeniqueMCP] = Punto{float32(cx + medio), float32(cy), 0}
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

func TestEmbragueArmadoUsaLaBandaDelPanel(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	casos := []struct {
		nombre string
		x      float64
		dentro bool
	}{
		{"arma en el borde real", 1.00, true},
		{"sigue dentro en la banda", 1.20, true},
		{"sale después de la banda", 1.31, false},
	}
	for i, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if i == 0 {
				// El pellizco se arma tras CuadrosParaApoyar cuadros cerrados.
				for k := 1; k < AjustesEmbraguePorDefecto.CuadrosParaApoyar; k++ {
					med.Cuadro(cuadroPanel(int64(k-10)*33, manoEnPanel(caso.x, 0, .2, true)))
				}
			}
			r := med.Cuadro(cuadroPanel(int64(i)*67, manoEnPanel(caso.x, 0, .2, true)))
			if r.Dentro != caso.dentro {
				t.Fatalf("x %.2f: dentro %v, quería %v", caso.x, r.Dentro, caso.dentro)
			}
			if r.Fase != EmbragueAgarrando || r.ManosArmadas != 1 {
				t.Fatalf("el pellizco armado se perdió antes de la histéresis de soltar: %+v", r)
			}
		})
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

// Sin zoom (23-nov): mientras una mano pellizca, la otra no se arma, aunque
// pellizque y se mueva; y si las dos cierran en el mismo cuadro, solo una.
func TestEmbragueUnSoloDedoALaVez(t *testing.T) {
	for _, juntas := range []bool{false, true} {
		med := NuevoMedidorEmbrague(panelCuadrado(t))
		var r ResultadoEmbrague
		var arrastres []Arrastre
		recoger := func(r ResultadoEmbrague) {
			if r.ManosArmadas > 1 || len(r.Armadas) > 1 {
				t.Fatalf("juntas=%v: dos dedos a la vez: %+v", juntas, r)
			}
			if r.Arrastre != nil {
				arrastres = append(arrastres, *r.Arrastre)
			}
		}
		segunda := juntas
		for i := 0; i < 4; i++ {
			r = med.Cuadro(cuadroPanel(int64(i)*67,
				manoEnPanel(-0.3, 0, .2, true), manoEnPanel(0.3, 0, .2, segunda)))
			recoger(r)
		}
		segunda = true
		for i := 1; i <= 10; i++ {
			sep := 0.3 + float64(i)*0.05
			r = med.Cuadro(cuadroPanel(int64(4+i)*67,
				manoEnPanel(-sep, 0, .2, true), manoEnPanel(sep, 0, .2, segunda)))
			recoger(r)
		}
		if r.Fase != EmbragueAgarrando || r.ManosArmadas != 1 {
			t.Fatalf("juntas=%v: quiere una mano agarrando: %+v", juntas, r)
		}
		for i := 0; i < 6; i++ {
			r = med.Cuadro(cuadroPanel(int64(15+i)*67,
				manoEnPanel(-0.8, 0, .2, false), manoEnPanel(0.8, 0, .2, false)))
			recoger(r)
		}
		if r.ManosArmadas != 0 || len(arrastres) != 1 {
			t.Fatalf("juntas=%v: al soltar, un solo arrastre y nada armado: %+v · %+v", juntas, arrastres, r)
		}
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
	arrastres  []Arrastre
	maxArmadas int
}

func medirToma(g Grabacion, tamano float64) medidaToma {
	med := NuevoMedidorEmbrague(calibracionDeToma(g, tamano))
	var m medidaToma
	for _, c := range g.Cuadros {
		r := med.Cuadro(c)
		if r.Arrastre != nil {
			m.arrastres = append(m.arrastres, *r.Arrastre)
		}
		m.maxArmadas = max(m.maxArmadas, r.ManosArmadas)
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
			// La línea base mide el movimiento físico, antes de la ganancia y
			// saturación de bordes que usa el puntero de pantalla.
			p, _ := cal.mapearCrudo(mu)
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

func TestEmbragueTomasDeZoomSoloUnDedo(t *testing.T) {
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
		if m.maxArmadas > 1 {
			t.Fatalf("%s: dos dedos a la vez (%d manos armadas): ya no hay zoom", g.Archivo, m.maxArmadas)
		}
		t.Logf("%s: %d arrastres con una mano", g.Archivo, len(m.arrastres))
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
		if len(m.arrastres) != 0 {
			t.Fatalf("%s: arrastres %+v", g.Archivo, m.arrastres)
		}
	}
}

// Un cierre corto (el pellizco que parpadea) no apoya el dedo: en las tomas
// reales salía como un tap de 200 ms que nadie pidió.
func TestEmbragueNoArmaConUnCierreCorto(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	cerrado := []bool{false, true, true, false, false, false, false}
	for i, c := range cerrado {
		r := med.Cuadro(cuadroPanel(int64(i)*67, manoEnPanel(0, 0, .2, c)))
		if r.ManosArmadas != 0 {
			t.Fatalf("cuadro %d: armó con un solo cuadro cerrado: %+v", i, r)
		}
	}
	var r ResultadoEmbrague
	for k := 0; k < AjustesEmbraguePorDefecto.CuadrosParaApoyar; k++ {
		r = med.Cuadro(cuadroPanel(500+int64(k)*67, manoEnPanel(0, 0, .2, true)))
	}
	if r.ManosArmadas != 1 {
		t.Fatalf("%d cuadros cerrados dentro deberían armar: %+v", AjustesEmbraguePorDefecto.CuadrosParaApoyar, r)
	}
}

// Dos niveles (cotejo-lgbapm): un pellizco muy cerrado apoya desde el primer
// cuadro, así el tap rápido (uno o dos cuadros, ~130 ms a 15 fps en Windows)
// cuenta; uno tibio sigue esperando CuadrosParaApoyar; y uno que solo se
// acerca sin bajar de UmbralCerrar no apoya nunca.
func TestEmbragueFirmeApoyaAlPrimerCuadro(t *testing.T) {
	aj := AjustesPellizcoPorDefecto
	firme := aj.UmbralFirme - 0.04
	tibio := (aj.UmbralFirme + aj.UmbralCerrar) / 2
	casi := aj.UmbralCerrar + 0.03

	med := NuevoMedidorEmbrague(panelCuadrado(t))
	med.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	if r := med.Cuadro(cuadroPanel(67, manoEnPanelApertura(0, 0, .2, firme))); r.ManosArmadas != 1 {
		t.Fatalf("firme (%.2f) no apoyó al primer cuadro: %+v", firme, r)
	}
	// El tap: se abre enseguida y se levanta al cumplirse el soltar.
	var r ResultadoEmbrague
	for k := int64(2); k < 8; k++ {
		r = med.Cuadro(cuadroPanel(k*67, manoEnPanel(0, 0, .2, false)))
	}
	if r.ManosArmadas != 0 {
		t.Fatalf("el tap no se levantó: %+v", r)
	}

	// Tibio y después firme en el segundo cuadro: apoya en el segundo.
	med = NuevoMedidorEmbrague(panelCuadrado(t))
	med.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	if r := med.Cuadro(cuadroPanel(67, manoEnPanelApertura(0, 0, .2, tibio))); r.ManosArmadas != 0 {
		t.Fatalf("tibio (%.2f) apoyó al primer cuadro: %+v", tibio, r)
	}
	if r := med.Cuadro(cuadroPanel(134, manoEnPanelApertura(0, 0, .2, firme))); r.ManosArmadas != 1 {
		t.Fatalf("tibio y luego firme no apoyó: %+v", r)
	}

	// Tibio sostenido: apoya justo en CuadrosParaApoyar.
	med = NuevoMedidorEmbrague(panelCuadrado(t))
	med.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	n := AjustesEmbraguePorDefecto.CuadrosParaApoyar
	for k := 1; k <= n; k++ {
		r := med.Cuadro(cuadroPanel(int64(k)*67, manoEnPanelApertura(0, 0, .2, tibio)))
		if (r.ManosArmadas == 1) != (k == n) {
			t.Fatalf("tibio, cuadro %d de %d: %+v", k, n, r)
		}
	}

	// Los dedos cerca sin tocarse (por encima de cerrar) no apoyan nunca.
	med = NuevoMedidorEmbrague(panelCuadrado(t))
	for k := int64(0); k < 10; k++ {
		if r := med.Cuadro(cuadroPanel(k*67, manoEnPanelApertura(0, 0, .2, casi))); r.ManosArmadas != 0 {
			t.Fatalf("%.2f, sin tocarse, apoyó: %+v", casi, r)
		}
	}

	// Un firme que empieza fuera del panel tampoco apoya (uy84og).
	med = NuevoMedidorEmbrague(panelCuadrado(t))
	for k := int64(0); k < 5; k++ {
		if r := med.Cuadro(cuadroPanel(k*67, manoEnPanelApertura(1.4, 0, .2, firme))); r.ManosArmadas != 0 {
			t.Fatalf("firme fuera del panel apoyó: %+v", r)
		}
	}
}

// La calibración con el pellizco del dueño cambia los umbrales del embrague.
func TestEmbragueUsaElPellizcoCalibrado(t *testing.T) {
	c := panelCuadrado(t)
	c.PellizcoCerrado = 0.15 // una mano cuyas puntas juntas miden 0,15
	aj := AjustesPellizcoPara(c.PellizcoCerrado)
	med := NuevoMedidorEmbrague(c)
	if med.Ajustes.Pellizco != aj {
		t.Fatalf("ajustes %+v, quería %+v", med.Ajustes.Pellizco, aj)
	}
	// 0,22 no cierra con los de por defecto (0,20) y sí con los de esta mano.
	med.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	var r ResultadoEmbrague
	for k := int64(1); k <= 3; k++ {
		r = med.Cuadro(cuadroPanel(k*67, manoEnPanelApertura(0, 0, .2, 0.22)))
	}
	if r.ManosArmadas != 1 {
		t.Fatalf("con el pellizco calibrado 0,22 debería apoyar: %+v", r)
	}
	def := NuevoMedidorEmbrague(panelCuadrado(t))
	def.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	for k := int64(1); k <= 3; k++ {
		r = def.Cuadro(cuadroPanel(k*67, manoEnPanelApertura(0, 0, .2, 0.22)))
	}
	if r.ManosArmadas != 0 {
		t.Fatalf("sin calibrar, 0,22 no debería apoyar: %+v", r)
	}
}

// Cerrar el pellizco mueve la palma: el dedo cae donde apuntaba la mano
// abierta (el ancla) y el arrastre sigue desde ahí con el mismo recorrido.
func TestEmbragueApoyaEnElAncla(t *testing.T) {
	med := NuevoMedidorEmbrague(panelCuadrado(t))
	abierta := med.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	n := AjustesEmbraguePorDefecto.CuadrosParaApoyar
	for k := 1; k < n; k++ {
		cerrando := med.Cuadro(cuadroPanel(int64(k)*67, manoEnPanel(0.08, 0.04, .2, true)))
		if cerrando.ManosArmadas != 0 || cerrando.X != abierta.X || cerrando.Y != abierta.Y {
			t.Fatalf("cerrando, el puntero debe quedarse en el ancla %.3f,%.3f: %+v", abierta.X, abierta.Y, cerrando)
		}
	}
	r := med.Cuadro(cuadroPanel(int64(n)*67, manoEnPanel(0.08, 0.04, .2, true)))
	if r.ManosArmadas != 1 {
		t.Fatalf("no armó: %+v", r)
	}
	a := r.Armadas[0]
	if math.Abs(a.X-abierta.X) > 1e-9 || math.Abs(a.Y-abierta.Y) > 1e-9 || r.X != a.X || r.Y != a.Y {
		t.Fatalf("apoyó en %.3f,%.3f (puntero %.3f,%.3f), quería el ancla %.3f,%.3f", a.X, a.Y, r.X, r.Y, abierta.X, abierta.Y)
	}
	sin := NuevoMedidorEmbragueCon(panelCuadrado(t), func() AjustesEmbrague { aj := AjustesEmbraguePorDefecto; aj.MsAncla = 0; return aj }())
	sin.Cuadro(cuadroPanel(0, manoEnPanel(0, 0, .2, false)))
	var rs ResultadoEmbrague
	for k := 1; k <= n; k++ {
		rs = sin.Cuadro(cuadroPanel(int64(k)*67, manoEnPanel(0.08, 0.04, .2, true)))
	}
	dx := rs.Armadas[0].X - a.X
	tSig := int64(n+1) * 67
	r2 := med.Cuadro(cuadroPanel(tSig, manoEnPanel(0.28, 0.04, .2, true)))
	rs2 := sin.Cuadro(cuadroPanel(tSig, manoEnPanel(0.28, 0.04, .2, true)))
	if math.Abs((r2.Armadas[0].X-a.X)-(rs2.Armadas[0].X-rs.Armadas[0].X)) > 1e-9 || dx == 0 {
		t.Fatalf("el ancla cambió el recorrido del arrastre o no movió el apoyo: con %+v, sin %+v", r2.Armadas, rs2.Armadas)
	}
}
