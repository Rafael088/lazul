// Package gestures convierte los puntos de la mano (MediaPipe Hand
// Landmarker, 21 puntos normalizados) en uno de los gestos 1 a 4.
//
// El clasificador es una función pura: mismo estado y mismo cuadro, mismo
// resultado. Así se prueba con grabaciones (fixtures/gestos, ./dev replay)
// sin cámara. Su implementación es la tarea cotejo-tw9i76; aquí quedan los
// tipos, el formato de las grabaciones y la medición.
package gestures

import "github.com/Rafael088/lazul/proto/go/lazulv1"

// NumPuntos es la cantidad de puntos por mano de MediaPipe.
const NumPuntos = 21

// Índices de MediaPipe que más se usan.
const (
	Muneca       = 0
	PulgarPunta  = 4
	IndicePunta  = 8
	MedioPunta   = 12
	AnularPunta  = 16
	MeniquePunta = 20
)

// Punto en coordenadas normalizadas de la imagen (0 a 1; Z relativa a la
// muñeca, negativa hacia la cámara).
type Punto struct{ X, Y, Z float32 }

// Mano es una mano detectada en un cuadro.
type Mano struct {
	Puntos  [NumPuntos]Punto
	Derecha bool // según MediaPipe (imagen sin espejar)
}

// Cuadro es lo que MediaPipe ve en un instante.
type Cuadro struct {
	TMs   int64 // ms desde el inicio de la grabación o de la cámara
	Manos []Mano
}

// Evento es un gesto reconocido.
type Evento struct {
	Gesto     lazulv1.Gesto
	Confianza float32
	Direccion lazulv1.Direccion
}

// Estado guarda lo que el clasificador necesita recordar entre cuadros
// (por ejemplo, desde cuándo se sostiene la L). Lo define cotejo-tw9i76.
type Estado struct{}

// Clasificar avanza un cuadro. Devuelve el estado nuevo y, si en este cuadro
// se completó un gesto, el Evento (si no, Gesto es GESTO_NINGUNO).
//
// Pendiente (cotejo-tw9i76): hoy nunca reconoce nada.
func Clasificar(e Estado, c Cuadro) (Estado, Evento) {
	return e, Evento{}
}
