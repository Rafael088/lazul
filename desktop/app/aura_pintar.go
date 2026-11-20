package main

import "math"

// Dibujo del aura sin navegador, para Windows: la misma figura que
// frontend/aura.html (halo radial Lapis sobre Noche y anillo fino), pintada
// una vez por estado en un mapa BGRA premultiplicado que la ventana por capas
// solo mueve. Las medidas están en píxeles CSS; escala pasa a píxeles físicos.

// ladoAura es el lado de la superficie en píxeles CSS (como LAZUL_AURA_LADO).
const ladoAura = 240

type colorAura struct{ r, g, b, a float64 } // r, g, b en 0..255; a en 0..1

type parada struct {
	en float64 // fracción del radio del degradado
	c  colorAura
}

var (
	lapis = colorAura{49, 95, 220, 1}
	noche = colorAura{16, 20, 40, 1}
	hielo = colorAura{232, 236, 255, 1}

	haloSuave = []parada{{0, conA(lapis, .58)}, {.42, conA(lapis, .30)}, {.62, conA(noche, .12)}, {.72, conA(noche, 0)}}
	haloVivo  = []parada{{0, conA(hielo, .80)}, {.34, conA(lapis, .62)}, {.56, conA(lapis, .22)}, {.74, conA(noche, 0)}}
)

func conA(c colorAura, a float64) colorAura { c.a = a; return c }

// px es un píxel premultiplicado en 0..1.
type px struct{ r, g, b, a float64 }

func premult(c colorAura) px {
	return px{c.r / 255 * c.a, c.g / 255 * c.a, c.b / 255 * c.a, c.a}
}

// sobre compone s encima de d (los dos premultiplicados).
func sobre(d, s px) px {
	k := 1 - s.a
	return px{s.r + d.r*k, s.g + d.g*k, s.b + d.b*k, s.a + d.a*k}
}

// degradado evalúa un radial-gradient en t (fracción del radio), con la
// interpolación premultiplicada que usa CSS.
func degradado(ps []parada, t float64) px {
	if t <= ps[0].en {
		return premult(ps[0].c)
	}
	for i := 1; i < len(ps); i++ {
		if t <= ps[i].en {
			a, b := premult(ps[i-1].c), premult(ps[i].c)
			f := (t - ps[i-1].en) / (ps[i].en - ps[i-1].en)
			return px{a.r + (b.r-a.r)*f, a.g + (b.g-a.g)*f, a.b + (b.b-a.b)*f, a.a + (b.a-a.a)*f}
		}
	}
	return premult(ps[len(ps)-1].c)
}

// brillo es la sombra difusa de CSS (box-shadow con desenfoque b) a una
// distancia d del borde, como una gaussiana de sigma b/2.
func brillo(d, b float64) float64 {
	if b <= 0 {
		return 0
	}
	s := b / 2
	return math.Exp(-d * d / (2 * s * s))
}

// pintarAura devuelve lado×lado píxeles BGRA premultiplicados, de arriba
// abajo, con el aura centrada. agarrando enciende el halo y agranda el anillo.
func pintarAura(lado int, escala float64, agarrando bool) []byte {
	// Halo: caja de 144 px con border-radius 50 %; el degradado «circle»
	// llega a la esquina más lejana (72·√2).
	radioHalo, paradas := 72.0, haloSuave
	// Anillo: 60 px por fuera con 2 px de borde y sus dos sombras.
	radioAro, borde := 30.0, 2.0
	colorBorde := colorAura{127, 168, 255, .78}
	type sombra struct {
		c      colorAura
		b      float64
		dentro bool
	}
	sombras := []sombra{{conA(lapis, .55), 12, false}, {conA(lapis, .35), 8, true}}
	if agarrando {
		radioHalo, paradas = 72*1.35, haloVivo
		radioAro, borde = 30*1.45, 2*1.45
		colorBorde = colorAura{232, 236, 255, .96}
		sombras = []sombra{{conA(lapis, .95), 22 * 1.45, false}, {conA(hielo, .9), 7 * 1.45, false}}
	}
	radioDeg := radioHalo * math.Sqrt2

	out := make([]byte, lado*lado*4)
	centro := float64(lado) / 2
	for y := 0; y < lado; y++ {
		for x := 0; x < lado; x++ {
			// Distancia al centro en píxeles CSS, en el centro del píxel.
			dx, dy := (float64(x)+.5-centro)/escala, (float64(y)+.5-centro)/escala
			r := math.Hypot(dx, dy)
			var p px
			if r <= radioHalo {
				h := degradado(paradas, r/radioDeg)
				// Medio píxel de suavizado en el borde del círculo.
				if f := (radioHalo - r) * escala; f < 1 {
					h = px{h.r * f, h.g * f, h.b * f, h.a * f}
				}
				p = h
			}
			for _, s := range sombras {
				var d float64
				switch {
				case s.dentro && r < radioAro-borde:
					d = radioAro - borde - r
				case !s.dentro && r > radioAro:
					d = r - radioAro
				default:
					continue
				}
				if a := s.c.a * brillo(d, s.b); a > 0 {
					p = sobre(p, premult(conA(s.c, a)))
				}
			}
			// Borde del anillo con suavizado de un píxel físico.
			fuera := (r - radioAro) * escala
			dentro := (radioAro - borde - r) * escala
			if cob := math.Min(math.Max(.5-fuera, 0), 1) * math.Min(math.Max(.5-dentro, 0), 1); cob > 0 {
				p = sobre(p, premult(conA(colorBorde, colorBorde.a*cob)))
			}
			i := (y*lado + x) * 4
			out[i+0] = byte(math.Round(math.Min(p.b, 1) * 255))
			out[i+1] = byte(math.Round(math.Min(p.g, 1) * 255))
			out[i+2] = byte(math.Round(math.Min(p.r, 1) * 255))
			out[i+3] = byte(math.Round(math.Min(p.a, 1) * 255))
		}
	}
	return out
}
