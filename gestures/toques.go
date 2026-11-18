package gestures

import "sort"

// Toques (dec-yp1a): el pellizco armado del embrague se convierte en un dedo
// apoyado sobre la pantalla táctil virtual. La app deja de recibir clics o
// flechas y recibe toques de verdad, así que el scroll, el zoom y el doble
// toque los pone ella con su propia física.
//
// ToquesDe es puro: la misma secuencia de resultados da siempre los mismos
// cuadros. No habla con platform; el adaptador táctil solo escribe lo que
// aquí sale.

// maxDedos es lo que admite la pantalla táctil: dos ranuras.
const maxDedos = 2

// Dedo es un dedo de la pantalla táctil en un cuadro. Ranura es 0 o 1, X e Y
// van en 0..1 sobre el panel calibrado (la misma coordenada que la app usa
// como pantalla) y Apoyado distingue el dedo que toca del que se levanta. Una
// mano armada que se sale del panel sigue tocando, como el puntero del panel,
// pero se acota al borde.
type Dedo struct {
	Ranura  int     `json:"ranura"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Apoyado bool    `json:"apoyado"`
}

// CuadroToques es lo que ve el táctil en un cuadro. Dedos lleva los dedos que
// tocan y los que se levantan en este cuadro; un dedo apoyado en el cuadro
// anterior y ausente aquí ya no toca. TMs es el tiempo del cuadro del
// embrague, para medir tap, pulsación larga y arrastre.
type CuadroToques struct {
	TMs   int64  `json:"t_ms"`
	Dedos []Dedo `json:"dedos"`
}

// ToquesDe convierte una secuencia de resultados del embrague en un cuadro de
// dedos por resultado: pellizco armado = apoyar, seguir armado = mover,
// soltar = levantar.
//
// La ranura se fija la primera vez que una mano apoya y no cambia mientras
// siga armada, aunque la otra mano se vaya o llegue otra: con dos manos hay
// dos dedos y ninguno se cruza de ranura. Si una mano se pierde, su dedo se
// levanta en ese cuadro con la última posición que tuvo (el embrague la
// mantiene durante su histéresis y suelta después; ahí se levanta). Una mano
// nueva ocupa la ranura libre más baja; si la que se acaba de levantar en el
// mismo cuadro es la única libre, espera al siguiente, para no llevar en un
// cuadro el levantar y el apoyar de la misma ranura.
//
// Los resultados se pueden pasar enteros o solo desde el último cuadro: las
// ranuras dependen solo de los resultados anteriores, así que los cuadros ya
// entregados no cambian.
func ToquesDe(resultados []ResultadoEmbrague) []CuadroToques {
	cuadros := make([]CuadroToques, len(resultados))
	ranura := map[ClaveMano]int{}
	ultima := map[int]PuntoPanel{} // última posición de la ranura, para el levantar
	for i, r := range resultados {
		ct := CuadroToques{TMs: r.TMs}

		armadas := append([]ManoArmada(nil), r.Armadas...)
		sort.Slice(armadas, func(a, b int) bool {
			return antesQue(armadas[a].Clave, armadas[b].Clave)
		})
		apoyadas := make(map[ClaveMano]bool, len(armadas))
		for _, m := range armadas {
			apoyadas[m.Clave] = true
		}

		// Primero se levanta lo que ya no está armado. Su ranura queda libre
		// para el cuadro siguiente, no para una mano que apoye en este: un
		// cuadro no puede llevar a la vez el levantar y el apoyar de la misma
		// ranura.
		liberadas := map[int]bool{}
		for clave, n := range ranura {
			if apoyadas[clave] {
				continue
			}
			p := ultima[n]
			ct.Dedos = append(ct.Dedos, Dedo{Ranura: n, X: p.X, Y: p.Y, Apoyado: false})
			delete(ranura, clave)
			delete(ultima, n)
			liberadas[n] = true
		}
		for _, m := range armadas {
			n, ok := ranura[m.Clave]
			if !ok {
				n = ranuraLibre(ranura, liberadas)
				if n < 0 {
					// No hay ranura para una mano nueva en este cuadro (la
					// otra mano sigue tocando o la ranura se acaba de
					// levantar): espera al siguiente, así nunca se mueve un
					// dedo que ya está tocando.
					continue
				}
				ranura[m.Clave] = n
			}
			p := PuntoPanel{X: acotar01(m.X), Y: acotar01(m.Y)}
			ultima[n] = p
			ct.Dedos = append(ct.Dedos, Dedo{Ranura: n, X: p.X, Y: p.Y, Apoyado: true})
		}
		sort.Slice(ct.Dedos, func(a, b int) bool {
			return ct.Dedos[a].Ranura < ct.Dedos[b].Ranura
		})
		cuadros[i] = ct
	}
	return cuadros
}

// acotar01 lleva una posición al borde del panel si la mano armada se salió.
func acotar01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ranuraLibre devuelve la ranura más baja que no esté ocupada ni se acabe de
// levantar en este cuadro, o -1 si no hay ninguna.
func ranuraLibre(ranura map[ClaveMano]int, liberadas map[int]bool) int {
	for n := 0; n < maxDedos; n++ {
		if liberadas[n] {
			continue
		}
		ocupada := false
		for _, usada := range ranura {
			if usada == n {
				ocupada = true
				break
			}
		}
		if !ocupada {
			return n
		}
	}
	return -1
}
