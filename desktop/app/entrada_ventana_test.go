package main

import (
	"testing"

	"github.com/Rafael088/lazul/desktop"
)

func TestRectPildoraDejaPasarLaVentanaTransparente(t *testing.T) {
	casos := []struct {
		nombre         string
		ancho, alto    int
		x0, y0, x1, y1 int
	}{
		{"indicador", desktop.AnchoIndicador, desktop.AltoIndicador, 6, 6, 230, 58},
		{"ventana grande atascada", 920, 660, 690, 602, 914, 654},
		{"ventana diminuta", 4, 3, 0, 0, 0, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			x0, y0, x1, y1 := rectPildora(c.ancho, c.alto)
			if x0 != c.x0 || y0 != c.y0 || x1 != c.x1 || y1 != c.y1 {
				t.Fatalf("rect = (%d,%d)-(%d,%d); quiero (%d,%d)-(%d,%d)", x0, y0, x1, y1, c.x0, c.y0, c.x1, c.y1)
			}
		})
	}
}
