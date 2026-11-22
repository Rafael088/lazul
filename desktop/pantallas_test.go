package desktop

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// pantallasPrueba es un escritorio falso. moverFoco imita a Hyprland (Target
// lleva el foco); sin él, a Windows (el foco se queda donde estaba).
type pantallasPrueba struct {
	ds        []platform.Display
	targets   []string
	moverFoco bool
}

func (f *pantallasPrueba) List(context.Context) ([]platform.Display, error) {
	return append([]platform.Display(nil), f.ds...), nil
}

func (f *pantallasPrueba) Target(_ context.Context, name string) error {
	f.targets = append(f.targets, name)
	if f.moverFoco {
		f.enfocar(name)
	}
	return nil
}

func (f *pantallasPrueba) enfocar(name string) {
	for i := range f.ds {
		f.ds[i].Focused = f.ds[i].Name == name
	}
}

func dosMonitores(foco string) []platform.Display {
	ds := []platform.Display{
		{Name: "eDP-1", X: 0, Y: 0, Width: 1440, Height: 900},
		{Name: "DP-1", X: 1440, Y: 0, Width: 1920, Height: 1080},
	}
	for i := range ds {
		ds[i].Focused = ds[i].Name == foco
	}
	return ds
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
	gs, err := gestures.LeerDirectorio("../fixtures/gestos")
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
	return tresRecorrido(t, t0, derecha, 1.5)
}

func tresRecorrido(t *testing.T, t0 int64, derecha bool, palmas float64) []gestures.Cuadro {
	m, asp := manoTres(t)
	a, b := m.Puntos[gestures.Muneca], m.Puntos[gestures.MedioMCP]
	palma := math.Hypot(float64(b.X-a.X)*float64(asp), float64(b.Y-a.Y)) / float64(asp)
	dx := float32(palmas * palma)
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

func pasarPantallas(p *Pantallas, cs []gestures.Cuadro, libre bool, t0 time.Time) int {
	cambios := 0
	for _, c := range cs {
		if _, cambio := p.Cuadro(c, libre, false, t0.Add(time.Duration(c.TMs)*time.Millisecond)); cambio {
			cambios++
		}
	}
	if esperarPantallas(p) {
		cambios++
	}
	return cambios
}

// espaciosPrueba anota cada cambio de espacio.
type espaciosPrueba struct {
	mu    sync.Mutex
	dirs  []platform.NavigationDirection
	falla error
}

func (e *espaciosPrueba) Switch(_ context.Context, d platform.NavigationDirection) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dirs = append(e.dirs, d)
	return e.falla
}

func abrirRegistroPrueba(t *testing.T) (*Registro, func() string) {
	t.Helper()
	dir := t.TempDir()
	reg, err := AbrirRegistro(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	return reg, func() string {
		if err := reg.Cerrar(); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, NombreRegistro))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
}

// esperarPantallas espera solo al escritorio falso de las pruebas. En la app
// real Cuadro recoge el cambio terminado en el fotograma siguiente.
func esperarPantallas(p *Pantallas) bool {
	limite := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(limite) {
		p.mu.Lock()
		trabajando := p.trabajando
		if !trabajando {
			cambio := p.cambio
			p.cambio = false
			p.mu.Unlock()
			return cambio
		}
		p.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	return false
}

func TestPantallasUnMonitorCambiaDeEspacio(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1")[:1]}
	e := &espaciosPrueba{}
	reg, leer := abrirRegistroPrueba(t)
	p := NuevasPantallas(f, e)
	p.Reg = reg
	var avisos []string
	p.AlCambiar = func(d platform.Display) { avisos = append(avisos, d.Name) }
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if n := pasarPantallas(p, tres(t, 0, true), true, t0); n != 0 {
		t.Fatalf("con un monitor los tres dedos cambiaron de monitor %d veces", n)
	}
	if n := pasarPantallas(p, tres(t, 3000, false), true, t0); n != 0 {
		t.Fatalf("con un monitor los tres dedos cambiaron de monitor %d veces", n)
	}
	if len(f.targets) != 0 {
		t.Fatalf("con un monitor se llamó a Target: %v", f.targets)
	}
	if len(e.dirs) != 2 || e.dirs[0] != platform.NavigateRight || e.dirs[1] != platform.NavigateLeft {
		t.Fatalf("espacios = %v, quiere [derecha izquierda]", e.dirs)
	}
	if d, ok := p.Actual(); !ok || d.Name != "eDP-1" || len(avisos) != 1 {
		t.Fatalf("actual = %v %v, avisos %v", d.Name, ok, avisos)
	}
	s := leer()
	if n := strings.Count(s, `"resultado":"disparo","motivo":"espacio"`); n != 2 {
		t.Errorf("%d disparos de espacio en el registro:\n%s", n, s)
	}
}

func TestPantallasSinNadaAnotaSinAccion(t *testing.T) {
	reg, leer := abrirRegistroPrueba(t)
	p := NuevasPantallas(nil, nil)
	p.Reg = reg
	p.Iniciar(time.Unix(0, 0))
	if pasarPantallas(p, tres(t, 0, true), true, time.Unix(0, 0)) != 0 {
		t.Fatal("sin Displays hubo cambio")
	}
	if _, ok := p.Actual(); ok {
		t.Fatal("sin Displays hay monitor actual")
	}
	if s := leer(); !strings.Contains(s, `"dir":"derecha","resultado":"disparo","motivo":"sin_accion"`) {
		t.Errorf("el registro no dice que no hubo acción:\n%s", s)
	}
}

func TestPantallasEspacioFallaSeAnota(t *testing.T) {
	reg, leer := abrirRegistroPrueba(t)
	p := NuevasPantallas(nil, &espaciosPrueba{falla: errors.New("hyprctl no responde")})
	p.Reg = reg
	pasarPantallas(p, tres(t, 0, true), true, time.Unix(0, 0))
	if s := leer(); !strings.Contains(s, `"motivo":"error"`) || !strings.Contains(s, "hyprctl no responde") {
		t.Errorf("el error no quedó en el registro:\n%s", s)
	}
}

func TestPantallasArrancaEnElFocoYCambiaConTresDedos(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1"), moverFoco: true}
	e := &espaciosPrueba{}
	reg, leer := abrirRegistroPrueba(t)
	p := NuevasPantallas(f, e)
	p.Reg = reg
	var avisos []string
	p.AlCambiar = func(d platform.Display) { avisos = append(avisos, d.Name) }
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if strings.Join(f.targets, ",") != "eDP-1" {
		t.Fatalf("al iniciar, targets = %v; quiere el del foco", f.targets)
	}
	// Tres dedos a la derecha de la persona: pasa a la externa.
	if n := pasarPantallas(p, tres(t, 0, true), true, t0); n != 1 {
		t.Fatalf("tres dedos a la derecha: %d cambios", n)
	}
	if d, _ := p.Actual(); d.Name != "DP-1" {
		t.Fatalf("actual = %s, quiere DP-1", d.Name)
	}
	// Otra vez a la derecha: no hay nada más allá (y no cambia de espacio).
	if n := pasarPantallas(p, tres(t, 3000, true), true, t0); n != 0 {
		t.Fatalf("sin vecino a la derecha cambió %d veces", n)
	}
	// Y a la izquierda vuelve al portátil.
	if n := pasarPantallas(p, tres(t, 6000, false), true, t0); n != 1 {
		t.Fatalf("tres dedos a la izquierda: %d cambios", n)
	}
	if strings.Join(f.targets, ",") != "eDP-1,DP-1,eDP-1" || strings.Join(avisos, ",") != "eDP-1,DP-1,eDP-1" {
		t.Fatalf("targets = %v, avisos = %v", f.targets, avisos)
	}
	if len(e.dirs) != 0 {
		t.Fatalf("con dos monitores se cambió de espacio: %v", e.dirs)
	}
	s := leer()
	for _, quiere := range []string{`"tipo":"pantalla","dir":"derecha","resultado":"disparo","motivo":"monitor"`, `"dir":"derecha","resultado":"disparo","motivo":"sin_vecino"`, `"dir":"izquierda","resultado":"disparo","motivo":"monitor"`, `"palmas":1.`} {
		if !strings.Contains(s, quiere) {
			t.Errorf("el registro no tiene %s:\n%s", quiere, s)
		}
	}
	if strings.Contains(s, `"x":`) || strings.Contains(s, "1440") {
		t.Errorf("el registro guarda posiciones o geometría: %s", s)
	}
}

func TestPantallasIntentosFallidosSeAnotan(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1"), moverFoco: true}
	reg, leer := abrirRegistroPrueba(t)
	p := NuevasPantallas(f, nil)
	p.Reg = reg
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	// Agarrando: no cambia y queda como ocupado.
	if n := pasarPantallas(p, tres(t, 0, true), false, t0); n != 0 {
		t.Fatalf("agarrando, los tres dedos cambiaron %d veces", n)
	}
	// Se queda corto: media palma.
	if n := pasarPantallas(p, tresRecorrido(t, 3000, true, 0.5), true, t0); n != 0 {
		t.Fatalf("media palma cambió %d veces", n)
	}
	s := leer()
	for _, quiere := range []string{`"resultado":"no","motivo":"ocupado"`, `"resultado":"no","motivo":"corto"`} {
		if !strings.Contains(s, quiere) {
			t.Errorf("el registro no tiene %s:\n%s", quiere, s)
		}
	}
	if len(f.targets) != 1 {
		t.Fatalf("targets = %v", f.targets)
	}
}

// Mientras se ve la pose, esa mano no llega al panel ni al clasificador;
// las demás sí.
func TestPantallasQuitaLaManoDeTresDedos(t *testing.T) {
	p := NuevasPantallas(nil, nil)
	abierta, _ := manoAbierta(t)
	t0 := time.Unix(0, 0)
	for _, c := range tres(t, 0, true) {
		hay := len(c.Manos)
		c.Manos = append(c.Manos, abierta)
		fuera, _ := p.Cuadro(c, true, false, t0)
		if hay == 1 && len(fuera.Manos) != 1 {
			t.Fatalf("a los %d ms pasan %d manos, quiere solo la abierta", c.TMs, len(fuera.Manos))
		}
		if len(fuera.Manos) > 0 && gestures.TresDedos(fuera.Manos[0], c.Aspecto) {
			t.Fatalf("a los %d ms la mano de tres dedos llegó al panel", c.TMs)
		}
	}
}

func TestPantallasSigueAlFoco(t *testing.T) {
	f := &pantallasPrueba{ds: dosMonitores("eDP-1")}
	p := NuevasPantallas(f, nil)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	quieto := []gestures.Cuadro{{TMs: 0}}
	// La persona hace clic en la externa: al siguiente vistazo, el panel va allí.
	f.enfocar("DP-1")
	if _, c := p.Cuadro(quieto[0], true, false, t0.Add(500*time.Millisecond)); c {
		t.Fatal("se consultó el foco antes de ConsultarFocoCada")
	}
	if _, c := p.Cuadro(quieto[0], true, false, t0.Add(ConsultarFocoCada)); c {
		t.Fatal("la consulta asíncrona anunció un cambio antes de terminar")
	}
	if !esperarPantallas(p) {
		t.Fatal("no siguió al foco")
	}
	if d, _ := p.Actual(); d.Name != "DP-1" {
		t.Fatalf("actual = %s", d.Name)
	}
}

func TestPantallasTresDedosSinMoverFocoNoSeDeshace(t *testing.T) {
	// Como en Windows: Target no mueve el foco. El seguimiento no debe
	// devolver el control al monitor del foco tras los tres dedos.
	f := &pantallasPrueba{ds: dosMonitores("eDP-1")}
	p := NuevasPantallas(f, nil)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	if pasarPantallas(p, tres(t, 0, true), true, t0) != 1 {
		t.Fatal("los tres dedos no cambiaron")
	}
	for i := 1; i <= 3; i++ {
		p.Cuadro(gestures.Cuadro{TMs: int64(i) * 1000}, true, false, t0.Add(time.Duration(i)*1100*time.Millisecond))
	}
	if d, _ := p.Actual(); d.Name != "DP-1" {
		t.Fatalf("el seguimiento del foco deshizo el cambio: actual = %s", d.Name)
	}
}

type pantallasRotas struct{ listas int }

func (f *pantallasRotas) List(context.Context) ([]platform.Display, error) {
	f.listas++
	return nil, errors.New("hyprctl: no hay Hyprland")
}
func (f *pantallasRotas) Target(context.Context, string) error { return errors.New("no") }

func TestPantallasSinCompositorAnotaUnaVez(t *testing.T) {
	dir := t.TempDir()
	reg, err := AbrirRegistro(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	f := &pantallasRotas{}
	p := NuevasPantallas(f, nil)
	p.Reg = reg
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)
	for i := 1; i <= 5; i++ {
		if _, c := p.Cuadro(gestures.Cuadro{}, true, false, t0.Add(time.Duration(i)*ConsultarFocoCada)); c {
			t.Fatal("sin monitores hubo cambio")
		}
		esperarPantallas(p)
	}
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, NombreRegistro))
	if n := strings.Count(string(b), "error_listar"); n != 1 || f.listas != 6 {
		t.Fatalf("error_listar %d veces en %d consultas:\n%s", n, f.listas, b)
	}
}

type pantallasBloqueadas struct {
	ds     []platform.Display
	inicio chan struct{}
	soltar chan struct{}
	mu     sync.Mutex
	listas int
	unaVez sync.Once
}

func (f *pantallasBloqueadas) List(ctx context.Context) ([]platform.Display, error) {
	f.mu.Lock()
	f.listas++
	n := f.listas
	f.mu.Unlock()
	if n == 1 {
		return append([]platform.Display(nil), f.ds...), nil
	}
	f.unaVez.Do(func() { close(f.inicio) })
	select {
	case <-f.soltar:
		return append([]platform.Display(nil), f.ds...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*pantallasBloqueadas) Target(context.Context, string) error { return nil }

func TestPantallasConsultarFocoNuncaBloqueaElCuadro(t *testing.T) {
	f := &pantallasBloqueadas{
		ds: dosMonitores("eDP-1"), inicio: make(chan struct{}), soltar: make(chan struct{}),
	}
	p := NuevasPantallas(f, nil)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0) // La primera lista prepara el estado sin bloquear.

	inicio := time.Now()
	p.Cuadro(gestures.Cuadro{}, true, false, t0.Add(ConsultarFocoCada))
	if tarda := time.Since(inicio); tarda > 50*time.Millisecond {
		t.Fatalf("consultar monitores bloqueó el cuadro durante %v", tarda)
	}
	select {
	case <-f.inicio:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("la consulta no arrancó fuera del cuadro")
	}
	close(f.soltar)
	esperarPantallas(p)
}

func TestPantallasConsultarParaTresDedosNuncaBloqueaElCuadro(t *testing.T) {
	f := &pantallasBloqueadas{
		ds: dosMonitores("eDP-1"), inicio: make(chan struct{}), soltar: make(chan struct{}),
	}
	p := NuevasPantallas(f, nil)
	t0 := time.Unix(0, 0)
	p.Iniciar(t0)

	inicio := time.Now()
	for _, c := range tres(t, 0, true) {
		p.Cuadro(c, true, false, t0.Add(time.Duration(c.TMs)*time.Millisecond))
	}
	if tarda := time.Since(inicio); tarda > 50*time.Millisecond {
		t.Fatalf("consultar monitores para los tres dedos bloqueó cuadros durante %v", tarda)
	}
	select {
	case <-f.inicio:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("la consulta de los tres dedos no arrancó fuera del cuadro")
	}
	close(f.soltar)
	esperarPantallas(p)
}
