package desktop

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rafael088/lazul/gestures"
)

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

func TestPanelExigeManoQuietaParaCapturar(t *testing.T) {
	p, _ := NuevoPanel(filepath.Join(t.TempDir(), NombreCalibracionPanel), &punteroPanelPrueba{})
	p.EmpezarCalibracion()
	for i := 0; i < muestrasParaEsquina; i++ {
		m := muestraPanelPrueba(-1+float64(i%2)*.3, -.8, .2)
		p.mu.Lock()
		p.observarMuestra(m, int64(i*33), false)
		p.mu.Unlock()
	}
	if err := p.CapturarEsquina(); err == nil {
		t.Fatal("capturó una mano que estaba temblando")
	}
}
