package gestures

import "math"

// Pose es la forma de la mano en un solo cuadro, sin memoria.
type Pose int

const (
	PoseNinguna Pose = iota // no hay mano
	PoseOtra                // hay mano pero no es ninguna de las de abajo
	PoseL                   // índice arriba, pulgar al lado (~90°), los demás recogidos
	PosePuno                // los cuatro dedos recogidos y el pulgar pegado
	PoseAbierta             // al menos tres dedos estirados y el pulgar fuera
	PosePinza               // puntas del pulgar y el índice juntas
	PoseSenalar             // índice estirado, los demás recogidos, el pulgar no está fuera
)

var nombresPose = [...]string{"ninguna", "otra", "L", "puño", "abierta", "pinza", "señalar"}

func (p Pose) String() string {
	if p < 0 || int(p) >= len(nombresPose) {
		return "?"
	}
	return nombresPose[p]
}

// Umbrales de la pose, en largos de palma (muñeca a nudillo del medio) o en
// grados. Salieron del modelo sintético de la mano (sintetica_test.go) y de
// la forma de los puntos de MediaPipe en prototipos/camara; se ajustan con
// las grabaciones reales (cotejo-x2hyu0) usando ./dev replay.
const (
	// Un dedo está estirado si su punta pasa su nudillo, a lo largo del eje
	// de la palma, por más de esto; recogido si pasa por menos.
	dedoEstirado = 0.55
	dedoRecogido = 0.30
	// El meñique de la L puede quedar a medias.
	meniqueL = 0.40
	// El índice de la L apunta hacia arriba con esta tolerancia y casi recto.
	indiceArribaMaxGrados = 40
	indiceRectoMaxGrados  = 35
	// Pulgar fuera: su punta lejos del nudillo del índice; en la L, bien
	// lejos. Por debajo de pulgarPegado cuenta como pegado (puño).
	pulgarFuera  = 0.50
	pulgarL      = 0.65
	pulgarPegado = 0.55
	// Ángulo entre el pulgar (base 1 → punta 4) y el índice (5→8) en la L:
	// cerca de 90°. Es lo que la separa de señalar con el pulgar a medio
	// salir (unos 45°). Se mide desde la base, no desde el nudillo, porque
	// el vector es más largo y el ruido lo tuerce menos.
	anguloLMin = 60
	anguloLMax = 125
	// En la L las puntas están bien separadas; en la pinza, juntas. Lo que
	// queda entre los dos (la «C») no es ninguna de las dos.
	puntasL     = 0.90
	puntasPinza = 0.30
	// En la pinza el índice no está recogido del todo (si no, es un puño).
	indicePinzaMin = -0.10
)

type vec struct{ x, y float64 }

func (a vec) menos(b vec) vec     { return vec{a.x - b.x, a.y - b.y} }
func (a vec) mas(b vec) vec       { return vec{a.x + b.x, a.y + b.y} }
func (a vec) por(k float64) vec   { return vec{a.x * k, a.y * k} }
func (a vec) punto(b vec) float64 { return a.x*b.x + a.y*b.y }
func (a vec) largo() float64      { return math.Hypot(a.x, a.y) }
func dist(a, b vec) float64       { return a.menos(b).largo() }
func (a vec) unitario() vec       { l := a.largo(); return vec{a.x / l, a.y / l} }
func gradosEntre(a, b vec) float64 {
	la, lb := a.largo(), b.largo()
	if la == 0 || lb == 0 {
		return 180
	}
	c := a.punto(b) / (la * lb)
	return math.Acos(math.Max(-1, math.Min(1, c))) * 180 / math.Pi
}

// rasgos son las medidas de la mano en un cuadro, ya corregidas por el
// aspecto de la imagen (en unidades del alto de la imagen).
type rasgos struct {
	palma    float64    // muñeca a nudillo del medio
	ext      [4]float64 // índice, medio, anular, meñique: avance de la punta sobre su nudillo, en palmas
	recto    float64    // cuánto se dobla el índice en su PIP, en grados
	arriba   float64    // ángulo del índice (5→8) con la vertical hacia arriba
	pulgar   float64    // punta del pulgar a nudillo del índice, en palmas
	angulo   float64    // pulgar (1→4) contra índice (5→8), en grados
	puntas   float64    // punta del pulgar a punta del índice, en palmas
	centro   vec        // centro de la palma
	pinza    vec        // punto medio entre las puntas del pulgar y el índice
	pose     Pose
	estirado int // cuántos de los cuatro dedos están estirados
}

func aspectoDe(a float32) float64 {
	if a <= 0 {
		return 4.0 / 3
	}
	return float64(a)
}

func medir(m Mano, aspecto float32) rasgos {
	asp := aspectoDe(aspecto)
	var p [NumPuntos]vec
	for i, q := range m.Puntos {
		p[i] = vec{float64(q.X) * asp, float64(q.Y)}
	}
	var r rasgos
	r.palma = dist(p[Muneca], p[MedioMCP])
	if r.palma < 1e-6 {
		r.pose = PoseOtra
		return r
	}
	eje := p[MedioMCP].menos(p[Muneca]).unitario()
	for d, mcp := range [4]int{IndiceMCP, MedioMCP, AnularMCP, MeniqueMCP} {
		r.ext[d] = p[mcp+3].menos(p[mcp]).punto(eje) / r.palma
		if r.ext[d] > dedoEstirado {
			r.estirado++
		}
	}
	indice := p[IndicePunta].menos(p[IndiceMCP])
	r.recto = gradosEntre(p[IndicePIP].menos(p[IndiceMCP]), p[IndicePunta].menos(p[IndicePIP]))
	r.arriba = gradosEntre(indice, vec{0, -1})
	r.pulgar = dist(p[PulgarPunta], p[IndiceMCP]) / r.palma
	r.angulo = gradosEntre(p[PulgarPunta].menos(p[PulgarBase]), indice)
	r.puntas = dist(p[PulgarPunta], p[IndicePunta]) / r.palma
	for _, i := range [5]int{Muneca, IndiceMCP, MedioMCP, AnularMCP, MeniqueMCP} {
		r.centro = r.centro.mas(p[i].por(0.2))
	}
	r.pinza = p[PulgarPunta].mas(p[IndicePunta]).por(0.5)
	r.pose = r.clasificarPose()
	return r
}

func (r rasgos) clasificarPose() Pose {
	idx, med, anu, men := r.ext[0], r.ext[1], r.ext[2], r.ext[3]
	recogidos := med < dedoRecogido && anu < dedoRecogido
	switch {
	case r.puntas < puntasPinza && idx > indicePinzaMin:
		return PosePinza
	case r.esL(recogidos, men):
		return PoseL
	case idx < dedoRecogido && recogidos && men < dedoRecogido && r.pulgar < pulgarPegado:
		return PosePuno
	case r.estirado >= 3 && r.pulgar > pulgarFuera:
		return PoseAbierta
	case idx > dedoEstirado && recogidos:
		return PoseSenalar
	}
	return PoseOtra
}

// esL: índice estirado hacia arriba, medio y anular recogidos, y el pulgar
// fuera, a unos 90° del índice y con las puntas separadas. Las tres
// condiciones del pulgar son las que la separan de la pinza (puntas juntas)
// y de señalar (pulgar pegado, paralelo, o perdido por MediaPipe).
func (r rasgos) esL(recogidos bool, menique float64) bool {
	return r.ext[0] > dedoEstirado && r.recto < indiceRectoMaxGrados && r.arriba < indiceArribaMaxGrados &&
		recogidos && menique < meniqueL &&
		r.pulgar > pulgarL && r.angulo > anguloLMin && r.angulo < anguloLMax && r.puntas > puntasL
}

// PoseDe dice la forma de la mano en un cuadro. aspecto es ancho/alto de la
// imagen (0 = 4:3).
func PoseDe(m Mano, aspecto float32) Pose { return medir(m, aspecto).pose }
