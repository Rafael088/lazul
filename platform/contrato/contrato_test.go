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

// appMal devuelve niveles mal formados (URL relativa, archivo relativo).
type appMal struct{ app platform.App }

func (a appMal) Active(ctx context.Context) (platform.App, error) {
	if err := ctx.Err(); err != nil {
		return platform.App{}, err
	}
	return a.app, nil
}

// La regla de los niveles distingue una URL o un archivo mal formados.
func TestNivelesMalFormadosFallan(t *testing.T) {
	malos := []platform.App{
		{ID: "x", Nombre: "x", URL: "example.org/sin-esquema"},
		{ID: "x", Nombre: "x", Archivo: "relativo/informe.pdf"},
		{ID: "x", Nombre: "x", Archivo: "/lazul-contrato-no-existe/informe.pdf"},
	}
	for _, app := range malos {
		s := platform.Sistema{ActiveApp: appMal{app}}
		for _, p := range contrato.Pruebas(s, opciones) {
			if p.Nombre == "ActiveApp/niveles_bien_formados" && p.Correr(context.Background()) == nil {
				t.Errorf("pasó con %+v", app)
			}
		}
	}
}
