// Package gestures convierte los puntos de la mano (MediaPipe Hand
// Landmarker, 21 puntos normalizados) en gestos discretos.
//
// El clasificador es una función pura: mismo estado y mismo cuadro, mismo
// resultado. Así se prueba con grabaciones (fixtures/gestos, ./dev replay)
// sin cámara. Tiene dos capas:
//
//   - PoseDe (pose.go): qué forma tiene la mano en un cuadro (la L, puño,
//     mano abierta, pinza, señalar u otra). Sin memoria.
//   - Clasificar (clasificador.go): los gestos 1 a 4 de transferencia.
//   - ClasificarTactil (tactil.go): la L alterna el modo táctil y, cuando
//     está activo, reconoce scroll, zoom y barrido sin cursor ni clic.
//   - Pellizco (pellizco.go): apertura pulgar-índice y estado con histéresis
//     para las tomas del modelo nuevo de manipulación directa.
//
// El formato de las grabaciones está en grabacion.go y la herramienta para
// grabarlas en gestures/cmd/grabar (./dev grabar).
package gestures

import "github.com/Rafael088/lazul/proto/go/lazulv1"

// NumPuntos es la cantidad de puntos por mano de MediaPipe.
const NumPuntos = 21

// NumPuntosCuerpo es la cantidad de puntos que entrega MediaPipe Pose.
const NumPuntosCuerpo = 33

// Índices de MediaPipe (ver la figura de Hand Landmarker): 0 muñeca;
// pulgar 1 a 4; índice 5 a 8; medio 9 a 12; anular 13 a 16; meñique 17 a 20.
// En cada dedo: nudillo (MCP), articulación media (PIP), la siguiente (DIP)
// y la punta.
const (
	Muneca       = 0
	PulgarBase   = 1
	PulgarMCP    = 2
	PulgarPunta  = 4
	IndiceMCP    = 5
	IndicePIP    = 6
	IndicePunta  = 8
	MedioMCP     = 9
	MedioPunta   = 12
	AnularMCP    = 13
	AnularPunta  = 16
	MeniqueMCP   = 17
	MeniquePunta = 20
)

// Índices de MediaPipe Pose que delimitan el torso. Los nombres son los de
// la persona (no los lados de la imagen, que puede venir espejada).
const (
	HombroIzquierdo = 11
	HombroDerecho   = 12
	CaderaIzquierda = 23
	CaderaDerecha   = 24
)

// Punto en coordenadas normalizadas de la imagen (0 a 1; Z relativa a la
// muñeca, negativa hacia la cámara).
type Punto struct{ X, Y, Z float32 }

// PuntoCuerpo añade la confianza de Pose. Una coordenada sin visibilidad
// suficiente no se usa para abrir la zona: ante la duda se falla cerrado.
type PuntoCuerpo struct {
	X, Y, Z     float32
	Visibilidad float32
}

// Cuerpo contiene los puntos normalizados de una persona detectada por
// MediaPipe Pose. Solo se conserva el cuadro actual; nunca se registra.
type Cuerpo struct{ Puntos [NumPuntosCuerpo]PuntoCuerpo }

// Mano es una mano detectada en un cuadro.
type Mano struct {
	Puntos  [NumPuntos]Punto
	Derecha bool // según MediaPipe (imagen sin espejar)
	// Pellizco solo viene en las tomas del modelo nuevo: estado del embrague
	// y apertura pulgar-índice de esta mano en este cuadro (ver pellizco.go).
	Pellizco *EstadoPellizco
}

// Cuadro es lo que MediaPipe ve en un instante.
type Cuadro struct {
	TMs    int64 // ms desde el inicio de la grabación o de la cámara
	Manos  []Mano
	Cuerpo *Cuerpo
	// Aspecto es ancho/alto de la imagen. MediaPipe normaliza X por el
	// ancho e Y por el alto, así que sin esto los ángulos salen torcidos.
	// 0 significa 4:3 (la cámara se pide a 640×480).
	Aspecto float32
}

// Evento es un gesto reconocido.
type Evento struct {
	Gesto     lazulv1.Gesto
	Confianza float32
	Direccion lazulv1.Direccion
}
