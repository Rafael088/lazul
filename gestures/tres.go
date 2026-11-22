package gestures

import "math"

// LadoPantalla es hacia dónde pasa el control al cambiar de pantalla, visto
// por la persona: su izquierda o su derecha (la imagen llega sin espejar).
type LadoPantalla int

const (
	PantallaNinguna LadoPantalla = iota
	PantallaIzquierda
	PantallaDerecha
)

func (l LadoPantalla) String() string {
	switch l {
	case PantallaIzquierda:
		return "izquierda"
	case PantallaDerecha:
		return "derecha"
	}
	return "ninguna"
}

// Umbrales de la pose de tres dedos, en largos de palma. Provisionales: salen
// del modelo sintético de la mano y de que ninguna grabación real (gestos 0.1
// y pellizco) la muestre sostenida; se ajustan con tomas reales y con las
// líneas «pantalla» de factibilidad.jsonl.
const (
	// El meñique recogido de la pose; a medias (como el de la L) basta.
	meniqueTres = 0.40
	// Pulgar recogido: su punta cerca del centro de la palma (cruzado sobre
	// ella) o pegada al nudillo del índice. Una mano abierta lo tiene a más
	// de una palma del centro.
	pulgarTresCentro = 0.70
	pulgarTresIndice = 0.45
)

// tresDedos: índice, medio y anular estirados; meñique y pulgar recogidos.
// La separa de la mano abierta el pulgar (fuera en la abierta) y el meñique,
// y de la pinza que las puntas del pulgar y el índice quedan lejos.
func (r rasgos) tresDedos(pulgarCentro float64) bool {
	return r.ext[0] > dedoEstirado && r.ext[1] > dedoEstirado && r.ext[2] > dedoEstirado &&
		r.ext[3] < meniqueTres &&
		(pulgarCentro < pulgarTresCentro || r.pulgar < pulgarTresIndice) &&
		r.puntas > puntasPinza
}

// medirTres mide la mano y dice si está en la pose de tres dedos.
func medirTres(m Mano, aspecto float32) (rasgos, bool) {
	r := medir(m, aspecto)
	if r.palma < 1e-6 {
		return r, false
	}
	asp := aspectoDe(aspecto)
	punta := vec{float64(m.Puntos[PulgarPunta].X) * asp, float64(m.Puntos[PulgarPunta].Y)}
	return r, r.tresDedos(dist(punta, r.centro) / r.palma)
}

// TresDedos dice si la mano está en la pose de cambiar de pantalla: índice,
// medio y anular estirados con el pulgar y el meñique recogidos.
func TresDedos(m Mano, aspecto float32) bool {
	_, ok := medirTres(m, aspecto)
	return ok
}

// AjustesTresDedos fija cuándo deslizar la mano en la pose de tres dedos
// cambia de pantalla. No hay velocidad mínima: como los tres dedos del panel
// táctil, cuenta el recorrido desde donde empezó la pose.
type AjustesTresDedos struct {
	Umbral      float64 // recorrido horizontal desde el ancla, en palmas
	Dominio     float64 // |dx| debe ser al menos Dominio·|dy| al disparar
	VerticalMax float64 // si |dy| pasa esto (palmas) antes de disparar, el intento se anula
	MinCuadros  int     // cuadros en la pose antes de poder disparar (los que fijan el ancla)
	Gracia      int64   // ms que la pose puede perderse sin terminar el intento
	SaltoMax    float64 // un paso entre cuadros mayor que esto (palmas) es un salto del seguimiento
	Refractario int64   // ms sin otro disparo tras uno
}

// AjustesTresDedosPorDefecto: ~1 palma a un lado, sin prisa. Con 15 fps los
// 2 cuadros del ancla son 133 ms.
var AjustesTresDedosPorDefecto = AjustesTresDedos{
	Umbral:      1.0,
	Dominio:     1.5,
	VerticalMax: 1.0,
	MinCuadros:  2,
	Gracia:      200,
	SaltoMax:    1.5,
	Refractario: 700,
}

// Motivos de un intento que no disparó (van al registro tal cual).
const (
	MotivoDisparo  = "disparo"
	MotivoCorto    = "corto"    // se soltó la pose sin llegar al Umbral
	MotivoVertical = "vertical" // la mano se fue hacia arriba o abajo
	MotivoSalto    = "salto"    // el seguimiento saltó
	MotivoDosManos = "dos_manos"
	MotivoOcupado  = "ocupado" // el pellizco agarraba o se calibraba
)

// IntentoTres resume un intento de cambiar de pantalla: cuando dispara o
// cuando termina sin disparar. Nunca lleva posiciones: solo el recorrido
// relativo en palmas y cuánto duró.
type IntentoTres struct {
	Motivo  string
	Lado    LadoPantalla // el del recorrido (en un disparo, el del cambio)
	Palmas  float64      // recorrido horizontal alcanzado, en palmas
	Ms      int64        // desde que se vio la pose
	Cuadros int          // cuadros con la pose
}

// ResultadoTres es lo que deja cada cuadro. Mano es el índice de la mano en
// la pose (o -1): mientras la hay, esa mano no toca ni arrastra en el panel.
// Lado no es PantallaNinguna solo en el cuadro del disparo. Intento no es
// nil cuando un intento disparó o terminó.
type ResultadoTres struct {
	Mano    int
	Lado    LadoPantalla
	Intento *IntentoTres
}

// DetectorTresDedos reconoce, cuadro a cuadro, la pose de tres dedos y su
// deslizamiento a un lado. Un intento empieza al ver la pose, fija el ancla
// tras MinCuadros y dispara una sola vez cuando la mano se aleja Umbral
// palmas en horizontal; para otro disparo hay que soltar la pose y volver
// a hacerla (como levantar los dedos del panel táctil).
type DetectorTresDedos struct {
	Aj AjustesTresDedos

	activo    bool // hay un intento en curso
	disparado bool // este intento ya disparó
	anulado   string
	inicio    int64 // ms del primer cuadro con la pose
	visto     int64 // ms del último cuadro con la pose
	cuadros   int
	ancla     vec
	palmaAcum float64
	ult       vec
	ultPalma  float64
	maxDx     float64 // recorrido horizontal con signo de mayor valor absoluto
	ultimo    int64   // ms del último disparo
	disparos  int
}

// NuevoDetectorTresDedos usa AjustesTresDedosPorDefecto.
func NuevoDetectorTresDedos() *DetectorTresDedos {
	return &DetectorTresDedos{Aj: AjustesTresDedosPorDefecto}
}

func (d *DetectorTresDedos) palma() float64 {
	if d.cuadros == 0 {
		return d.ultPalma
	}
	return d.palmaAcum / float64(d.cuadros)
}

// terminar cierra el intento en curso y devuelve su resumen (nil si no
// había, si ya disparó —el disparo ya se contó— o si fue un cuadro suelto).
func (d *DetectorTresDedos) terminar(motivo string) *IntentoTres {
	if !d.activo {
		return nil
	}
	in := &IntentoTres{Motivo: motivo, Palmas: math.Abs(d.maxDx), Ms: d.visto - d.inicio, Cuadros: d.cuadros, Lado: ladoDe(d.maxDx)}
	if d.anulado != "" {
		in.Motivo = d.anulado
	}
	ya := d.disparado
	d.activo, d.disparado, d.anulado = false, false, ""
	d.cuadros, d.palmaAcum, d.maxDx = 0, 0, 0
	if ya {
		return nil
	}
	if in.Cuadros < d.Aj.MinCuadros {
		// La pose en un cuadro suelto (al cambiar de forma la mano) no es
		// un intento: no se anota.
		return nil
	}
	return in
}

// Reiniciar termina el intento en curso (por ejemplo, porque el pellizco
// agarra) y devuelve su resumen con el motivo dado.
func (d *DetectorTresDedos) Reiniciar(motivo string) *IntentoTres {
	return d.terminar(motivo)
}

func ladoDe(dx float64) LadoPantalla {
	switch {
	case dx < 0: // imagen sin espejar: la X menor es la derecha de la persona
		return PantallaDerecha
	case dx > 0:
		return PantallaIzquierda
	}
	return PantallaNinguna
}

// Cuadro procesa un cuadro.
func (d *DetectorTresDedos) Cuadro(c Cuadro) ResultadoTres {
	res := ResultadoTres{Mano: -1}
	mano := -1
	var r rasgos
	n := 0
	for i, m := range c.Manos {
		if rr, ok := medirTres(m, c.Aspecto); ok {
			if n == 0 {
				mano, r = i, rr
			}
			n++
		}
	}
	if n > 1 {
		// Dos manos en la pose: no se sabe cuál desliza.
		res.Intento = d.terminar(MotivoDosManos)
		return res
	}
	if mano < 0 {
		if d.activo && c.TMs-d.visto <= d.Aj.Gracia {
			// Un cuadro suelto sin la pose: la mano sigue siendo la del
			// gesto y el panel tampoco la recibe.
			if len(c.Manos) == 1 {
				res.Mano = 0
			}
			return res
		}
		res.Intento = d.terminar(MotivoCorto)
		return res
	}
	res.Mano = mano
	if d.activo && (c.TMs < d.visto || dist(r.centro, d.ult) > d.Aj.SaltoMax*(r.palma+d.ultPalma)/2) {
		// Salto del seguimiento (o reloj hacia atrás): el intento no vale.
		res.Intento = d.terminar(MotivoSalto)
	}
	if !d.activo {
		d.activo, d.inicio = true, c.TMs
	}
	d.visto, d.ult, d.ultPalma = c.TMs, r.centro, r.palma
	d.cuadros++
	if d.cuadros <= d.Aj.MinCuadros {
		// El ancla es la media de los primeros cuadros: el primero suele
		// salir movido al formar la pose.
		d.palmaAcum += r.palma
		d.ancla = d.ancla.por(float64(d.cuadros-1) / float64(d.cuadros)).mas(r.centro.por(1 / float64(d.cuadros)))
		return res
	}
	d.palmaAcum += r.palma
	if d.disparado || d.anulado != "" {
		return res
	}
	p := d.palma()
	dx := (r.centro.x - d.ancla.x) / p
	dy := (r.centro.y - d.ancla.y) / p
	if math.Abs(dx) > math.Abs(d.maxDx) {
		d.maxDx = dx
	}
	if math.Abs(dy) > d.Aj.VerticalMax && math.Abs(dx) < d.Aj.Umbral {
		d.anulado = MotivoVertical
		return res
	}
	if math.Abs(dx) < d.Aj.Umbral || math.Abs(dx) < d.Aj.Dominio*math.Abs(dy) {
		return res
	}
	if d.disparos > 0 && c.TMs-d.ultimo < d.Aj.Refractario {
		return res
	}
	d.disparado = true
	d.ultimo, d.disparos = c.TMs, d.disparos+1
	res.Lado = ladoDe(dx)
	res.Intento = &IntentoTres{Motivo: MotivoDisparo, Lado: res.Lado, Palmas: math.Abs(dx), Ms: c.TMs - d.inicio, Cuadros: d.cuadros}
	return res
}
