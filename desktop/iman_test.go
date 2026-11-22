package desktop

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// Pantalla sintética de 1440×900 píxeles lógicos.
const anchoPrueba, altoPrueba = 1440.0, 900.0

// obj es un objetivo en píxeles lógicos.
func obj(x, y, w, h float64) platform.Target {
	return objRol(platform.TargetRoleButton, x, y, w, h)
}

func objRol(rol platform.TargetRole, x, y, w, h float64) platform.Target {
	return platform.Target{X: x / anchoPrueba, Y: y / altoPrueba, Width: w / anchoPrueba, Height: h / altoPrueba, Role: rol}
}

func conjuntoPrueba(ventana uint64, ts ...platform.Target) platform.TargetSet {
	return platform.TargetSet{
		Window: ventana, Targets: ts, LogicalWidth: anchoPrueba, LogicalHeight: altoPrueba,
		WindowBounds: platform.TargetBounds{Width: 1, Height: 1},
	}
}

// px pasa píxeles lógicos a un punto normalizado.
func px(x, y float64) gestures.PuntoPanel {
	return gestures.PuntoPanel{X: x / anchoPrueba, Y: y / altoPrueba}
}

func enPx(p gestures.PuntoPanel) (float64, float64) { return p.X * anchoPrueba, p.Y * altoPrueba }

func mismoPunto(a, b gestures.PuntoPanel) bool {
	ax, ay := enPx(a)
	bx, by := enPx(b)
	return math.Hypot(ax-bx, ay-by) < .01
}

func TestAjustarToqueConObjetivosSinteticos(t *testing.T) {
	o := NuevosObjetivosIman(conjuntoPrueba(1,
		obj(100, 100, 24, 24),   // botón chico solo
		obj(300, 100, 24, 24),   // dos botones pegados: 300-324 y 330-354
		obj(330, 100, 24, 24),   //
		obj(500, 100, 300, 20),  // enlace largo de una línea
		obj(100, 400, 400, 200), // tarjeta grande que la accesibilidad llama botón
		obj(100, 100, 24, 24),   // repetido (un enlace dentro del botón)
	))
	if o.CantObjetivo != 5 || o.CantChicos != 4 {
		t.Fatalf("objetivos %d (chicos %d), quería 5 (4): el repetido se junta y la tarjeta no es chica", o.CantObjetivo, o.CantChicos)
	}
	casos := []struct {
		nombre string
		toque  gestures.PuntoPanel
		quiero gestures.PuntoPanel
		res    ResultadoAjuste
	}{
		{"dentro de un botón no se mueve", px(105, 103), px(105, 103), AjusteDentro},
		{"a 10 px va al centro", px(134, 112), px(112, 112), AjusteMovido},
		{"en diagonal a 12 px va al centro", px(92, 92), px(112, 112), AjusteMovido},
		{"a 30 px no lo alcanza", px(154, 112), px(154, 112), AjusteNada},
		{"entre dos botones parecidos: duda", px(327, 90), px(327, 90), AjusteDuda},
		{"más cerca de uno pero sin margen: duda", px(326, 112), px(326, 112), AjusteDuda},
		{"a 1 px de uno y 5 del otro: duda", px(325, 112), px(325, 112), AjusteDuda},
		{"claro hacia uno de los dos", px(290, 112), px(312, 112), AjusteMovido},
		{"enlace largo: el x de donde tocó, el y al centro", px(600, 130), px(600, 110), AjusteMovido},
		{"enlace largo, por la punta: a 12 px del borde", px(810, 110), px(788, 110), AjusteMovido},
		{"dentro de la tarjeta grande: no se mueve", px(110, 410), px(110, 410), AjusteDentro},
		{"cerca de la tarjeta grande: no atrae", px(90, 390), px(90, 390), AjusteNada},
	}
	for _, c := range casos {
		got, res := o.AjustarToque(c.toque)
		if res != c.res || !mismoPunto(got, c.quiero) {
			gx, gy := enPx(got)
			t.Errorf("%s: (%.1f, %.1f) %q; quería %q", c.nombre, gx, gy, res, c.res)
		}
	}
	var nada *ObjetivosIman
	if got, res := nada.AjustarToque(px(134, 112)); res != AjusteSinWeb || !mismoPunto(got, px(134, 112)) {
		t.Fatalf("sin lista movió el toque: %v %q", got, res)
	}
}

func TestImanExcluyeMarcoRolesNoContenidoYObjetivosPegadosAlBorde(t *testing.T) {
	o := NuevosObjetivosIman(conjuntoPrueba(1,
		obj(0, 200, 24, 24),                                    // borde izquierdo
		obj(700, 0, 24, 24),                                    // borde superior
		obj(anchoPrueba-24, 300, 24, 24),                       // borde derecho
		obj(900, altoPrueba-24, 24, 24),                        // borde inferior
		objRol(platform.TargetRoleFrame, 200, 200, 24, 24),     // marco
		objRol(platform.TargetRoleScrollBar, 250, 200, 24, 24), // barra
		objRol(platform.TargetRoleSeparator, 300, 200, 24, 24), // separador
		objRol(platform.TargetRoleTab, 350, 200, 80, 24),       // pestaña de la app
		objRol(platform.TargetRoleLink, 500, 300, 80, 24),      // contenido real
	))
	if o.CantObjetivo != 1 || o.CantChicos != 1 {
		t.Fatalf("objetivos %d (chicos %d), quería solo el enlace de contenido", o.CantObjetivo, o.CantChicos)
	}
	if got, res := o.AjustarToque(px(30, 212)); res != AjusteNada || !mismoPunto(got, px(30, 212)) {
		t.Fatalf("el borde izquierdo atrajo el toque: %v, %q", got, res)
	}
	if got, res := o.AjustarToque(px(590, 312)); res != AjusteMovido || !mismoPunto(got, px(568, 312)) {
		t.Fatalf("el enlace interior no atrajo: %v, %q", got, res)
	}
	var f FrenoIman
	f.Aplicar(o, px(5, 212))
	if got := f.Aplicar(o, px(15, 212)); !mismoPunto(got, px(15, 212)) {
		t.Fatalf("el borde frenó el puntero: %v", got)
	}
}

func TestFrenoImanVaMasLentoSobreUnBotonYVuelveConLaMano(t *testing.T) {
	o := NuevosObjetivosIman(conjuntoPrueba(1, obj(100, 100, 40, 24)))
	var f FrenoIman
	f.Aplicar(o, px(105, 110))
	got := f.Aplicar(o, px(115, 110)) // la mano avanza 10 px encima del botón
	if x, _ := enPx(got); math.Abs(x-110) > .01 {
		t.Fatalf("encima del botón avanzó a %.2f; quería 110 (la mitad)", x)
	}
	// La mano sigue: el puntero nunca se separa más de RadioIman.
	for x := 125.0; x <= 400; x += 10 {
		got = f.Aplicar(o, px(x, 110))
		gx, _ := enPx(got)
		if x-gx > RadioIman+.01 {
			t.Fatalf("con la mano en %.0f el puntero quedó en %.2f", x, gx)
		}
	}
	if gx, _ := enPx(got); math.Abs(gx-395) > 1 {
		t.Fatalf("fuera de los botones el puntero no volvió a la mano: %.2f", gx)
	}
	// Sin lista vigente, va con la mano.
	if got := f.Aplicar(nil, px(500, 500)); !mismoPunto(got, px(500, 500)) {
		t.Fatalf("sin lista frenó: %v", got)
	}
}

func TestRegistroImanDiceRolYTamanoSinRepetirElFreno(t *testing.T) {
	p, _, _, _ := panelConIman(t, objRol(platform.TargetRoleLink, 100, 100, 40, 24))
	ruta := filepath.Join(t.TempDir(), "iman.jsonl")
	reg, err := AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	p.Reg = reg
	o := p.objetivosIman()
	p.freno.aplicar(o, px(105, 110))
	for _, x := range []float64{110, 115, 120} {
		_, objetivo := p.freno.aplicar(o, px(x, 110))
		if objetivo != nil {
			p.anotarObjetivoIman("iman_frena", objetivo)
		}
	}
	p.ajustarIman(px(150, 112))
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	ls := lineas(t, ruta)
	if len(ls) != 2 {
		t.Fatalf("registro = %+v, quería un freno y un ajuste", ls)
	}
	for _, l := range ls {
		if l.Tipo != TipoPanel || l.Rol != string(platform.TargetRoleLink) || l.Ancho != 40 || l.Alto != 24 {
			t.Errorf("detalle incompleto: %+v", l)
		}
	}
	if ls[0].Resultado != "iman_frena" || ls[1].Resultado != "iman_ajusta" {
		t.Fatalf("resultados = %q, %q", ls[0].Resultado, ls[1].Resultado)
	}
}

// buscadorPrueba es un TargetFinder con la ventana y la lista que se quiera.
type buscadorPrueba struct {
	ventana  uint64
	conjunto platform.TargetSet
	err      error
	lecturas int
}

func (b *buscadorPrueba) FocusedWindow(context.Context) (uint64, error) { return b.ventana, nil }
func (b *buscadorPrueba) Targets(context.Context) (platform.TargetSet, error) {
	b.lecturas++
	s := b.conjunto
	s.Window = b.ventana
	return s, b.err
}

func TestImanCacheaPorVentanaYNoPorCuadro(t *testing.T) {
	b := &buscadorPrueba{ventana: 7, conjunto: conjuntoPrueba(0, obj(100, 100, 24, 24))}
	m := NuevoIman(b)
	ctx := context.Background()
	t0 := time.Unix(1000, 0)

	m.Paso(ctx, t0)
	if b.lecturas != 0 {
		t.Fatal("leyó sin que el panel lo pidiera (mano fuera)")
	}
	m.Pedir(true)
	for i := range 6 { // 6 vueltas de 250 ms: 1,25 s
		m.Paso(ctx, t0.Add(time.Duration(i)*ConsultarVentanaCada))
	}
	if b.lecturas != 1 {
		t.Fatalf("misma ventana: %d lecturas, quería 1", b.lecturas)
	}
	if m.Objetivos(t0.Add(time.Second)) == nil {
		t.Fatal("la lista recién leída no vale")
	}
	// Cambia la ventana con el foco: lo leído ya no vale y se vuelve a leer.
	b.ventana = 8
	t1 := t0.Add(1300 * time.Millisecond)
	m.Paso(ctx, t1)
	if b.lecturas != 2 || m.Objetivos(t1) == nil || m.Objetivos(t1).Ventana != 8 {
		t.Fatalf("al cambiar de ventana: %d lecturas", b.lecturas)
	}
	// Levantar el dedo invalida la lista y espera a que la página se asiente.
	t2 := t1.Add(100 * time.Millisecond)
	m.Cambio(t2)
	if m.Objetivos(t2) != nil {
		t.Fatal("la lista de antes del toque sigue valiendo")
	}
	m.Paso(ctx, t2.Add(ConsultarVentanaCada))
	if b.lecturas != 2 {
		t.Fatal("leyó mientras la página se movía")
	}
	m.Paso(ctx, t2.Add(2*ConsultarVentanaCada))
	if b.lecturas != 3 || m.Objetivos(t2.Add(2*ConsultarVentanaCada)) == nil {
		t.Fatalf("no volvió a leer tras la espera: %d", b.lecturas)
	}
	// Vieja: no corrige.
	if m.Objetivos(t2.Add(2*ConsultarVentanaCada+VigenciaIman+time.Millisecond)) != nil {
		t.Fatal("una lista vieja sigue valiendo")
	}
	// Apagado: nada.
	m.Encender(false)
	m.Paso(ctx, t2.Add(5*time.Second))
	if b.lecturas != 3 || m.Objetivos(t2.Add(5*time.Second)) != nil || m.Encendido() {
		t.Fatal("apagado siguió leyendo o corrigiendo")
	}
	// Un error de la accesibilidad deja sin lista, no con la vieja.
	m.Encender(true)
	b.err = errors.New("sin UIA")
	m.Paso(ctx, t2.Add(6*time.Second))
	if m.Objetivos(t2.Add(6*time.Second)) != nil || m.Estado(t2).Fallos != 1 {
		t.Fatal("con error quedó una lista")
	}
}

// panelConIman arma un panel calibrado con táctil y un imán ya leído sobre
// la lista dada, con el reloj fijo en ahora.
func panelConIman(t *testing.T, ts ...platform.Target) (*Panel, *tactilPanelPrueba, *Iman, *time.Time) {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, _ := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	tactil := &tactilPanelPrueba{}
	p, _ := NuevoPanel(ruta, &punteroPanelPrueba{})
	p.Tactil = tactil
	if err := p.Preparar(); err != nil {
		t.Fatal(err)
	}
	b := &buscadorPrueba{ventana: 1, conjunto: conjuntoPrueba(0, ts...)}
	m := NuevoIman(b)
	ahora := time.Unix(2000, 0)
	m.Pedir(true)
	m.Paso(context.Background(), ahora)
	p.Iman = m
	p.ahora = func() time.Time { return ahora }
	return p, tactil, m, &ahora
}

// tocar pasa unos cuadros con la mano quieta en q, apoya el dedo unos
// cuadros moviéndolo mover px a la derecha y lo levanta. Devuelve los
// cuadros táctiles de ese toque.
func tocar(p *Panel, tactil *tactilPanelPrueba, t0 int64, q gestures.PuntoPanel, mover float64) [][]platform.TouchPoint {
	clave := gestures.ClaveMano{}
	desde := len(tactil.frames)
	paso := func(i int64, fase gestures.FaseEmbrague, pos gestures.PuntoPanel) {
		r := gestures.ResultadoEmbrague{TMs: t0 + i*33, Fase: fase, Dentro: true, TienePos: true, X: pos.X, Y: pos.Y}
		if fase == gestures.EmbragueAgarrando {
			r.ManosArmadas = 1
			r.Armadas = []gestures.ManoArmada{{Clave: clave, X: pos.X, Y: pos.Y}}
		}
		p.mu.Lock()
		p.aplicar(r, r.TMs, true)
		p.mu.Unlock()
	}
	// La mano llega de fuera del panel: el filtro de la palma empieza en q.
	p.mu.Lock()
	p.aplicar(gestures.ResultadoEmbrague{TMs: t0 - 33, Fase: gestures.EmbragueFuera}, t0-33, true)
	p.mu.Unlock()
	for i := range int64(5) {
		paso(i, gestures.EmbragueDentro, q)
	}
	for i := range int64(4) {
		paso(5+i, gestures.EmbragueAgarrando, gestures.PuntoPanel{X: q.X + float64(i)*mover/3/anchoPrueba, Y: q.Y})
	}
	paso(9, gestures.EmbragueDentro, q)
	return tactil.frames[desde:]
}

func TestPanelConImanLlevaElToqueAlBotonSinVolverloArrastre(t *testing.T) {
	p, tactil, m, ahora := panelConIman(t, obj(700, 440, 24, 24))
	fs := tocar(p, tactil, 0, px(735, 452), 0) // 11 px a la derecha del botón
	if len(fs) != 5 || !fs[0][0].Down || fs[4][0].Down {
		t.Fatalf("cuadros: %+v", fs)
	}
	for i, f := range fs {
		if !mismoPunto(gestures.PuntoPanel{X: f[0].X, Y: f[0].Y}, px(712, 452)) {
			x, y := enPx(gestures.PuntoPanel{X: f[0].X, Y: f[0].Y})
			t.Fatalf("cuadro %d en (%.1f, %.1f): quería el centro del botón y quieto", i, x, y)
		}
	}
	if e := m.Estado(*ahora); e.Ajustes != 1 {
		t.Fatalf("estado: %+v", e)
	}
	// El arrastre que sigue a un ajuste va entero con la mano, desplazado.
	*ahora = ahora.Add(2 * time.Second)
	m.Paso(context.Background(), *ahora)
	fs = tocar(p, tactil, 1000, px(735, 452), 60)
	// El mismo arrastre sin imán: cada cuadro tiene que avanzar lo mismo.
	sinIman, tactilSin, _, _ := panelConIman(t)
	sinIman.Iman = nil
	ref := tocar(sinIman, tactilSin, 1000, px(735, 452), 60)
	if len(ref) != len(fs) {
		t.Fatalf("cuadros con imán %d, sin imán %d", len(fs), len(ref))
	}
	x0, _ := enPx(gestures.PuntoPanel{X: fs[0][0].X})
	r0, _ := enPx(gestures.PuntoPanel{X: ref[0][0].X})
	if math.Abs(x0-712) > .01 || math.Abs(r0-735) > .01 {
		t.Fatalf("arrastre: con imán empezó en %.1f (quería 712), sin imán en %.1f (735)", x0, r0)
	}
	for i := range fs {
		xi, _ := enPx(gestures.PuntoPanel{X: fs[i][0].X})
		ri, _ := enPx(gestures.PuntoPanel{X: ref[i][0].X})
		if math.Abs((xi-x0)-(ri-r0)) > .01 {
			t.Fatalf("cuadro %d: con imán avanzó %.2f px, sin imán %.2f", i, xi-x0, ri-r0)
		}
	}
	if xN, _ := enPx(gestures.PuntoPanel{X: fs[3][0].X}); xN <= x0 {
		t.Fatalf("el arrastre no avanzó: %.1f → %.1f", x0, xN)
	}
}

func TestPanelConImanNoCorrigeConDudaNiApagado(t *testing.T) {
	p, tactil, m, ahora := panelConIman(t, obj(700, 440, 24, 24), obj(730, 440, 24, 24))
	fs := tocar(p, tactil, 0, px(727, 430), 0) // entre los dos, por arriba
	if got := (gestures.PuntoPanel{X: fs[0][0].X, Y: fs[0][0].Y}); !mismoPunto(got, px(727, 430)) {
		x, y := enPx(got)
		t.Fatalf("con duda movió el toque a (%.1f, %.1f)", x, y)
	}
	if e := m.Estado(*ahora); e.Dudas != 1 || e.Ajustes != 0 {
		t.Fatalf("estado: %+v", e)
	}
	// Apagado: el toque cae donde está la mano, igual que sin imán.
	m.Encender(false)
	*ahora = ahora.Add(2 * time.Second)
	fs = tocar(p, tactil, 1000, px(690, 452), 0)
	if got := (gestures.PuntoPanel{X: fs[0][0].X, Y: fs[0][0].Y}); !mismoPunto(got, px(690, 452)) {
		t.Fatal("apagado corrigió el toque")
	}
}

func TestPanelConImanRepiteElAjusteEnElDobleToque(t *testing.T) {
	p, tactil, _, ahora := panelConIman(t, obj(700, 440, 24, 24))
	primero := tocar(p, tactil, 0, px(735, 452), 0)
	// 300 ms después, la lista ya no vale (se acaba de levantar el dedo),
	// pero el segundo toque cae en el mismo sitio que el primero.
	*ahora = ahora.Add(300 * time.Millisecond)
	segundo := tocar(p, tactil, 400, px(737, 455), 0)
	a := gestures.PuntoPanel{X: primero[0][0].X, Y: primero[0][0].Y}
	b := gestures.PuntoPanel{X: segundo[0][0].X, Y: segundo[0][0].Y}
	if !mismoPunto(a, b) || !mismoPunto(a, px(712, 452)) {
		t.Fatalf("doble toque en sitios distintos: %v y %v", a, b)
	}
}

// TestImanNoTocaObjetivosQueNoSeQuerian simula toques con error de puntería
// sobre páginas sintéticas de botones chicos: el imán sube los aciertos y
// no agrega ni un toque en un objetivo distinto del que se quería.
func TestImanNoTocaObjetivosQueNoSeQuerian(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	var sin, con, malSin, malCon, dudosos, total int
	for pagina := range 200 {
		var ts []platform.Target
		var rects []rectIman
		// Una fila de botones chicos con huecos de 4 a 40 px y una columna
		// de enlaces de una línea, como una barra de herramientas y un menú.
		x := 50.0
		for x < 1300 {
			w := 16 + r.Float64()*24
			ts = append(ts, obj(x, 100, w, 24))
			rects = append(rects, rectIman{x0: x, y0: 100, x1: x + w, y1: 124})
			x += w + 4 + r.Float64()*36
		}
		y := 200.0
		for y < 800 {
			h := 16 + r.Float64()*8
			w := 60 + r.Float64()*200
			ts = append(ts, obj(400, y, w, h))
			rects = append(rects, rectIman{x0: 400, y0: y, x1: 400 + w, y1: y + h})
			y += h + 2 + r.Float64()*20
		}
		o := NuevosObjetivosIman(conjuntoPrueba(uint64(pagina), ts...))
		donde := func(x, y float64) int {
			for i, q := range rects {
				if q.contiene(x, y) {
					return i
				}
			}
			return -1
		}
		for range 50 {
			i := r.IntN(len(rects))
			q := rects[i]
			// Se apunta a un punto del objetivo y la mano yerra con 8 px de
			// desviación por eje (el temblor medido tras el filtro).
			ax := q.x0 + r.Float64()*(q.x1-q.x0)
			ay := q.y0 + r.Float64()*(q.y1-q.y0)
			tx, ty := ax+r.NormFloat64()*8, ay+r.NormFloat64()*8
			total++
			switch donde(tx, ty) {
			case i:
				sin++
			case -1:
			default:
				malSin++
			}
			p, _ := o.AjustarToque(px(tx, ty))
			cx, cy := enPx(p)
			switch j := donde(cx, cy); j {
			case i:
				con++
			case -1:
			default:
				malCon++
				if donde(tx, ty) < 0 && rects[j].distancia(tx, ty)+MargenDudaIman > q.distancia(tx, ty) {
					dudosos++
				}
			}
		}
	}
	t.Logf("%d toques: aciertos sin imán %.1f %%, con imán %.1f %%; en otro objetivo sin imán %.1f %%, con imán %.1f %%",
		total, 100*float64(sin)/float64(total), 100*float64(con)/float64(total),
		100*float64(malSin)/float64(total), 100*float64(malCon)/float64(total))
	if con-sin < total*15/100 {
		t.Fatalf("el imán subió poco los aciertos: %d → %d", sin, con)
	}
	// El imán no mueve un toque que ya cae en algo, así que los que caían
	// en otro objetivo siguen ahí. Los que agrega son dedos que cayeron
	// pegados a otro objetivo (a ≥ MargenDudaIman más cerca de él que del
	// querido, nunca a la par): ninguna corrección sabe que se quería el
	// otro. Se exige que sean pocos.
	if agregados := malCon - malSin; agregados*1000 > total*3 {
		t.Fatalf("el imán agregó %d toques en objetivos que no se querían (de %d): más de 0,3 %%", agregados, total)
	}
	if dudosos > 0 {
		t.Fatalf("%d toques fueron a otro objetivo sin estar claramente más cerca de él", dudosos)
	}
}

// BenchmarkImanPorCuadro mide lo que el imán le suma a cada cuadro de la
// cámara con una página de 400 objetivos: el freno del puntero y, al apoyar,
// el ajuste.
func BenchmarkImanPorCuadro(b *testing.B) {
	r := rand.New(rand.NewPCG(3, 4))
	ts := make([]platform.Target, 400)
	for i := range ts {
		ts[i] = obj(r.Float64()*1400, r.Float64()*880, 16+r.Float64()*60, 16+r.Float64()*16)
	}
	o := NuevosObjetivosIman(conjuntoPrueba(1, ts...))
	var f FrenoIman
	b.ResetTimer()
	for i := range b.N {
		q := px(float64(i%1440), float64(i%900))
		f.Aplicar(o, q)
		o.AjustarToque(q)
	}
}
