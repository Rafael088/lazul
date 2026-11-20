//go:build linux

package linux

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

// Salida recortada de hyprctl monitors -j en este equipo: el portátil a
// escala 2 a la izquierda y la externa a escala 1 a su derecha.
const monitoresJSON = `[
 {"id":0,"name":"eDP-1","width":2880,"height":1800,"x":0,"y":0,"scale":2.00,"transform":0,"focused":false,"disabled":false,"mirrorOf":"none"},
 {"id":1,"name":"DP-1","width":1920,"height":1080,"x":1440,"y":0,"scale":1.00,"transform":0,"focused":true,"disabled":false,"mirrorOf":"none"}
]`

func TestLeerMonitoresHyprland(t *testing.T) {
	ds, err := leerMonitoresHyprland([]byte(monitoresJSON))
	if err != nil {
		t.Fatal(err)
	}
	quiere := []platform.Display{
		{Name: "eDP-1", X: 0, Y: 0, Width: 1440, Height: 900},
		{Name: "DP-1", X: 1440, Y: 0, Width: 1920, Height: 1080, Focused: true},
	}
	if len(ds) != len(quiere) {
		t.Fatalf("ds = %+v", ds)
	}
	for i := range ds {
		if ds[i] != quiere[i] {
			t.Errorf("monitor %d = %+v, quiere %+v", i, ds[i], quiere[i])
		}
	}
}

func TestLeerMonitoresHyprlandGiradoApagadoEspejo(t *testing.T) {
	ds, err := leerMonitoresHyprland([]byte(`[
 {"name":"DP-2","width":2560,"height":1440,"x":-1440,"y":0,"scale":1,"transform":1},
 {"name":"HDMI-A-1","width":1920,"height":1080,"x":0,"y":0,"scale":1,"disabled":true},
 {"name":"DP-3","width":1920,"height":1080,"x":0,"y":0,"scale":1,"mirrorOf":"DP-2"}
]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0] != (platform.Display{Name: "DP-2", X: -1440, Width: 1440, Height: 2560}) {
		t.Fatalf("ds = %+v", ds)
	}
	if _, err := leerMonitoresHyprland([]byte("no es json")); err == nil {
		t.Fatal("aceptó una salida que no es JSON")
	}
}

func TestCodigoDirigir(t *testing.T) {
	c, err := codigoDirigir("DP-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, parte := range []string{`hl.device({ name = "lazul-tactil-virtual", output = "DP-1" })`, `hl.dsp.focus({ monitor = "DP-1" })`} {
		if !strings.Contains(c, parte) {
			t.Errorf("falta %s en %s", parte, c)
		}
	}
	for _, malo := range []string{"", `DP-1" }) os.execute("x`, "a b", strings.Repeat("x", 40)} {
		if _, err := codigoDirigir(malo); !errors.Is(err, platform.ErrAccionInvalida) {
			t.Errorf("codigoDirigir(%q) = %v, quiere ErrAccionInvalida", malo, err)
		}
	}
}

type tactilRecreado struct{ veces int }

func (r *tactilRecreado) Recrear() error { r.veces++; return nil }

func hyprctlFalso(eval *[]string, respuesta string, monitores string) func(context.Context, string, ...string) ([]byte, error) {
	return func(_ context.Context, programa string, args ...string) ([]byte, error) {
		if programa != "hyprctl" {
			return nil, errors.New("programa inesperado " + programa)
		}
		switch args[0] {
		case "monitors":
			return []byte(monitores), nil
		case "eval":
			*eval = append(*eval, args[1])
			return []byte(respuesta), nil
		}
		return nil, errors.New("orden inesperada")
	}
}

func TestTargetDirigeTactilPunteroYFoco(t *testing.T) {
	var eval []string
	puntero := &punteroVirtual{}
	tactil := &tactilRecreado{}
	p := pantallasHyprland{ejecutar: hyprctlFalso(&eval, "ok", monitoresJSON), puntero: puntero, tactil: tactil}
	if err := p.Target(context.Background(), "eDP-1"); err != nil {
		t.Fatal(err)
	}
	if len(eval) != 1 || !strings.Contains(eval[0], `output = "eDP-1"`) || !strings.Contains(eval[0], `monitor = "eDP-1"`) {
		t.Fatalf("eval = %q", eval)
	}
	if tactil.veces != 1 {
		t.Errorf("el táctil se recreó %d veces, quiere 1", tactil.veces)
	}
	if puntero.destino == nil || puntero.destino.Name != "eDP-1" || puntero.caja.Width != 3360 {
		t.Fatalf("puntero dirigido a %+v en %+v", puntero.destino, puntero.caja)
	}
	// El centro del portátil cae a 720/3360 del escritorio entero.
	x, y := platform.EnAbarque(*puntero.destino, puntero.caja, .5, .5)
	if x < 0.2142 || x > 0.2144 || y < 0.4166 || y > 0.4167 {
		t.Errorf("centro del portátil = (%v, %v)", x, y)
	}
}

func TestTargetUnSoloMonitorDejaElPunteroEntero(t *testing.T) {
	var eval []string
	puntero := &punteroVirtual{}
	uno := `[{"name":"eDP-1","width":2880,"height":1800,"x":0,"y":0,"scale":2,"focused":true}]`
	p := pantallasHyprland{ejecutar: hyprctlFalso(&eval, "", uno), puntero: puntero}
	if err := p.Target(context.Background(), "eDP-1"); err != nil {
		t.Fatal(err)
	}
	if puntero.destino != nil {
		t.Fatalf("con un monitor el puntero quedó dirigido a %+v", puntero.destino)
	}
}

func TestTargetErrores(t *testing.T) {
	var eval []string
	p := pantallasHyprland{ejecutar: hyprctlFalso(&eval, "ok", monitoresJSON)}
	if err := p.Target(context.Background(), "HDMI-A-9"); !errors.Is(err, platform.ErrAccionInvalida) {
		t.Errorf("monitor inexistente: %v", err)
	}
	if len(eval) != 0 {
		t.Errorf("con un monitor inexistente se mandó %q", eval)
	}
	p = pantallasHyprland{ejecutar: hyprctlFalso(&eval, "error: attempt to call a nil value", monitoresJSON), tactil: &tactilRecreado{}}
	if err := p.Target(context.Background(), "DP-1"); err == nil {
		t.Error("un error de Lua no llegó como error")
	}
}
