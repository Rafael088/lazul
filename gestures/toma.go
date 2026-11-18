package gestures

import (
	"errors"
	"sort"
)

// Las tomas de fixtures/pellizco guardan el rectángulo del panel en la
// cabecera, pero no la calibración entera: falta el tamaño aparente de la
// mano, que la app mide al calibrar. Para volver a pasar una toma por el
// embrague se toma el tamaño de palma mediano de las propias tomas como
// referencia de profundidad; es lo que hacen las pruebas del embrague desde
// que existen las tomas reales.

// CalibracionDeToma reconstruye la calibración del panel con la que se grabó
// una toma. tamano es el tamaño de palma de referencia (TamanoMedianoPellizco
// de todas las tomas).
func CalibracionDeToma(g Grabacion, tamano float64) (CalibracionPanel, error) {
	p := g.Cabecera.Panel
	if p == nil {
		return CalibracionPanel{}, errors.New(g.Archivo + ": la cabecera no lleva panel calibrado")
	}
	if g.Cabecera.Ancho <= 0 || g.Cabecera.Alto <= 0 {
		return CalibracionPanel{}, errors.New(g.Archivo + ": la cabecera no lleva el tamaño de la imagen")
	}
	a := float64(g.Cabecera.Ancho) / float64(g.Cabecera.Alto)
	// La imagen llega sin espejar: la izquierda de la persona es la X mayor.
	return CalibracionPanel{
		Version:           1,
		SuperiorIzquierda: MuestraPanel{X: (p.X1 - 0.5) * a, Y: p.Y0 - 0.5, Tamano: tamano},
		InferiorDerecha:   MuestraPanel{X: (p.X0 - 0.5) * a, Y: p.Y1 - 0.5, Tamano: tamano},
	}, nil
}

// TamanoMedianoPellizco devuelve el tamaño de palma mediano de todas las
// manos de todas las tomas: la referencia de profundidad para reconstruir la
// calibración de cada una.
func TamanoMedianoPellizco(gs []Grabacion) float64 {
	var ts []float64
	for _, g := range gs {
		for _, c := range g.Cuadros {
			for _, m := range c.Manos {
				if mu, ok := MuestraPanelDe(m, c.Aspecto); ok {
					ts = append(ts, mu.Tamano)
				}
			}
		}
	}
	sort.Float64s(ts)
	if len(ts) == 0 {
		return 0
	}
	return ts[len(ts)/2]
}
