// Package platform define lo que Lazul le pide al sistema operativo: una
// interfaz pequeña por capacidad. Cada sistema la implementa en su propio
// paquete (platform/linux, platform/windows), un archivo por capacidad, y
// pasa las mismas pruebas de contrato (platform/contrato).
//
// El núcleo (core/) solo habla con estas interfaces; nunca llama al sistema.
package platform

import (
	"context"
	"errors"
)

// Errores que las implementaciones devuelven envueltos (fmt.Errorf("...: %w",
// err)) para que quien llama los distinga con errors.Is.
var (
	// ErrNoImplementado: esta capacidad todavía no existe en este sistema.
	ErrNoImplementado = errors.New("platform: no implementado en este sistema")
	// ErrNoEncontrada: la app, URL o archivo pedido no existe aquí.
	ErrNoEncontrada = errors.New("platform: no encontrada")
	// ErrRefInvalida: la referencia vino vacía o contradictoria.
	ErrRefInvalida = errors.New("platform: referencia inválida")
	// ErrSinVentana: no hay ninguna ventana con el foco.
	ErrSinVentana = errors.New("platform: ninguna ventana tiene el foco")
)

// Todas las llamadas reciben un context.Context. Si llega ya cancelado, la
// implementación devuelve un error que cumple errors.Is(err, ctx.Err()) sin
// tocar el sistema. Lo exige el contrato.

// Imagen es una captura de pantalla en PNG.
type Imagen struct {
	PNG   []byte
	Ancho int // en píxeles, igual al del PNG
	Alto  int
}

// Screenshot captura la pantalla donde está la ventana con el foco (o la
// principal si no se sabe).
type Screenshot interface {
	Capture(ctx context.Context) (Imagen, error)
}

// Clipboard lee y escribe texto en el portapapeles. El texto vuelve igual a
// como se escribió: UTF-8 y saltos de línea "\n" (en Windows la
// implementación convierte de y hacia "\r\n").
type Clipboard interface {
	ReadText(ctx context.Context) (string, error)
	WriteText(ctx context.Context, texto string) error
}

// App describe la ventana con el foco. URL y Archivo son los niveles 1 y 2
// del gesto 4 (dec-r2ii): se llenan cuando se pueden saber.
type App struct {
	ID      string // ejecutable o AUMID en Windows; clase o .desktop en Linux
	Nombre  string // legible: "Firefox"
	Titulo  string // título de la ventana
	PID     int
	URL     string // si es un navegador y se pudo leer
	Archivo string // ruta del documento abierto, si se pudo leer
}

// ActiveApp dice qué app tiene el foco.
type ActiveApp interface {
	Active(ctx context.Context) (App, error)
}

// AppRef es lo que se pide abrir. Se usa el primer campo no vacío en este
// orden: URL, Archivo, ID. Si todos están vacíos: ErrRefInvalida. Si el ID no
// corresponde a ninguna app instalada: ErrNoEncontrada.
type AppRef struct {
	ID      string
	URL     string
	Archivo string
}

// LaunchApp abre una app, una URL o un archivo, sin esperar a que se cierre.
type LaunchApp interface {
	Launch(ctx context.Context, ref AppRef) error
}

// Sistema junta las capacidades de un equipo. Cada paquete de sistema
// expone New() Sistema.
type Sistema struct {
	Screenshot Screenshot
	Clipboard  Clipboard
	ActiveApp  ActiveApp
	LaunchApp  LaunchApp
}
