package gestures

// visibilidadTorsoMin evita que una conjetura de Pose abra el control. El
// modelo entrega 0..1; 0,5 conserva oclusiones leves y rechaza puntos que no
// está viendo de forma fiable.
const visibilidadTorsoMin = .5

type punto2 struct{ x, y float64 }

// centroPalma usa solo muñeca y nudillos. Las puntas pueden estar sobre la
// cabeza o señalando fuera aunque la palma siga delante del torso.
func centroPalma(m Mano) punto2 {
	indices := [...]int{Muneca, IndiceMCP, MedioMCP, AnularMCP, MeniqueMCP}
	var p punto2
	for _, i := range indices {
		p.x += float64(m.Puntos[i].X)
		p.y += float64(m.Puntos[i].Y)
	}
	p.x /= float64(len(indices))
	p.y /= float64(len(indices))
	return p
}

// largoTorso es cuánto baja el torso desde los hombros, en anchos de hombro,
// cuando Pose no ve las caderas (sentado frente al PC las tapa el escritorio
// o quedan fuera de cuadro). Un adulto mide de hombros a cadera ~1,25 veces
// el ancho de hombros.
const largoTorso = 1.25

func puntosTorso(c *Cuerpo) ([4]punto2, bool) { return puntosTorsoAspecto(c, 0) }

// puntosTorsoAspecto da el cuadrilátero hombros-caderas. Si las caderas no
// son fiables pero los dos hombros sí, las estima bajando desde los hombros
// en perpendicular, con el aspecto de la imagen para que el ángulo recto sea
// en píxeles (4:3 si no se sabe).
func puntosTorsoAspecto(c *Cuerpo, aspecto float32) ([4]punto2, bool) {
	var q [4]punto2
	if c == nil {
		return q, false
	}
	fiable := func(i int) (punto2, bool) {
		p := c.Puntos[i]
		if p.Visibilidad < visibilidadTorsoMin || p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			return punto2{}, false
		}
		return punto2{float64(p.X), float64(p.Y)}, true
	}
	hi, ok1 := fiable(HombroIzquierdo)
	hd, ok2 := fiable(HombroDerecho)
	if !ok1 || !ok2 {
		return q, false
	}
	// El orden dibuja el cuadrilátero: hombro izq. → hombro der. → cadera
	// der. → cadera izq. Funciona igual si la imagen está espejada.
	cd, ok3 := fiable(CaderaDerecha)
	ci, ok4 := fiable(CaderaIzquierda)
	if ok3 && ok4 {
		return [4]punto2{hi, hd, cd, ci}, true
	}
	a := float64(aspecto)
	if a <= 0 {
		a = 4.0 / 3.0
	}
	// En píxeles (x por el aspecto), la perpendicular a los hombros que
	// apunta hacia abajo de la imagen.
	wx, wy := (hd.x-hi.x)*a, hd.y-hi.y
	nx, ny := -wy, wx
	if ny < 0 {
		nx, ny = -nx, -ny
	}
	nx, ny = nx*largoTorso/a, ny*largoTorso
	return [4]punto2{hi, hd, {hd.x + nx, hd.y + ny}, {hi.x + nx, hi.y + ny}}, true
}

// TorsoFiable permite distinguir «Pose no encontró un cuerpo» de «sí hay
// cuerpo pero la mano está fuera» al registrar una evitada.
func TorsoFiable(c *Cuerpo) bool {
	_, ok := puntosTorso(c)
	return ok
}

func cruz(a, b, p punto2) float64 {
	return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
}

// dentroConvexo acepta puntos en el borde y cuadriláteros vistos en sentido
// horario o antihorario.
func dentroConvexo(p punto2, q [4]punto2) bool {
	var signo int
	for i := range q {
		z := cruz(q[i], q[(i+1)%len(q)], p)
		if z > 1e-9 {
			if signo < 0 {
				return false
			}
			signo = 1
		} else if z < -1e-9 {
			if signo > 0 {
				return false
			}
			signo = -1
		}
	}
	return signo != 0
}

// ManoDelanteDelTorso dice si el centro de la palma cae en el cuadrilátero
// hombros-caderas. Pose y Hand usan el mismo fotograma y coordenadas 0..1,
// así que no hay que transformar ni comparar sus escalas Z incompatibles.
func ManoDelanteDelTorso(m Mano, cuerpo *Cuerpo) bool {
	torso, ok := puntosTorso(cuerpo)
	return ok && dentroConvexo(centroPalma(m), torso)
}

// FiltrarPorTorso devuelve una copia superficial del cuadro que conserva
// solo las manos delante del torso y cuántas rechazó. Sin caderas fiables
// (sentado), el torso se estima desde los hombros. Si Pose no ve los dos
// hombros, rechaza todas las manos: una pérdida de Pose no puede activar nada.
func FiltrarPorTorso(c Cuadro) (Cuadro, int) {
	filtrado := c
	filtrado.Manos = make([]Mano, 0, len(c.Manos))
	torso, ok := puntosTorsoAspecto(c.Cuerpo, c.Aspecto)
	for _, mano := range c.Manos {
		if ok && dentroConvexo(centroPalma(mano), torso) {
			filtrado.Manos = append(filtrado.Manos, mano)
		}
	}
	return filtrado, len(c.Manos) - len(filtrado.Manos)
}
