package desktop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

type accesibilidadFalsa struct {
	res      platform.AccessibilityResult
	err      error
	llamadas int
}

func (a *accesibilidadFalsa) Enable(context.Context) (platform.AccessibilityResult, error) {
	a.llamadas++
	return a.res, a.err
}

func TestPrepararAccesibilidad(t *testing.T) {
	casos := []struct {
		nombre    string
		cap       platform.Accessibility
		usable    bool
		encendida bool
		aviso     string
		resultado string // Resultado de la línea del registro; "" = sin línea
		errorLine string
	}{
		{
			nombre:    "la enciende",
			cap:       &accesibilidadFalsa{res: platform.AccessibilityResult{TurnedOn: true}},
			usable:    true,
			encendida: true,
			aviso:     AvisoReiniciarNavegadores,
			resultado: "encendida",
		},
		{
			nombre:    "ya estaba",
			cap:       &accesibilidadFalsa{res: platform.AccessibilityResult{WasOn: true}},
			usable:    true,
			resultado: "ya_estaba",
		},
		{
			nombre:    "sin bus",
			cap:       &accesibilidadFalsa{err: fmt.Errorf("sin servicio: %w", platform.ErrSinAccesibilidad)},
			usable:    false,
			aviso:     "sin accesibilidad de la sesión (¿falta at-spi2-core?): el imán queda apagado",
			resultado: "sin_bus",
			errorLine: "sin_accesibilidad",
		},
		{
			nombre:    "otro error: el imán sigue",
			cap:       &accesibilidadFalsa{err: errors.New("el bus contestó tarde")},
			usable:    true,
			aviso:     "no se pudo preparar la accesibilidad de la sesión: el bus contestó tarde",
			resultado: "error",
			errorLine: "otro",
		},
		{
			nombre: "sin capacidad",
			cap:    nil,
			usable: true,
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			ruta := filepath.Join(t.TempDir(), "registro.jsonl")
			reg, err := AbrirRegistro(ruta)
			if err != nil {
				t.Fatal(err)
			}
			defer reg.Cerrar()
			r := PrepararAccesibilidad(context.Background(), c.cap, reg)
			if r.Usable != c.usable || r.Encendida != c.encendida || r.Aviso != c.aviso {
				t.Errorf("resultado = %+v, quiero usable=%v encendida=%v aviso=%q", r, c.usable, c.encendida, c.aviso)
			}
			lineas, err := LeerRegistroDe(ruta)
			if err != nil {
				t.Fatal(err)
			}
			if c.resultado == "" {
				if len(lineas) != 0 {
					t.Fatalf("líneas = %+v, no quería ninguna", lineas)
				}
				return
			}
			if len(lineas) != 1 {
				t.Fatalf("líneas = %+v, quería una", lineas)
			}
			l := lineas[0]
			if l.Tipo != TipoAccesibilidad || l.Resultado != c.resultado || l.Error != c.errorLine {
				t.Errorf("línea = %+v, quiero tipo=%s resultado=%s error=%q", l, TipoAccesibilidad, c.resultado, c.errorLine)
			}
		})
	}
}

// La capacidad se llama una sola vez y solo si existe.
func TestPrepararAccesibilidadLlamaUnaVez(t *testing.T) {
	f := &accesibilidadFalsa{res: platform.AccessibilityResult{WasOn: true}}
	reg, err := AbrirRegistro(filepath.Join(t.TempDir(), "registro.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Cerrar()
	PrepararAccesibilidad(context.Background(), f, reg)
	if f.llamadas != 1 {
		t.Errorf("Enable llamado %d veces, quiero 1", f.llamadas)
	}
}
