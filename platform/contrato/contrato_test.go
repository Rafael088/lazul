package contrato_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/contrato"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/platform/vacio"
)

var opciones = contrato.Opciones{AppInofensiva: "calculadora", Abrir: true}

// Una implementación vacía no puede pasar ninguna prueba: si alguna pasa,
// esa prueba no está comprobando nada.
func TestVacioFallaTodo(t *testing.T) {
	for _, p := range contrato.Pruebas(vacio.New(), opciones) {
		if err := p.Correr(context.Background()); err == nil || errors.Is(err, contrato.ErrOmitida) {
			t.Errorf("%s pasó con la implementación vacía (err=%v)", p.Nombre, err)
		}
	}
}

// Tampoco puede pasar un Sistema sin piezas.
func TestNilFalla(t *testing.T) {
	for _, p := range contrato.Pruebas(platform.Sistema{}, opciones) {
		if p.Correr(context.Background()) == nil {
			t.Errorf("%s pasó con un Sistema sin piezas", p.Nombre)
		}
	}
}

// El contrato es cumplible: el sistema falso lo pasa entero.
func TestFalsoCumple(t *testing.T) {
	contrato.Correr(t, falso.New("prueba", "calculadora").Sistema(), opciones)
}
