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

func puntosTorso(c *Cuerpo) ([4]punto2, bool) {
	var q [4]punto2
	if c == nil {
		return q, false
	}
	// El orden dibuja el cuadrilátero: hombro izq. → hombro der. → cadera
	// der. → cadera izq. Funciona igual si la imagen está espejada.
	indices := [...]int{HombroIzquierdo, HombroDerecho, CaderaDerecha, CaderaIzquierda}
	for i, indice := range indices {
		p := c.Puntos[indice]
		if p.Visibilidad < visibilidadTorsoMin || p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			return q, false
		}
		q[i] = punto2{float64(p.X), float64(p.Y)}
	}
	return q, true
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
// solo las manos delante del torso y cuántas rechazó. Si Pose no ve un torso
// fiable, rechaza todas las manos: una pérdida de Pose no puede activar nada.
func FiltrarPorTorso(c Cuadro) (Cuadro, int) {
	filtrado := c
	filtrado.Manos = make([]Mano, 0, len(c.Manos))
	for _, mano := range c.Manos {
		if ManoDelanteDelTorso(mano, c.Cuerpo) {
			filtrado.Manos = append(filtrado.Manos, mano)
		}
	}
	return filtrado, len(c.Manos) - len(filtrado.Manos)
}
