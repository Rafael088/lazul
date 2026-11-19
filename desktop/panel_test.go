package desktop

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

type tactilPanelPrueba struct {
	err    error
	frames [][]platform.TouchPoint
}

func (t *tactilPanelPrueba) Prepare(context.Context) error { return t.err }
func (t *tactilPanelPrueba) Frame(_ context.Context, puntos []platform.TouchPoint) error {
	if t.err != nil {
		return t.err
	}
	t.frames = append(t.frames, append([]platform.TouchPoint(nil), puntos...))
	return nil
}

type accionesPanelPrueba struct{ zoom int }

func (*accionesPanelPrueba) Scroll(context.Context, platform.ScrollDirection) error { return nil }
func (a *accionesPanelPrueba) Zoom(context.Context, platform.ZoomDirection) error {
	a.zoom++
	return nil
}
func (*accionesPanelPrueba) Navigate(context.Context, platform.NavigationDirection, platform.NavigationMode) error {
	return nil
}

type punteroPanelPrueba struct {
	preparado bool
	puntos    []gestures.PuntoPanel
	botones   []bool
}

func (p *punteroPanelPrueba) Prepare(context.Context) error { p.preparado = true; return nil }
func (p *punteroPanelPrueba) MoveAbsolute(_ context.Context, x, y float64) error {
	p.puntos = append(p.puntos, gestures.PuntoPanel{X: x, Y: y})
	return nil
}
func (p *punteroPanelPrueba) Button(_ context.Context, down bool) error {
	p.botones = append(p.botones, down)
	return nil
}

func muestraPanelPrueba(x, y, tamano float64) gestures.MuestraPanel {
	return gestures.MuestraPanel{X: x * tamano, Y: y * tamano, Tamano: tamano}
}

// manoPanelPrueba construye una mano cuyo MuestraPanelDe da (x,y) en el panel
// de prueba [-1,1]² y el tamaño pedido, con el pellizco abierto o cerrado.
func manoPanelPrueba(x, y, tam float64, cerrado bool) gestures.Mano {
	var m gestures.Mano
	medio := tam / math.Sqrt2 / 2
	cx, cy := x*tam+0.5, y*tam+0.5
	m.Puntos[gestures.Muneca] = gestures.Punto{X: float32(cx), Y: float32(cy - medio)}
	m.Puntos[gestures.IndiceMCP] = gestures.Punto{X: float32(cx - medio), Y: float32(cy)}
	m.Puntos[gestures.MedioMCP] = gestures.Punto{X: float32(cx), Y: float32(cy + medio)}
	m.Puntos[gestures.AnularMCP] = gestures.Punto{X: float32(cx), Y: float32(cy)}
	m.Puntos[gestures.MeniqueMCP] = gestures.Punto{X: float32(cx + medio), Y: float32(cy)}
	apertura := 0.80
	if cerrado {
		apertura = 0.10
	}
	m.Puntos[gestures.PulgarPunta] = gestures.Punto{X: float32(cx), Y: float32(cy)}
	m.Puntos[gestures.IndicePunta] = gestures.Punto{X: float32(cx + apertura*tam/math.Sqrt2), Y: float32(cy)}
	return m
}

func cuadroPanelPrueba(t int64, manos ...gestures.Mano) gestures.Cuadro {
	return gestures.Cuadro{TMs: t, Aspecto: 1, Manos: manos}
}

func TestPanelPellizcoPulsaYSueltaElBoton(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, err := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err != nil {
		t.Fatal(err)
	}
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	puntero := &punteroPanelPrueba{}
	p, err := NuevoPanel(ruta, puntero)
	if err != nil {
		t.Fatal(err)
	}
	// Agarra dentro y arrastra a la derecha.
	for i := 0; i < 5; i++ {
		p.Cuadro(cuadroPanelPrueba(int64(i)*67, manoPanelPrueba(-0.5, 0, .2, true)), true)
	}
	if e := p.Estado(); e.Fase != "agarrando" || e.ManosArmadas != 1 {
		t.Fatalf("estado al agarrar: %+v", e)
	}
	if len(puntero.botones) != 1 || !puntero.botones[0] {
		t.Fatalf("no pulsó el botón: %+v", puntero.botones)
	}
	for i := 1; i <= 8; i++ {
		p.Cuadro(cuadroPanelPrueba(int64(5+i)*67, manoPanelPrueba(-0.5+float64(i)*0.1, 0, .2, true)), true)
	}
	// Abre y suelta: el arrastre termina una sola vez, a la derecha.
	for i := 0; i < 5; i++ {
		p.Cuadro(cuadroPanelPrueba(int64(15+i)*67, manoPanelPrueba(0.3, 0, .2, false)), true)
	}
	e := p.Estado()
	if e.Fase != "dentro" || e.Arrastre != "derecha" {
		t.Fatalf("estado al soltar: %+v", e)
	}
	if len(puntero.botones) != 2 || puntero.botones[1] {
		t.Fatalf("no soltó el botón: %+v", puntero.botones)
	}
	if len(puntero.puntos) == 0 {
		t.Fatal("el puntero no se movió")
	}
	// Parar con el botón pulsado no lo deja pulsado.
	for i := 0; i < 5; i++ {
		p.Cuadro(cuadroPanelPrueba(int64(20+i)*67, manoPanelPrueba(0, 0, .2, true)), true)
	}
	p.Parar()
	if ultimo := puntero.botones[len(puntero.botones)-1]; ultimo {
		t.Fatalf("Parar dejó el botón pulsado: %+v", puntero.botones)
	}
}

func TestPanelTactilMuevePunteroSinBotonFiltraYLevanta(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, _ := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	puntero := &punteroPanelPrueba{}
	tactil := &tactilPanelPrueba{}
	acciones := &accionesPanelPrueba{}
	p, _ := NuevoPanel(ruta, puntero)
	p.Tactil, p.Acciones = tactil, acciones
	if err := p.Preparar(); err != nil {
		t.Fatal(err)
	}

	clave := gestures.ClaveMano{}
	for i, dx := range []float64{0, .01, -.01, .01, -.01} {
		r := gestures.ResultadoEmbrague{TMs: int64(i * 33), Fase: gestures.EmbragueAgarrando, Dentro: true, TienePos: true,
			X: .5 + dx, Y: .5 - dx, ManosArmadas: 1, Armadas: []gestures.ManoArmada{{Clave: clave, X: .5 + dx, Y: .5 - dx}}}
		p.mu.Lock()
		p.aplicar(r, r.TMs, true)
		p.mu.Unlock()
	}
	if len(puntero.botones) != 0 {
		t.Fatalf("el táctil pulsó el botón: %+v", puntero.botones)
	}
	if len(tactil.frames) != 5 || len(tactil.frames[0]) != 1 || !tactil.frames[0][0].Down {
		t.Fatalf("cuadros táctiles: %+v", tactil.frames)
	}
	if tactil.frames[0][0].X != puntero.puntos[0].X || tactil.frames[0][0].Y != puntero.puntos[0].Y {
		t.Fatalf("el primer toque %+v no coincide con el puntero %+v", tactil.frames[0][0], puntero.puntos[0])
	}
	ultimo := tactil.frames[len(tactil.frames)-1][0]
	if math.Abs(ultimo.X-.5) >= .01 || math.Abs(ultimo.Y-.5) >= .01 {
		t.Fatalf("el filtro no redujo el temblor: %+v", ultimo)
	}

	// Dos manos llegan como dos dedos; el zoom de teclado no se ejecuta.
	r := gestures.ResultadoEmbrague{TMs: 200, Fase: gestures.EmbragueAgarrando, Dentro: true, TienePos: true, X: .4, Y: .5,
		ManosArmadas: 2, Armadas: []gestures.ManoArmada{{Clave: clave, X: .4, Y: .5}, {Clave: gestures.ClaveMano{Derecha: true}, X: .7, Y: .5}},
		Zoom: &gestures.Zoom{Direccion: gestures.ZoomAcercar}}
	p.mu.Lock()
	p.aplicar(r, r.TMs, true)
	p.mu.Unlock()
	if got := tactil.frames[len(tactil.frames)-1]; len(got) != 2 || !got[0].Down || !got[1].Down {
		t.Fatalf("zoom no salió como dos dedos: %+v", got)
	}
	if acciones.zoom != 0 {
		t.Fatalf("el táctil también mandó Ctrl +/−: %d", acciones.zoom)
	}

	p.Parar()
	final := tactil.frames[len(tactil.frames)-1]
	if len(final) != 2 || final[0].Down || final[1].Down {
		t.Fatalf("Parar no levantó los dedos: %+v", final)
	}

	// Recalibrar usa la misma garantía: ningún contacto queda pegado.
	p.mu.Lock()
	p.aplicar(gestures.ResultadoEmbrague{TMs: 300, Fase: gestures.EmbragueAgarrando, Dentro: true, TienePos: true,
		X: .5, Y: .5, ManosArmadas: 1, Armadas: []gestures.ManoArmada{{Clave: clave, X: .5, Y: .5}}}, 300, true)
	p.mu.Unlock()
	p.EmpezarCalibracion()
	final = tactil.frames[len(tactil.frames)-1]
	if len(final) != 1 || final[0].Down {
		t.Fatalf("recalibrar no levantó el dedo: %+v", final)
	}
}

func TestPanelRespaldoSiNoPuedePrepararTactil(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, _ := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	puntero := &punteroPanelPrueba{}
	acciones := &accionesPanelPrueba{}
	p, _ := NuevoPanel(ruta, puntero)
	p.Tactil = &tactilPanelPrueba{err: platform.ErrNoImplementado}
	p.Acciones = acciones
	if err := p.Preparar(); !errors.Is(err, platform.ErrNoImplementado) {
		t.Fatalf("error al preparar: %v", err)
	}
	if e := p.Estado(); e.ModoEntrada != "respaldo" || e.AvisoEntrada == "" {
		t.Fatalf("la app no muestra el respaldo: %+v", e)
	}
	r := gestures.ResultadoEmbrague{TMs: 1, Fase: gestures.EmbragueAgarrando, Dentro: true, TienePos: true,
		X: .5, Y: .5, ManosArmadas: 1, Armadas: []gestures.ManoArmada{{X: .5, Y: .5}}}
	p.mu.Lock()
	p.aplicar(r, r.TMs, true)
	p.mu.Unlock()
	if len(puntero.botones) != 1 || !puntero.botones[0] {
		t.Fatalf("el respaldo no pulsó el botón: %+v", puntero.botones)
	}
	r.ManosArmadas = 2
	r.Zoom = &gestures.Zoom{Direccion: gestures.ZoomAlejar}
	p.mu.Lock()
	p.aplicar(r, 2, true)
	p.mu.Unlock()
	if acciones.zoom != 1 {
		t.Fatalf("el respaldo no mandó zoom: %d", acciones.zoom)
	}
}

func TestRegistroPanelNoGuardaPosiciones(t *testing.T) {
	dir := t.TempDir()
	reg, err := AbrirRegistro(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := NuevoPanel(filepath.Join(dir, NombreCalibracionPanel), nil)
	p.Tactil, p.Reg = &tactilPanelPrueba{}, reg
	if err := p.Preparar(); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	p.tactilListo = true
	p.enviarToques(gestures.ResultadoEmbrague{TMs: 1, ManosArmadas: 1,
		Armadas: []gestures.ManoArmada{{X: .123456, Y: .654321}}}, 1)
	p.mu.Unlock()
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, NombreRegistro))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"x":`) || strings.Contains(string(b), `"y":`) || strings.Contains(string(b), "123456") || strings.Contains(string(b), "654321") {
		t.Fatalf("el registro contiene una posición: %s", b)
	}
}

func alimentarEsquina(p *Panel, m gestures.MuestraPanel, desde int64) {
	for i := 0; i < muestrasParaEsquina; i++ {
		p.mu.Lock()
		p.observarMuestra(m, desde+int64(i*33), false)
		p.mu.Unlock()
	}
}

func TestPanelCalibraGuardaRecargaYMueve(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	puntero := &punteroPanelPrueba{}
	p, err := NuevoPanel(ruta, puntero)
	if err != nil {
		t.Fatal(err)
	}
	p.EmpezarCalibracion()
	alimentarEsquina(p, muestraPanelPrueba(-1, -.8, .2), 0)
	if !p.Estado().ListoCapturar {
		t.Fatal("la primera esquina estable no quedó lista")
	}
	if err := p.CapturarEsquina(); err != nil {
		t.Fatal(err)
	}
	alimentarEsquina(p, muestraPanelPrueba(1, .8, .3), 500)
	if err := p.CapturarEsquina(); err != nil {
		t.Fatal(err)
	}
	if e := p.Estado(); !e.Calibrado || e.Calibrando {
		t.Fatalf("estado tras calibrar: %+v", e)
	}
	if info, err := os.Stat(ruta); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("archivo privado: info %v, error %v", info, err)
	}

	p.mu.Lock()
	p.observarMuestra(muestraPanelPrueba(0, 0, .25), 1000, true)
	p.mu.Unlock()
	if len(puntero.puntos) != 1 || puntero.puntos[0].X < .49 || puntero.puntos[0].X > .51 ||
		puntero.puntos[0].Y < .49 || puntero.puntos[0].Y > .51 {
		t.Fatalf("puntero en el centro: %+v", puntero.puntos)
	}

	otroPuntero := &punteroPanelPrueba{}
	recargado, err := NuevoPanel(ruta, otroPuntero)
	if err != nil || !recargado.Estado().Calibrado {
		t.Fatalf("no recargó la calibración: estado %+v, error %v", recargado.Estado(), err)
	}
	if err := recargado.Preparar(); err != nil || !otroPuntero.preparado {
		t.Fatalf("no preparó el puntero: preparado %v, error %v", otroPuntero.preparado, err)
	}
}

func TestPanelNoMueveFueraNiDuranteLaPrueba(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, _ := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	puntero := &punteroPanelPrueba{}
	p, _ := NuevoPanel(ruta, puntero)
	p.mu.Lock()
	p.observarMuestra(muestraPanelPrueba(1.2, 0, .2), 0, true)
	p.observarMuestra(muestraPanelPrueba(0, 0, .2), 33, false)
	p.mu.Unlock()
	if len(puntero.puntos) != 0 {
		t.Fatalf("movió fuera del panel o con mover=false: %+v", puntero.puntos)
	}
}

func TestPanelFiltraManosFueraDelRectanguloCalibrado(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, _ := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	p, _ := NuevoPanel(ruta, &punteroPanelPrueba{})
	cuadro := cuadroPanelPrueba(0, manoPanelPrueba(0, 0, .2, false), manoPanelPrueba(1.5, 0, .2, false))
	filtrado, rechazadas := p.Filtrar(cuadro)
	if len(filtrado.Manos) != 1 || rechazadas != 1 {
		t.Fatalf("manos dentro %d, rechazadas %d", len(filtrado.Manos), rechazadas)
	}

	p.EmpezarCalibracion()
	filtrado, rechazadas = p.Filtrar(cuadro)
	if len(filtrado.Manos) != 2 || rechazadas != 0 {
		t.Fatalf("durante calibración no debe filtrar: manos %d, rechazadas %d", len(filtrado.Manos), rechazadas)
	}
}

func TestPanelFiltraConLaMismaBandaDeHisteresis(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	c, _ := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	p, _ := NuevoPanel(ruta, &punteroPanelPrueba{})
	casos := []struct {
		nombre string
		x      float64
		pasa   bool
	}{
		{"no entra desde fuera", 1.10, false},
		{"entra por el borde", 1.00, true},
		{"permanece en la banda", 1.20, true},
		{"sale de la banda", 1.31, false},
		{"no reentra desde fuera", 1.10, false},
		{"reentra por el borde", 1.00, true},
	}
	for i, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			filtrado, rechazadas := p.Filtrar(cuadroPanelPrueba(int64(i), manoPanelPrueba(caso.x, 0, .2, false)))
			pasa := len(filtrado.Manos) == 1 && rechazadas == 0
			if pasa != caso.pasa {
				t.Fatalf("x %.2f: pasa %v, rechazadas %d; quería pasa %v", caso.x, pasa, rechazadas, caso.pasa)
			}
		})
	}
}

func TestAvisoEsquinaCercaDelBordeDeImagen(t *testing.T) {
	casos := []struct {
		nombre string
		m      gestures.MuestraPanel
		avisa  bool
	}{
		{"centro seguro", gestures.MuestraPanel{X: 0, Y: 0, Tamano: .2}, false},
		{"cerca de izquierda", gestures.MuestraPanel{X: -.55, Y: 0, Tamano: .2}, true},
		{"cerca de abajo", gestures.MuestraPanel{X: 0, Y: .45, Tamano: .2}, true},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			avisa := avisoEsquinaCercaDelBorde("La esquina", caso.m, 4.0/3.0) != ""
			if avisa != caso.avisa {
				t.Fatalf("avisa %v, quería %v", avisa, caso.avisa)
			}
		})
	}
}

func TestPanelExigeManoQuietaParaCapturar(t *testing.T) {
	p, _ := NuevoPanel(filepath.Join(t.TempDir(), NombreCalibracionPanel), &punteroPanelPrueba{})
	p.EmpezarCalibracion()
	for i := 0; i < muestrasParaEsquina; i++ {
		m := muestraPanelPrueba(-1+float64(i%2)*.6, -.8, .2) // se mueve media palma por cuadro
		p.mu.Lock()
		p.observarMuestra(m, int64(i*33), false)
		p.mu.Unlock()
	}
	if err := p.CapturarEsquina(); err == nil {
		t.Fatal("capturó una mano que estaba temblando")
	}
}

// Con el brazo estirado hacia una esquina la palma tiembla unos píxeles por
// cuadro (en las tomas reales, 3–7 px a 640): eso tiene que poder capturarse.
func TestPanelCapturaConElTemblorNormal(t *testing.T) {
	p, _ := NuevoPanel(filepath.Join(t.TempDir(), NombreCalibracionPanel), &punteroPanelPrueba{})
	p.EmpezarCalibracion()
	for i := 0; i < muestrasParaEsquina; i++ {
		d := float64(i%3-1) * .12
		m := muestraPanelPrueba(-1+d, -.8-d, .2*(1+d/4))
		p.mu.Lock()
		p.observarMuestra(m, int64(i*33), false)
		p.mu.Unlock()
	}
	if err := p.CapturarEsquina(); err != nil {
		t.Fatalf("no capturó una mano quieta con temblor normal: %v", err)
	}
}

func TestPanelSinCalibrarNoDejaPasarManos(t *testing.T) {
	p, _ := NuevoPanel(filepath.Join(t.TempDir(), NombreCalibracionPanel), &punteroPanelPrueba{})
	cuadro := cuadroPanelPrueba(0, manoPanelPrueba(0, 0, .2, false))
	filtrado, rechazadas := p.Filtrar(cuadro)
	if len(filtrado.Manos) != 0 || rechazadas != 1 {
		t.Fatalf("sin calibrar: manos %d, rechazadas %d", len(filtrado.Manos), rechazadas)
	}
}
