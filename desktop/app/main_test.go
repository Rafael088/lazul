package main

import (
	"path/filepath"
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
