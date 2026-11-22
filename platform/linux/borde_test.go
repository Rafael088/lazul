//go:build linux

package linux

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLeerResizeOnBorder(t *testing.T) {
	casos := []struct {
		nombre string
		salida string
		quiere bool
		mal    bool
	}{
		{"texto encendido", "int: 1\nset: true\n", true, false},
		{"texto apagado", "int: 0\nset: true\n", false, false},
		{"texto con option", "option: general:resize_on_border\nint: 1\nset: true\n", true, false},
		{"json int", `{"option":"general:resize_on_border","int":1}`, true, false},
		{"json bool", `{"option":"general:resize_on_border","bool":false}`, false, false},
		{"asignación", "general:resize_on_border = 1", true, false},
		{"vacío", "", false, true},
		{"basura", "no sé", false, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			v, err := leerResizeOnBorder([]byte(c.salida))
			if c.mal {
				if err == nil {
					t.Fatalf("aceptó %q y devolvió %v", c.salida, v)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v != c.quiere {
				t.Errorf("leerResizeOnBorder(%q) = %v, quiere %v", c.salida, v, c.quiere)
			}
		})
	}
}

// hyprctlBorde es un hyprctl falso; guarda las órdenes y cambia el valor
// cuando ve el hl.config.
type hyprctlBorde struct {
	valor    bool
	llamadas [][]string
	errLeer  error
	errEval  error
	evalRaro string
}

func (h *hyprctlBorde) ejecutar(_ context.Context, programa string, args ...string) ([]byte, error) {
	h.llamadas = append(h.llamadas, append([]string{programa}, args...))
	if programa != "hyprctl" {
		return nil, errors.New("programa inesperado " + programa)
	}
	switch args[0] {
	case "getoption":
		if h.errLeer != nil {
			return nil, h.errLeer
		}
		if h.valor {
			return []byte("int: 1\nset: true\n"), nil
		}
		return []byte("int: 0\nset: true\n"), nil
	case "eval":
		if h.errEval != nil {
			return nil, h.errEval
		}
		if h.evalRaro != "" {
			return []byte(h.evalRaro), nil
		}
		switch {
		case strings.Contains(args[1], "resize_on_border = false"):
			h.valor = false
		case strings.Contains(args[1], "resize_on_border = true"):
			h.valor = true
		default:
			return nil, errors.New("el Lua no cambia resize_on_border")
		}
		return []byte("ok"), nil
	}
	return nil, errors.New("orden inesperada")
}

func TestBordeHyprlandLeeYCambiaConHyprctl(t *testing.T) {
	h := &hyprctlBorde{valor: true}
	b := bordeHyprland{ejecutar: h.ejecutar}
	v, err := b.ResizeOnBorder(context.Background())
	if err != nil || !v {
		t.Fatalf("ResizeOnBorder = %v, %v; quiere true", v, err)
	}
	if err := b.SetResizeOnBorder(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if h.valor {
		t.Error("el valor no se apagó")
	}
	if err := b.SetResizeOnBorder(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if !h.valor {
		t.Error("el valor no se encendió")
	}
	quiere := [][]string{
		{"hyprctl", "getoption", "general:resize_on_border"},
		{"hyprctl", "eval", "hl.config({ general = { resize_on_border = false } })"},
		{"hyprctl", "eval", "hl.config({ general = { resize_on_border = true } })"},
	}
	if len(h.llamadas) != len(quiere) {
		t.Fatalf("llamadas = %q", h.llamadas)
	}
	for i := range quiere {
		if strings.Join(h.llamadas[i], " ") != strings.Join(quiere[i], " ") {
			t.Errorf("llamada %d = %q, quiere %q", i, h.llamadas[i], quiere[i])
		}
	}
}

func TestBordeHyprlandErrores(t *testing.T) {
	if _, err := (bordeHyprland{ejecutar: (&hyprctlBorde{errLeer: errors.New("sin Hyprland")}).ejecutar}).ResizeOnBorder(context.Background()); err == nil || !strings.Contains(err.Error(), "sin Hyprland") {
		t.Errorf("error al leer = %v", err)
	}
	if err := (bordeHyprland{ejecutar: (&hyprctlBorde{errEval: errors.New("hyprctl colgado")}).ejecutar}).SetResizeOnBorder(context.Background(), false); err == nil || !strings.Contains(err.Error(), "hyprctl colgado") {
		t.Errorf("error al cambiar = %v", err)
	}
	h := &hyprctlBorde{evalRaro: "hl.config: valor no válido"}
	if err := (bordeHyprland{ejecutar: h.ejecutar}).SetResizeOnBorder(context.Background(), false); err == nil || !strings.Contains(err.Error(), "valor no válido") {
		t.Errorf("un error de Lua pasó como bueno: %v", err)
	}
}

// Con el contexto ya cancelado no se llama a hyprctl.
func TestBordeHyprlandContextoCancelado(t *testing.T) {
	h := &hyprctlBorde{valor: true}
	b := bordeHyprland{ejecutar: h.ejecutar}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.ResizeOnBorder(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("ResizeOnBorder con contexto cancelado = %v", err)
	}
	if err := b.SetResizeOnBorder(ctx, false); !errors.Is(err, context.Canceled) {
		t.Errorf("SetResizeOnBorder con contexto cancelado = %v", err)
	}
	if len(h.llamadas) != 0 {
		t.Errorf("llamó a hyprctl con el contexto cancelado: %q", h.llamadas)
	}
}
