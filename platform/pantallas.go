package platform

// Funciones puras sobre los monitores que comparten los adaptadores y la app.

// Enfocado devuelve el monitor con el foco; si ninguno lo marca, el primero.
func Enfocado(ds []Display) (Display, bool) {
	for _, d := range ds {
		if d.Focused {
			return d, true
		}
	}
	if len(ds) == 0 {
		return Display{}, false
	}
	return ds[0], true
}

// PorNombre busca el monitor llamado nombre.
func PorNombre(ds []Display, nombre string) (Display, bool) {
	for _, d := range ds {
		if d.Name == nombre {
			return d, true
		}
	}
	return Display{}, false
}

// Vecino devuelve el monitor más cercano al lado pedido de actual: su centro
// tiene que quedar a ese lado. Entre dos a la misma distancia gana el que más
// se solapa en vertical. Con un solo monitor no hay vecino.
func Vecino(ds []Display, actual string, izquierda bool) (Display, bool) {
	a, ok := PorNombre(ds, actual)
	if !ok {
		return Display{}, false
	}
	ca := 2*a.X + a.Width // dos veces el centro: sin fracciones
	var mejor Display
	hay := false
	mejorDist, mejorSolape := 0, 0
	for _, d := range ds {
		if d.Name == a.Name || d.Width <= 0 || d.Height <= 0 {
			continue
		}
		dist := 2*d.X + d.Width - ca
		if izquierda {
			dist = -dist
		}
		if dist <= 0 {
			continue
		}
		solape := min(a.Y+a.Height, d.Y+d.Height) - max(a.Y, d.Y)
		if !hay || dist < mejorDist || (dist == mejorDist && solape > mejorSolape) {
			mejor, mejorDist, mejorSolape, hay = d, dist, solape, true
		}
	}
	return mejor, hay
}

// Abarque es el rectángulo que cubre todos los monitores: el escritorio
// entero al que un puntero absoluto sin salida fija reparte su rango.
func Abarque(ds []Display) Display {
	if len(ds) == 0 {
		return Display{}
	}
	x0, y0 := ds[0].X, ds[0].Y
	x1, y1 := x0+ds[0].Width, y0+ds[0].Height
	for _, d := range ds[1:] {
		x0, y0 = min(x0, d.X), min(y0, d.Y)
		x1, y1 = max(x1, d.X+d.Width), max(y1, d.Y+d.Height)
	}
	return Display{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// EnAbarque lleva un punto normalizado de d (0..1) al mismo punto
// normalizado sobre caja (el abarque de todos los monitores).
func EnAbarque(d, caja Display, x, y float64) (float64, float64) {
	if caja.Width <= 0 || caja.Height <= 0 {
		return x, y
	}
	px, py := d.Map(x, y)
	nx := (px - float64(caja.X)) / float64(caja.Width)
	ny := (py - float64(caja.Y)) / float64(caja.Height)
	return min(max(nx, 0), 1), min(max(ny, 0), 1)
}
