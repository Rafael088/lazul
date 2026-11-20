package main

import (
	"math"

	"github.com/Rafael088/lazul/platform"
)

// EstadoAura es lo que el halo pinta en cada cuadro. La posición llega en
// coordenadas del panel (0..1); la página la escala a la pantalla.
type EstadoAura struct {
	Activa    bool    `json:"activa"`    // modo táctil con la vista de cámara cerrada
	Calibrado bool    `json:"calibrado"` // el panel ya tiene dos esquinas guardadas
	Dentro    bool    `json:"dentro"`    // la mano está dentro del panel calibrado
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Agarrando bool    `json:"agarrando"` // pellizco cerrado (lo expone cotejo-l9dyjg)
}

// posicionAura da la esquina superior izquierda de una superficie de lado
// píxeles centrada en la mano, dentro del monitor m (en las coordenadas del
// escritorio). La usa el aura de Windows; la de Linux la calcula GTK.
func posicionAura(e EstadoAura, m platform.Display, lado int32) (int32, int32) {
	x := int32(m.X) + int32(math.Round(e.X*float64(m.Width-1))) - lado/2
	y := int32(m.Y) + int32(math.Round(e.Y*float64(m.Height-1))) - lado/2
	return x, y
}
