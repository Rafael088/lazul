package main

import "math"

// Dibujo del aura sin navegador, para Windows: la misma figura que
// frontend/aura.html (aro blanco chico con un contorno oscuro fino; relleno
// y más chico al pellizcar), pintada una vez por estado en un mapa BGRA
// premultiplicado que la ventana por capas solo mueve. Las medidas están en
// píxeles CSS; escala pasa a píxeles físicos.

// ladoAura es el lado de la superficie en píxeles CSS (como LAZUL_AURA_LADO).
const ladoAura = 48

// px es un píxel premultiplicado en 0..1.
type px struct{ r, g, b, a float64 }

// blanco y oscuro con alfa a, ya premultiplicados.
func blanco(a float64) px { return px{a, a, a, a} }
func oscuro(a float64) px { return px{0, 0, 0, a} }

// sobre compone s encima de d (los dos premultiplicados).
func sobre(d, s px) px {
	k := 1 - s.a
	return px{s.r + d.r*k, s.g + d.g*k, s.b + d.b*k, s.a + d.a*k}
}

func escalar(p px, f float64) px { return px{p.r * f, p.g * f, p.b * f, p.a * f} }

// cobertura de la corona entre r0 y r1 para un píxel a distancia r, con un
// píxel físico de suavizado en cada borde.
func cobertura(r, r0, r1, escala float64) float64 {
	dentro := math.Min(math.Max((r-r0)*escala+.5, 0), 1)
	fuera := math.Min(math.Max((r1-r)*escala+.5, 0), 1)
	return dentro * fuera
}

// pintarAura devuelve lado×lado píxeles BGRA premultiplicados, de arriba
// abajo, con el aro centrado. agarrando lo encoge y lo rellena.
func pintarAura(lado int, escala float64, agarrando bool) []byte {
	// Aro: caja de 18 px con borde de 2 px (radio exterior 9). Al pellizcar
	// la caja se escala a 0,72 y se rellena.
	radio, borde, relleno := 9.0, 2.0, 0.0
	if agarrando {
		radio, borde, relleno = 9*.72, 2*.72, .92
	}
	out := make([]byte, lado*lado*4)
	centro := float64(lado) / 2
	for y := 0; y < lado; y++ {
		for x := 0; x < lado; x++ {
			dx, dy := (float64(x)+.5-centro)/escala, (float64(y)+.5-centro)/escala
			r := math.Hypot(dx, dy)
			var p px
			// Sombra difusa debajo (0 1px 3px) y contorno de 1 px fuera del aro.
			if d := r - radio; d > -1 && d < 4 {
				s := math.Exp(-math.Max(d, 0) * math.Max(d, 0) / (2 * 1.5 * 1.5))
				p = sobre(p, oscuro(.30*s*cobertura(r, radio-1, radio+4, escala)))
			}
			p = sobre(p, oscuro(.28*cobertura(r, radio, radio+1, escala)))
			if relleno > 0 {
				p = sobre(p, blanco(relleno*cobertura(r, -1, radio-borde/2, escala)))
			}
			p = sobre(p, blanco(.92*cobertura(r, radio-borde, radio, escala)))
			i := (y*lado + x) * 4
			out[i+0] = byte(math.Round(math.Min(p.b, 1) * 255))
			out[i+1] = byte(math.Round(math.Min(p.g, 1) * 255))
			out[i+2] = byte(math.Round(math.Min(p.r, 1) * 255))
			out[i+3] = byte(math.Round(math.Min(p.a, 1) * 255))
		}
	}
	return out
}
