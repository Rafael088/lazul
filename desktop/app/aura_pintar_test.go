package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestPintarAura(t *testing.T) {
	const lado = 360
	for _, agarrando := range []bool{false, true} {
		b := pintarAura(lado, 1.5, agarrando)
		if len(b) != lado*lado*4 {
			t.Fatalf("%d bytes", len(b))
		}
		at := func(x, y int) (bb, g, r, a byte) {
			i := (y*lado + x) * 4
			return b[i], b[i+1], b[i+2], b[i+3]
		}
		// Esquinas transparentes: la superficie no tapa nada fuera del halo.
		if _, _, _, a := at(0, 0); a != 0 {
			t.Errorf("agarrando=%v: esquina con alfa %d", agarrando, a)
		}
		// Centro azul y visible; premultiplicado: ningún canal pasa al alfa.
		bb, g, r, a := at(lado/2, lado/2)
		if a < 100 || bb > a || g > a || r > a {
			t.Errorf("agarrando=%v: centro BGRA %d,%d,%d,%d", agarrando, bb, g, r, a)
		}
		if !agarrando && bb <= r {
			t.Errorf("el halo suave es azul: BGRA %d,%d,%d,%d", bb, g, r, a)
		}
		if ruta := os.Getenv("LAZUL_AURA_PNG"); ruta != "" {
			img := image.NewNRGBA(image.Rect(0, 0, lado, lado))
			for y := 0; y < lado; y++ {
				for x := 0; x < lado; x++ {
					bb, g, r, a := at(x, y)
					c := color.NRGBA{A: a}
					if a > 0 {
						c.R, c.G, c.B = byte(int(r)*255/int(a)), byte(int(g)*255/int(a)), byte(int(bb)*255/int(a))
					}
					img.SetNRGBA(x, y, c)
				}
			}
			n := ruta + "-suave.png"
			if agarrando {
				n = ruta + "-vivo.png"
			}
			f, _ := os.Create(n)
			png.Encode(f, img)
			f.Close()
		}
	}
}
