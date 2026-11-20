//go:build windows

package windows

import (
	"context"

	"github.com/Rafael088/lazul/platform"
)

// accionesTactiles implementa platform.TactileActions con SendInput: las
// teclas van a la ventana con foco, como en Linux (flechas, Ctrl + «+»/«-»,
// Re Pág/Av Pág). Es el respaldo del táctil virtual y el del modo de gestos.
type accionesTactiles struct{}

var (
	_ platform.TactileActions     = accionesTactiles{}
	_ platform.ContinuousScroller = accionesTactiles{}
)

// ScrollPulsaciones es cuántas flechas manda un gesto de scroll, como en Linux.
const ScrollPulsaciones = 3

// Teclas virtuales (winuser.h).
const (
	vkControl = 0x11
	vkPrior   = 0x21 // Re Pág
	vkNext    = 0x22 // Av Pág
	vkLeft    = 0x25
	vkUp      = 0x26
	vkRight   = 0x27
	vkDown    = 0x28
	vkOemMas  = 0xBB
	vkOemMen  = 0xBD
)

func extendida(vk uint16) bool {
	return vk >= vkPrior && vk <= vkDown
}

// pulsaciones arma bajada y subida de la tecla veces veces, con mods
// apretados alrededor de todas.
func pulsaciones(mods []uint16, vk uint16, veces int) []entradaTeclado {
	var es []entradaTeclado
	tecla := func(v uint16, soltar bool) {
		var f uint32
		if extendida(v) {
			f |= teclaExtendida
		}
		if soltar {
			f |= teclaSoltar
		}
		es = append(es, entradaTeclado{tipo: inputTeclado, vk: v, flags: f})
	}
	for _, m := range mods {
		tecla(m, false)
	}
	for range veces {
		tecla(vk, false)
		tecla(vk, true)
	}
	for i := len(mods) - 1; i >= 0; i-- {
		tecla(mods[i], true)
	}
	return es
}

func (accionesTactiles) Scroll(ctx context.Context, direction platform.ScrollDirection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch direction {
	case platform.ScrollUp:
		return enviarTeclado(pulsaciones(nil, vkUp, ScrollPulsaciones))
	case platform.ScrollDown:
		return enviarTeclado(pulsaciones(nil, vkDown, ScrollPulsaciones))
	}
	return platform.ErrAccionInvalida
}

func (accionesTactiles) Zoom(ctx context.Context, direction platform.ZoomDirection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch direction {
	case platform.ZoomIn:
		return enviarTeclado(pulsaciones([]uint16{vkControl}, vkOemMas, 1))
	case platform.ZoomOut:
		return enviarTeclado(pulsaciones([]uint16{vkControl}, vkOemMen, 1))
	}
	return platform.ErrAccionInvalida
}

func teclaNavegacion(direction platform.NavigationDirection, mode platform.NavigationMode) (uint16, bool) {
	izq, der := uint16(vkLeft), uint16(vkRight)
	switch mode {
	case platform.NavigationNormal:
	case platform.NavigationSlides:
		izq, der = vkPrior, vkNext
	default:
		return 0, false
	}
	switch direction {
	case platform.NavigateLeft:
		return izq, true
	case platform.NavigateRight:
		return der, true
	}
	return 0, false
}

func (accionesTactiles) Navigate(ctx context.Context, direction platform.NavigationDirection, mode platform.NavigationMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	vk, ok := teclaNavegacion(direction, mode)
	if !ok {
		return platform.ErrAccionInvalida
	}
	return enviarTeclado(pulsaciones(nil, vk, 1))
}

// ScrollContinuo mueve la rueda: en Windows 120 unidades son un paso, igual
// que en platform. Positivo hacia abajo, así que va con el signo cambiado.
func (accionesTactiles) ScrollContinuo(ctx context.Context, unidades int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if unidades == 0 {
		return nil
	}
	return enviarRaton([]entradaRaton{{tipo: inputRaton, datos: uint32(int32(-unidades)), flags: ratonRueda}})
}
