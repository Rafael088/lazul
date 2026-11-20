//go:build windows

package windows

import (
	"context"
	"math"

	"github.com/Rafael088/lazul/platform"
)

// punteroAbsoluto implementa platform.AbsolutePointer con SendInput: el
// movimiento absoluto va de 0 a 65535 sobre la pantalla principal (la misma
// que recibe los toques), así no depende del DPI.
type punteroAbsoluto struct{}

var _ platform.AbsolutePointer = punteroAbsoluto{}

func (punteroAbsoluto) Prepare(ctx context.Context) error { return ctx.Err() }

func coordenadaAbsoluta(v float64) (int32, bool) {
	if math.IsNaN(v) || v < 0 || v > 1 {
		return 0, false
	}
	return int32(math.Round(v * rangoAbsoluto)), true
}

func (punteroAbsoluto) MoveAbsolute(ctx context.Context, x, y float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dx, okx := coordenadaAbsoluta(x)
	dy, oky := coordenadaAbsoluta(y)
	if !okx || !oky {
		return platform.ErrAccionInvalida
	}
	return enviarRaton([]entradaRaton{{tipo: inputRaton, dx: dx, dy: dy, flags: ratonMover | ratonAbsoluto}})
}

func (punteroAbsoluto) Button(ctx context.Context, down bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := uint32(ratonIzqArriba)
	if down {
		flags = ratonIzqAbajo
	}
	return enviarRaton([]entradaRaton{{tipo: inputRaton, flags: flags}})
}
