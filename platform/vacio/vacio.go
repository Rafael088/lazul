// Package vacio es la implementación que no hace nada: todo devuelve
// platform.ErrNoImplementado. Sirve de punto de partida para un sistema
// nuevo y para comprobar que el contrato no pasa solo.
package vacio

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

type vacio struct{}

// New devuelve un Sistema en el que nada está implementado.
func New() platform.Sistema {
	v := vacio{}
	return platform.Sistema{Screenshot: v, Clipboard: v, ActiveApp: v, LaunchApp: v}
}

func (vacio) Capture(context.Context) (platform.Imagen, error) {
	return platform.Imagen{}, platform.ErrNoImplementado
}

func (vacio) ReadText(context.Context) (string, error) { return "", platform.ErrNoImplementado }

func (vacio) WriteText(context.Context, string) error { return platform.ErrNoImplementado }

func (vacio) Active(context.Context) (platform.App, error) {
	return platform.App{}, platform.ErrNoImplementado
}

func (vacio) Launch(context.Context, platform.AppRef) error { return platform.ErrNoImplementado }
