package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rafael088/lazul/desktop"
	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

type flujoPrueba struct {
	prueba bool
	modo   string
	ultimo gestures.Cuadro
}

func (f *flujoPrueba) Cuadro(c gestures.Cuadro, _ time.Time) desktop.Vista {
	f.ultimo = c
	return desktop.Vista{Modo: f.modo}
}
func (*flujoPrueba) Parar(time.Time)                     {}
func (*flujoPrueba) Prueba(bool, time.Time)              {}
func (f *flujoPrueba) EnPrueba() bool                    { return f.prueba }
func (*flujoPrueba) Marcar(bool, time.Time) (int, error) { return 0, nil }
func (*flujoPrueba) Sesion() string                      { return "sesion-prueba" }

func TestRegistrarEvitadasCuentaEpisodiosNoCuadros(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "registro.jsonl")
	reg, err := desktop.AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{reg: reg, flujo: &flujoPrueba{prueba: true}}
	ahora := time.Now()
	a.registrarEvitadas(map[string]int{"fuera_torso": 1}, ahora)
	a.registrarEvitadas(map[string]int{"fuera_torso": 1}, ahora.Add(time.Millisecond))
	a.registrarEvitadas(nil, ahora.Add(2*time.Millisecond))
	a.registrarEvitadas(map[string]int{"fuera_torso": 1}, ahora.Add(3*time.Millisecond))
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	ls, err := desktop.LeerRegistroDe(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 || ls[0].Tipo != desktop.TipoEvitada || ls[0].Resultado != "fuera_torso" || !ls[0].Prueba {
		t.Fatalf("líneas: %+v", ls)
	}
	if a.evitadas["fuera_torso"] != 2 {
		t.Fatalf("contador: %+v", a.evitadas)
	}
}

func TestRegistrarEvitadasAnotaElMotivoDeCadaSalida(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "registro.jsonl")
	reg, err := desktop.AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{reg: reg, flujo: &flujoPrueba{}}
	ahora := time.Now()
	rechazos := map[string]int{
		claveRechazo("fuera_panel", desktop.MotivoFueraBorde): 1,
		claveRechazo("fuera_panel", desktop.MotivoLejos):      1,
	}
	a.registrarEvitadas(rechazos, ahora)
	a.registrarEvitadas(rechazos, ahora.Add(time.Millisecond)) // el mismo episodio no se repite
	a.registrarEvitadas(map[string]int{claveRechazo("fuera_panel", desktop.MotivoLejos): 1}, ahora.Add(2*time.Millisecond))
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	ls, err := desktop.LeerRegistroDe(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 {
		t.Fatalf("líneas: %+v", ls)
	}
	motivos := map[string]bool{}
	for _, l := range ls {
		if l.Tipo != desktop.TipoEvitada || l.Resultado != "fuera_panel" {
			t.Fatalf("línea: %+v", l)
		}
		motivos[l.Motivo] = true
	}
	if !motivos[desktop.MotivoFueraBorde] || !motivos[desktop.MotivoLejos] {
		t.Fatalf("motivos: %+v", ls)
	}
	// El motivo dice por qué, nunca dónde: ni coordenadas ni nada de la cámara.
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"x":`) || strings.Contains(string(b), `"y":`) || strings.Contains(string(b), `"p":`) {
		t.Fatalf("el registro contiene una posición: %s", b)
	}
}

// manoEnImagen pone la palma de una mano de prueba centrada en (x, y) de la
// imagen, con un tamaño aparente (fracción del ancho) que el panel mide.
func manoEnImagen(x, y, tam float32) gestures.Mano {
	var m gestures.Mano
	medio := tam / math.Sqrt2 / 2
	m.Puntos[gestures.Muneca] = gestures.Punto{X: x, Y: y - medio}
	m.Puntos[gestures.IndiceMCP] = gestures.Punto{X: x - medio, Y: y}
	m.Puntos[gestures.MedioMCP] = gestures.Punto{X: x, Y: y + medio}
	m.Puntos[gestures.AnularMCP] = gestures.Punto{X: x, Y: y}
	m.Puntos[gestures.MeniqueMCP] = gestures.Punto{X: x + medio, Y: y}
	return m
}

// El cuadro completo: una mano fuera del panel deja en el registro el motivo,
// no solo que quedó fuera.
func TestCuadroAnotaElMotivoDeLaSalidaDelPanel(t *testing.T) {
	dir := t.TempDir()
	cal, err := gestures.NuevaCalibracionPanel(
		gestures.MuestraPanel{X: -.2, Y: -.2, Tamano: .2},
		gestures.MuestraPanel{X: .2, Y: .2, Tamano: .2})
	if err != nil {
		t.Fatal(err)
	}
	if err := desktop.GuardarCalibracionPanel(filepath.Join(dir, desktop.NombreCalibracionPanel), cal); err != nil {
		t.Fatal(err)
	}
	panel, err := desktop.NuevoPanel(filepath.Join(dir, desktop.NombreCalibracionPanel), nil)
	if err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(dir, "registro.jsonl")
	reg, err := desktop.AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{cfg: Config{Modo: "tactil", Reproducir: true}, flujo: &flujoPrueba{}, panel: panel,
		ind: &desktop.Indicador{}, reg: reg}
	// El panel calibrado ocupa x e y de .3 a .7 de la imagen.
	a.cuadro(gestures.Cuadro{TMs: 1, Aspecto: 1, Manos: []gestures.Mano{manoEnImagen(.8, .5, .2529)}})
	a.cuadro(gestures.Cuadro{TMs: 2, Aspecto: 1, Manos: []gestures.Mano{manoEnImagen(.5, .5, .3794)}})
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	ls, err := desktop.LeerRegistroDe(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var motivos []string
	for _, l := range ls {
		if l.Tipo == desktop.TipoEvitada && l.Resultado == "fuera_panel" {
			motivos = append(motivos, l.Motivo)
		}
	}
	if len(motivos) != 2 || motivos[0] != desktop.MotivoFueraBorde || motivos[1] != desktop.MotivoCerca {
		t.Fatalf("motivos anotados: %v (líneas %+v)", motivos, ls)
	}
}

func TestCalibracionVeLaManoPeroNoActivaElFlujo(t *testing.T) {
	panel, err := desktop.NuevoPanel(filepath.Join(t.TempDir(), desktop.NombreCalibracionPanel), nil)
	if err != nil {
		t.Fatal(err)
	}
	panel.EmpezarCalibracion()
	flujo := &flujoPrueba{}
	a := &App{cfg: Config{Modo: "tactil"}, flujo: flujo, panel: panel, ind: &desktop.Indicador{}}

	var cuerpo gestures.Cuerpo
	for _, p := range []struct {
		i    int
		x, y float32
	}{
		{gestures.HombroIzquierdo, .3, .2}, {gestures.HombroDerecho, .7, .2},
		{gestures.CaderaIzquierda, .35, .8}, {gestures.CaderaDerecha, .65, .8},
	} {
		cuerpo.Puntos[p.i] = gestures.PuntoCuerpo{X: p.x, Y: p.y, Visibilidad: 1}
	}
	var mano gestures.Mano
	mano.Puntos[gestures.Muneca] = gestures.Punto{X: .5, Y: .55}
	mano.Puntos[gestures.IndiceMCP] = gestures.Punto{X: .45, Y: .45}
	mano.Puntos[gestures.MedioMCP] = gestures.Punto{X: .5, Y: .45}
	mano.Puntos[gestures.AnularMCP] = gestures.Punto{X: .55, Y: .45}
	mano.Puntos[gestures.MeniqueMCP] = gestures.Punto{X: .6, Y: .48}
	a.cuadro(gestures.Cuadro{TMs: 1, Aspecto: 1, Cuerpo: &cuerpo, Manos: []gestures.Mano{mano}})
	if len(flujo.ultimo.Manos) != 0 {
		t.Fatalf("el flujo recibió la mano durante la calibración: %+v", flujo.ultimo.Manos)
	}
}

func TestAuraDeduplicaLimitaMovimientoYPreservaEstados(t *testing.T) {
	t0 := time.Unix(100, 0)
	dentro := EstadoAura{Activa: true, Calibrado: true, Dentro: true, X: .4, Y: .5}
	if !debeActualizarAura(EstadoAura{}, dentro, false, time.Time{}, t0) {
		t.Fatal("el primer estado debe llegar")
	}
	if debeActualizarAura(dentro, dentro, true, t0, t0.Add(time.Second)) {
		t.Fatal("un estado idéntico no debe cruzar a la ventana")
	}
	movido := dentro
	movido.X = .41
	if debeActualizarAura(dentro, movido, true, t0, t0.Add(intervaloAura-time.Millisecond)) {
		t.Fatal("el movimiento debe quedar limitado a 25 Hz")
	}
	if !debeActualizarAura(dentro, movido, true, t0, t0.Add(intervaloAura)) {
		t.Fatal("el movimiento debe pasar al vencer el intervalo")
	}
	agarrando := movido
	agarrando.Agarrando = true
	if !debeActualizarAura(dentro, agarrando, true, t0, t0.Add(time.Millisecond)) {
		t.Fatal("un cambio visual debe pasar sin esperar")
	}
	fuera := agarrando
	fuera.Dentro = false
	if !debeActualizarAura(agarrando, fuera, true, t0, t0.Add(time.Millisecond)) {
		t.Fatal("ocultar el aura debe pasar sin esperar")
	}
}

// espaciosCuenta anota cada cambio de escritorio o app.
type espaciosCuenta struct {
	mu   sync.Mutex
	dirs []platform.NavigationDirection
}

func (e *espaciosCuenta) Switch(_ context.Context, d platform.NavigationDirection) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dirs = append(e.dirs, d)
	return nil
}

func (e *espaciosCuenta) n() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.dirs)
}

var (
	manoAbiertaUna  sync.Once
	manoAbiertaGuar gestures.Mano
	manoAbiertaAsp  float32
)

// manoAbierta saca una mano abierta real de las grabaciones.
func manoAbierta(t *testing.T) (gestures.Mano, float32) {
	t.Helper()
	manoAbiertaUna.Do(func() { manoAbiertaGuar, manoAbiertaAsp = buscarManoAbierta(t) })
	return manoAbiertaGuar, manoAbiertaAsp
}

func buscarManoAbierta(t *testing.T) (gestures.Mano, float32) {
	t.Helper()
	gs, err := gestures.LeerDirectorio("../../fixtures/gestos")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gs {
		for _, c := range g.Cuadros {
			if len(c.Manos) == 1 && gestures.PoseDe(c.Manos[0], c.Aspecto) == gestures.PoseAbierta {
				return c.Manos[0], c.Aspecto
			}
		}
	}
	t.Fatal("ninguna mano abierta")
	return gestures.Mano{}, 0
}

// manoTres pasa la mano abierta real a la pose de tres dedos: dobla el
// meñique hacia la muñeca y cruza el pulgar sobre la palma.
func manoTres(t *testing.T) (gestures.Mano, float32) {
	t.Helper()
	m, asp := manoAbierta(t)
	mu := m.Puntos[gestures.Muneca]
	mcp := m.Puntos[gestures.MeniqueMCP]
	for k, f := range map[int]float32{18: 0.15, 19: 0.30, gestures.MeniquePunta: 0.40} {
		m.Puntos[k].X = mcp.X + f*(mu.X-mcp.X)
		m.Puntos[k].Y = mcp.Y + f*(mu.Y-mcp.Y)
	}
	var cx, cy float32
	for _, k := range []int{gestures.Muneca, gestures.IndiceMCP, gestures.MedioMCP, gestures.AnularMCP, gestures.MeniqueMCP} {
		cx += m.Puntos[k].X / 5
		cy += m.Puntos[k].Y / 5
	}
	m.Puntos[gestures.PulgarPunta].X, m.Puntos[gestures.PulgarPunta].Y = cx, cy
	m.Puntos[3].X = (m.Puntos[gestures.PulgarMCP].X + cx) / 2
	m.Puntos[3].Y = (m.Puntos[gestures.PulgarMCP].Y + cy) / 2
	if !gestures.TresDedos(m, asp) {
		t.Fatal("la mano de prueba no queda en la pose de tres dedos")
	}
	return m, asp
}

// tres arma el gesto a ~15 fps: la pose quieta 200 ms, deslizarla 1,5
// palmas en 660 ms hacia un lado (la derecha de la persona es la X menor:
// la imagen no se espeja) y 330 ms sin mano, que terminan el intento.
func tres(t *testing.T, t0 int64, derecha bool) []gestures.Cuadro {
	t.Helper()
	m, asp := manoTres(t)
	a, b := m.Puntos[gestures.Muneca], m.Puntos[gestures.MedioMCP]
	palma := math.Hypot(float64(b.X-a.X)*float64(asp), float64(b.Y-a.Y)) / float64(asp)
	dx := float32(1.5 * palma)
	if derecha {
		dx = -dx
	}
	var cs []gestures.Cuadro
	tm := t0
	agregar := func(mano *gestures.Mano) {
		c := gestures.Cuadro{TMs: tm, Aspecto: asp}
		if mano != nil {
			c.Manos = []gestures.Mano{*mano}
		}
		cs = append(cs, c)
		tm += 66
	}
	for range 3 {
		agregar(&m)
	}
	for i := 1; i <= 10; i++ {
		mano := m
		for k := range mano.Puntos {
			mano.Puntos[k].X += dx * float32(i) / 10
		}
		agregar(&mano)
	}
	for range 5 {
		agregar(nil)
	}
	return cs
}

// alimentar pasa el gesto por la app y comprueba que la mano de tres dedos
// no llega al clasificador (el flujo) en ningún cuadro.
func alimentar(t *testing.T, a *App, flujo *flujoPrueba, cs []gestures.Cuadro) {
	t.Helper()
	for _, c := range cs {
		a.cuadro(c)
		for _, m := range flujo.ultimo.Manos {
			if gestures.TresDedos(m, flujo.ultimo.Aspecto) {
				t.Fatalf("a los %d ms la mano de tres dedos llegó al clasificador", c.TMs)
			}
		}
	}
}

// esperarEspacios espera a que el trabajo asíncrono de Pantallas llegue a n.
func esperarEspacios(e *espaciosCuenta, n int) bool {
	limite := time.Now().Add(time.Second)
	for time.Now().Before(limite) {
		if e.n() >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// Con el panel apagado, los tres dedos no cambian de pantalla ni de
// escritorio; encendido, sí; y al apagarlo otra vez, no. El intento queda
// en el registro con motivo apagado, distinto de ocupado.
func TestTresDedosRespetanElInterruptor(t *testing.T) {
	dir := t.TempDir()
	cal, err := gestures.NuevaCalibracionPanel(
		gestures.MuestraPanel{X: -.2, Y: -.2, Tamano: .2},
		gestures.MuestraPanel{X: .2, Y: .2, Tamano: .2})
	if err != nil {
		t.Fatal(err)
	}
	if err := desktop.GuardarCalibracionPanel(filepath.Join(dir, desktop.NombreCalibracionPanel), cal); err != nil {
		t.Fatal(err)
	}
	panel, err := desktop.NuevoPanel(filepath.Join(dir, desktop.NombreCalibracionPanel), nil)
	if err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(dir, "registro.jsonl")
	reg, err := desktop.AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	esp := &espaciosCuenta{}
	p := desktop.NuevasPantallas(nil, esp)
	p.Reg = reg
	flujo := &flujoPrueba{}
	a := &App{cfg: Config{Modo: "tactil", Reproducir: true}, flujo: flujo, panel: panel,
		pantallas: p, ind: &desktop.Indicador{}, reg: reg}

	// Apagado de arranque: el gesto no dispara nada.
	alimentar(t, a, flujo, tres(t, 0, true))
	time.Sleep(100 * time.Millisecond)
	if n := esp.n(); n != 0 {
		t.Fatalf("apagado, los tres dedos cambiaron %d veces", n)
	}

	// Encendido: el flujo queda en activo y el panel se enciende.
	flujo.modo = "activo"
	a.cuadro(gestures.Cuadro{TMs: 2000})
	alimentar(t, a, flujo, tres(t, 3000, true))
	if !esperarEspacios(esp, 1) {
		t.Fatal("encendido, los tres dedos no cambiaron")
	}
	time.Sleep(50 * time.Millisecond)

	// Apagado otra vez: no dispara.
	flujo.modo = ""
	a.cuadro(gestures.Cuadro{TMs: 5000})
	alimentar(t, a, flujo, tres(t, 6000, true))
	time.Sleep(100 * time.Millisecond)
	if n := esp.n(); n != 1 {
		t.Fatalf("tras apagar, los tres dedos cambiaron %d veces", n-1)
	}

	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), `"resultado":"no","motivo":"apagado"`); n != 2 {
		t.Errorf("%d intentos con motivo apagado en el registro:\n%s", n, b)
	}
}
