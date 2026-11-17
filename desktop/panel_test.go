package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rafael088/lazul/gestures"
)

type punteroPanelPrueba struct {
	preparado bool
	puntos    []gestures.PuntoPanel
}

func (p *punteroPanelPrueba) Prepare(context.Context) error { p.preparado = true; return nil }
func (p *punteroPanelPrueba) MoveAbsolute(_ context.Context, x, y float64) error {
	p.puntos = append(p.puntos, gestures.PuntoPanel{X: x, Y: y})
	return nil
}

func muestraPanelPrueba(x, y, tamano float64) gestures.MuestraPanel {
	return gestures.MuestraPanel{X: x * tamano, Y: y * tamano, Tamano: tamano}
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
