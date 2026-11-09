package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

const fixtures = "../fixtures/gestos"

// pasar mete los cuadros en v con la hora de pared siguiendo al reloj de la cámara.
func pasar(t *testing.T, v *Vivo, cs []gestures.Cuadro, t0 time.Time) []Vista {
	t.Helper()
	var vs []Vista
	for _, c := range cs {
		vs = append(vs, v.Cuadro(c, t0.Add(time.Duration(c.TMs)*time.Millisecond)))
	}
	return vs
}

func recorrido(t *testing.T, nombres string) []gestures.Cuadro {
	t.Helper()
	gs, err := LeerRecorrido(fixtures, nombres)
	if err != nil {
		t.Fatal(err)
	}
	return Encadenar(gs, 400)
}

func lineas(t *testing.T, ruta string) []Linea {
	t.Helper()
	ls, err := LeerRegistroDe(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return ls
}

func deTipo(ls []Linea, tipo string) []Linea {
	return slices.DeleteFunc(slices.Clone(ls), func(l Linea) bool { return l.Tipo != tipo })
}

func TestRecorridoCompletoDisparaAgarrarYSoltar(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, err := AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var disparos []Disparo
	v := &Vivo{Reg: reg, AlGesto: func(d Disparo) { disparos = append(disparos, d) }}
	t0 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	vs := pasar(t, v, recorrido(t, "activar-buena-002,agarrar-buena-003,soltar-buena-003"), t0)
	v.Parar(t0.Add(time.Minute))
	reg.Cerrar()

	var gestos []lazulv1.Gesto
	for _, d := range disparos {
		gestos = append(gestos, d.Evento.Gesto)
		if d.Activacion != 1 || d.Prueba {
			t.Errorf("disparo %v: activación %d prueba %v", d.Evento.Gesto, d.Activacion, d.Prueba)
		}
	}
	if !slices.Equal(gestos, []lazulv1.Gesto{lazulv1.Gesto_GESTO_AGARRAR, lazulv1.Gesto_GESTO_SOLTAR}) {
		t.Fatalf("disparos %v, quería agarrar y soltar", gestos)
	}
	if u := vs[len(vs)-1]; !strings.HasPrefix(u.Ultimo, "SOLTAR") || u.Activacion != 1 {
		t.Errorf("vista final %+v", u)
	}
	ls := lineas(t, ruta)
	acts := deTipo(ls, TipoActivacion)
	if len(acts) != 1 || acts[0].Resultado != ActCaptura {
		t.Fatalf("activaciones %+v, quería una que terminó en captura", acts)
	}
	if len(deTipo(ls, TipoInicio)) != 1 || len(deTipo(ls, TipoFin)) != 1 || len(deTipo(ls, TipoPose)) == 0 {
		t.Errorf("faltan inicio, fin o poses: %d líneas", len(ls))
	}
}

func TestActivacionSinGestoVence(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, _ := AbrirRegistro(ruta)
	v := &Vivo{Reg: reg}
	cs := recorrido(t, "activar-buena-002")
	// 6 s sin mano: vence el modo activo (5 s).
	ult := cs[len(cs)-1].TMs
	for tt := ult; tt < ult+6000; tt += 33 {
		cs = append(cs, gestures.Cuadro{TMs: tt})
	}
	pasar(t, v, cs, time.Now())
	reg.Cerrar()
	acts := deTipo(lineas(t, ruta), TipoActivacion)
	if len(acts) != 1 || acts[0].Resultado != ActSinGesto {
		t.Fatalf("activaciones %+v, quería una sin gesto", acts)
	}
}

func TestPruebaNoMandaYMarca(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, _ := AbrirRegistro(ruta)
	var disparos []Disparo
	v := &Vivo{Reg: reg, AlGesto: func(d Disparo) { disparos = append(disparos, d) }}
	if _, err := v.Marcar(true, time.Now()); err == nil {
		t.Error("marcar sin activaciones debería fallar")
	}
	t0 := time.Now()
	v.Prueba(true, t0)
	pasar(t, v, recorrido(t, "activar-buena-002,agarrar-buena-003,soltar-buena-003"), t0)
	if n, err := v.Marcar(true, time.Now()); err != nil || n != 1 {
		t.Errorf("marcar: %d %v", n, err)
	}
	v.Prueba(false, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()
	for _, d := range disparos {
		if !d.Prueba {
			t.Errorf("en la prueba, el disparo %v debería ir marcado", d.Evento.Gesto)
		}
	}
	ls := lineas(t, ruta)
	if a := deTipo(ls, TipoActivacion); len(a) != 1 || !a[0].Prueba {
		t.Errorf("activación %+v", a)
	}
	if m := deTipo(ls, TipoMarca); len(m) != 1 || m[0].Falsa == nil || !*m[0].Falsa {
		t.Errorf("marca %+v", m)
	}
	if p := deTipo(ls, TipoPrueba); len(p) != 2 {
		t.Errorf("prueba: %+v", p)
	}
}

// El registro no lleva puntos de la mano, imágenes, URL ni rutas: solo
// estas claves, y ninguna con texto libre que venga de la pantalla.
func TestRegistroSinDatosDeCamaraNiPantalla(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, _ := AbrirRegistro(ruta)
	v := &Vivo{Reg: reg}
	gs, err := gestures.LeerDirectorio(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	pasar(t, v, Encadenar(gs[:20], 300), time.Now())
	v.Parar(time.Now())
	reg.Cerrar()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	permitidas := map[string]bool{"t": true, "tipo": true, "sesion": true, "pose": true, "modo": true, "gesto": true, "dir": true,
		"conf": true, "act": true, "resultado": true, "que": true, "destino": true, "error": true, "ms": true, "cuadros": true,
		"con_mano": true, "prueba": true, "falsa": true}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		for k, val := range m {
			if !permitidas[k] {
				t.Errorf("clave %q en el registro: %s", k, l)
			}
			if _, esLista := val.([]any); esLista {
				t.Errorf("lista en el registro (¿puntos?): %s", l)
			}
		}
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(ruta); info.Mode().Perm() != 0o600 {
			t.Errorf("permisos %v, quería 0600", info.Mode().Perm())
		}
	}
}
