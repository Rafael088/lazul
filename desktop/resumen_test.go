package desktop

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)

func en(min float64) time.Time { return t0.Add(time.Duration(min * float64(time.Minute))) }

// uso: una sesión con latidos cada 30 s entre a y b minutos.
func uso(sesion string, a, b float64) []Linea {
	ls := []Linea{{T: en(a), Tipo: TipoInicio, Sesion: sesion}}
	for m := a + 0.5; m < b; m += 0.5 {
		ls = append(ls, Linea{T: en(m), Tipo: TipoLatido, Sesion: sesion})
	}
	return append(ls, Linea{T: en(b), Tipo: TipoFin, Sesion: sesion})
}

func act(sesion string, min float64, n int, res string, prueba bool) Linea {
	return Linea{T: en(min), Tipo: TipoActivacion, Sesion: sesion, Activacion: n, Resultado: res, Prueba: prueba}
}

func cerca(a, b time.Duration) bool { d := a - b; return d > -time.Second && d < time.Second }

func TestResumenVentanaDe30MinDeUso(t *testing.T) {
	var ls []Linea
	ls = append(ls, uso("a", 0, 20)...)  // 20 min
	ls = append(ls, uso("b", 60, 75)...) // 15 min, una hora después
	ls = append(ls, act("a", 2, 1, ActSinGesto, false), act("a", 12, 2, ActCaptura, false),
		act("b", 61, 1, ActSinGesto, false), act("b", 70, 2, ActGesto4, false))
	ls = append(ls, Linea{T: en(62), Tipo: TipoMarca, Sesion: "b", Activacion: 1, Falsa: new(false)})
	r := Resumir(ls)
	if !cerca(r.UsoTotal, 35*time.Minute) || !cerca(r.Uso, 30*time.Minute) {
		t.Fatalf("uso %v total %v", r.Uso, r.UsoTotal)
	}
	// 30 min hacia atrás: los 15 de b y los últimos 15 de a (desde el minuto 5).
	if !r.Desde.Equal(en(5)) {
		t.Errorf("desde %v, quería el minuto 5", r.Desde.Sub(t0))
	}
	// Entran la captura del minuto 12, la de b marcada «a propósito» y el gesto 4.
	if r.Activaciones != 3 || r.Falsas != 0 || r.Buscadas != 3 {
		t.Errorf("activaciones %d falsas %d buscadas %d", r.Activaciones, r.Falsas, r.Buscadas)
	}
	if !strings.HasPrefix(r.Veredicto(), "PASA") {
		t.Errorf("veredicto %q", r.Veredicto())
	}
}

func TestResumenFalsas(t *testing.T) {
	ls := uso("a", 0, 40)
	ls = append(ls, act("a", 30, 1, ActSinGesto, false), act("a", 31, 2, ActCaptura, false), act("a", 32, 3, ActCaptura, false))
	ls = append(ls, Linea{T: en(33), Tipo: TipoMarca, Sesion: "a", Activacion: 3, Falsa: new(true)})
	r := Resumir(ls)
	if r.Falsas != 2 || r.Buscadas != 1 {
		t.Errorf("falsas %d buscadas %d", r.Falsas, r.Buscadas)
	}
	if !strings.HasPrefix(r.Veredicto(), "NO PASA") {
		t.Errorf("veredicto %q", r.Veredicto())
	}
}

func TestResumenFaltaUso(t *testing.T) {
	r := Resumir(uso("a", 0, 10))
	if !strings.HasPrefix(r.Veredicto(), "FALTA USO") {
		t.Errorf("veredicto %q", r.Veredicto())
	}
	if r := Resumir(nil); !strings.HasPrefix(r.Veredicto(), "FALTA USO") || r.Activaciones != 0 {
		t.Errorf("vacío: %+v", r)
	}
}

func TestResumenHuecosNoSonUso(t *testing.T) {
	// Sin latidos durante 10 min (la app colgada o el equipo dormido).
	ls := []Linea{{T: en(0), Tipo: TipoInicio, Sesion: "a"}, {T: en(10), Tipo: TipoLatido, Sesion: "a"}, {T: en(10.5), Tipo: TipoLatido, Sesion: "a"}}
	if r := Resumir(ls); !cerca(r.UsoTotal, 30*time.Second) {
		t.Errorf("uso %v, quería 30 s", r.UsoTotal)
	}
}

func TestResumenPruebaDeTrabajoNormal(t *testing.T) {
	ls := uso("a", 0, 35)
	ls = append(ls, Linea{T: en(1), Tipo: TipoPrueba, Sesion: "a", Resultado: "empieza", Prueba: true},
		Linea{T: en(34), Tipo: TipoPrueba, Sesion: "a", Resultado: "termina"})
	r := Resumir(ls)
	if !cerca(r.PruebaUso, 33*time.Minute) || r.PruebaActivaciones != 0 || !strings.HasPrefix(r.Veredicto(), "PASA") {
		t.Errorf("prueba %v %d %q", r.PruebaUso, r.PruebaActivaciones, r.Veredicto())
	}
	ls = append(ls, act("a", 20, 1, ActCaptura, true), act("a", 25, 2, ActSinGesto, true))
	r = Resumir(ls)
	if r.PruebaActivaciones != 2 || r.Falsas != 2 || !strings.HasPrefix(r.Veredicto(), "NO PASA") {
		t.Errorf("prueba %d falsas %d %q", r.PruebaActivaciones, r.Falsas, r.Veredicto())
	}
	if !strings.Contains(r.Texto(), "Prueba de trabajo normal (terminada") {
		t.Errorf("texto:\n%s", r.Texto())
	}
}

func TestResumenCuentaActivacionesEvitadasPorZona(t *testing.T) {
	ls := uso("a", 0, 35)
	ls = append(ls,
		Linea{T: en(6), Tipo: TipoEvitada, Sesion: "a", Resultado: "sin_torso"},
		Linea{T: en(10), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_torso"},
		Linea{T: en(10.5), Tipo: TipoPrueba, Sesion: "a", Resultado: "empieza", Prueba: true},
		Linea{T: en(11), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_panel", Prueba: true},
		Linea{T: en(12), Tipo: TipoPrueba, Sesion: "a", Resultado: "termina"},
	)
	r := Resumir(ls)
	if r.Evitadas != 3 || r.EvitadasPorMotivo["fuera_torso"] != 1 || r.PruebaEvitadas != 1 {
		t.Fatalf("evitadas %+v", r)
	}
	texto := r.Texto()
	if !strings.Contains(texto, "Activaciones evitadas por la zona: 3") || !strings.Contains(texto, "fuera_panel 1") {
		t.Fatalf("texto:\n%s", texto)
	}
}

func TestResumenCuentaLasSalidasDelPanelPorMotivo(t *testing.T) {
	ls := uso("a", 0, 35)
	ls = append(ls,
		Linea{T: en(6), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_panel", Motivo: MotivoFueraBorde},
		Linea{T: en(7), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_panel", Motivo: MotivoFueraBorde},
		Linea{T: en(8), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_panel", Motivo: MotivoCerca},
		Linea{T: en(9), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_panel", Motivo: MotivoLejos},
		Linea{T: en(9.5), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_panel"}, // sin motivo (registro anterior)
		Linea{T: en(10), Tipo: TipoEvitada, Sesion: "a", Resultado: "fuera_torso"},
	)
	r := Resumir(ls)
	if r.Evitadas != 6 || r.EvitadasPorMotivo["fuera_panel"] != 5 {
		t.Fatalf("evitadas %+v", r)
	}
	for motivo, quiere := range map[string]int{MotivoFueraBorde: 2, MotivoCerca: 1, MotivoLejos: 1} {
		if r.EvitadasPanelPorMotivo[motivo] != quiere {
			t.Errorf("%s: %d, quería %d (%v)", motivo, r.EvitadasPanelPorMotivo[motivo], quiere, r.EvitadasPanelPorMotivo)
		}
	}
	texto := r.Texto()
	if !strings.Contains(texto, "Salidas del panel: cerca 1, fuera_borde 2, lejos 1") {
		t.Fatalf("texto:\n%s", texto)
	}
}

func TestResumenPromediaCostoDePose(t *testing.T) {
	ls := uso("a", 0, 35)
	ls = append(ls,
		Linea{T: en(6), Tipo: TipoRendimiento, Sesion: "a", FPS: 25, CPUCore: 80, CPUTotal: 10, ManoMs: 12, PoseMs: 18},
		Linea{T: en(7), Tipo: TipoRendimiento, Sesion: "a", FPS: 27, CPUCore: 100, CPUTotal: 12, ManoMs: 10, PoseMs: 20},
	)
	r := Resumir(ls)
	if r.MuestrasRendimiento != 2 || r.FPSPromedio != 26 || r.CPUCorePromedio != 90 || r.PoseMsPromedio != 19 {
		t.Fatalf("rendimiento: %+v", r)
	}
	if texto := r.Texto(); !strings.Contains(texto, "26.0 cuadros/s") || !strings.Contains(texto, "Pose 19.0 ms") {
		t.Fatalf("texto:\n%s", texto)
	}
}
