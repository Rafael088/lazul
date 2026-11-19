package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rafael088/lazul/desktop"
	"github.com/Rafael088/lazul/gestures"
)

type flujoPrueba struct {
	prueba bool
	ultimo gestures.Cuadro
}

func (f *flujoPrueba) Cuadro(c gestures.Cuadro, _ time.Time) desktop.Vista {
	f.ultimo = c
	return desktop.Vista{}
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
