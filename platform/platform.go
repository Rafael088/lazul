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
	// ErrAccionInvalida: una acción del sistema lleva una dirección o modo
	// que esa capacidad no admite.
	ErrAccionInvalida = errors.New("platform: acción inválida")
	// ErrSinAccesibilidad: no hay bus de accesibilidad (falta at-spi2-core o
	// no hay bus de sesión); sin él el imán no puede leer objetivos.
	ErrSinAccesibilidad = errors.New("platform: no hay bus de accesibilidad")
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

// ScrollDirection expresa el sentido de una rueda vertical.
type ScrollDirection uint8

const (
	ScrollUp ScrollDirection = iota + 1
	ScrollDown
)

// ZoomDirection expresa el sentido del zoom de la ventana con foco.
type ZoomDirection uint8

const (
	ZoomIn ZoomDirection = iota + 1
	ZoomOut
)

// NavigationDirection expresa el sentido de una navegación horizontal.
type NavigationDirection uint8

const (
	NavigateLeft NavigationDirection = iota + 1
	NavigateRight
)

// NavigationMode elige entre flechas y teclas para diapositivas.
type NavigationMode uint8

const (
	NavigationNormal NavigationMode = iota + 1
	NavigationSlides
)

// TactileActions envía scroll, zoom o navegación a la ventana con foco. No
// mueve el cursor, no hace clic y no cambia qué ventana tiene el foco.
type TactileActions interface {
	Scroll(ctx context.Context, direction ScrollDirection) error
	Zoom(ctx context.Context, direction ZoomDirection) error
	Navigate(ctx context.Context, direction NavigationDirection, mode NavigationMode) error
}

// ContinuousScroller desplaza como la rueda del ratón, de forma continua:
// unidades de alta resolución (120 = un paso de rueda), positivas hacia
// abajo. Como la rueda física, actúa sobre la ventana que está bajo el
// puntero. Es opcional: quien no la tenga usa TactileActions.Scroll.
type ContinuousScroller interface {
	ScrollContinuo(ctx context.Context, unidades int) error
}

// AbsolutePointer mueve un puntero virtual a una posición normalizada de la
// pantalla. (0,0) es la esquina superior izquierda y (1,1), la inferior
// derecha. Button pulsa o suelta el botón principal, que es el embrague del
// pellizco. Es opcional: hoy solo lo ofrece Linux mediante uinput.
type AbsolutePointer interface {
	Prepare(ctx context.Context) error
	MoveAbsolute(ctx context.Context, x, y float64) error
	Button(ctx context.Context, down bool) error
}

// TouchPoint describe un dedo en una ranura estable de la pantalla táctil.
// Slot solo puede ser 0 o 1. X e Y están normalizados: (0,0) es la esquina
// superior izquierda y (1,1), la inferior derecha. Down dice si el dedo está
// apoyado; un punto omitido del cuadro también se considera levantado.
type TouchPoint struct {
	Slot int
	X, Y float64
	Down bool
}

// DiametroContacto es el lado del área de contacto de cada dedo táctil, en
// píxeles lógicos (los CSS a escala 1). Con un dedo «gordo» el navegador y
// el sistema pueden llevar un toque que cae casi encima de un botón o
// enlace chico hasta él (touch adjustment de Chromium/Edge, targeting de
// Windows). Cada adaptador lo pasa a sus unidades según la escala del
// monitor. Un dedo de verdad mide unos 30-40 px lógicos.
const DiametroContacto = 36

// TouchScreen inyecta cuadros completos de hasta dos dedos. Prepare crea el
// dispositivo con antelación, porque el compositor puede tardar en verlo.
// Frame sustituye el estado anterior: toda ranura omitida queda levantada.
type TouchScreen interface {
	Prepare(ctx context.Context) error
	Frame(ctx context.Context, points []TouchPoint) error
}

// Display es un monitor del escritorio. X, Y, Width y Height van en las
// coordenadas del escritorio que usa el adaptador (Linux: píxeles lógicos de
// Hyprland; Windows: píxeles físicos de la pantalla virtual). Focused marca
// el monitor de la ventana con el foco.
type Display struct {
	Name          string
	X, Y          int
	Width, Height int
	Focused       bool
}

// Map lleva un punto normalizado del monitor (0..1) a las coordenadas del
// escritorio en píxeles.
func (d Display) Map(x, y float64) (float64, float64) {
	return float64(d.X) + x*float64(d.Width), float64(d.Y) + y*float64(d.Height)
}

// Displays lista los monitores y dirige a uno de ellos el puntero absoluto y
// la pantalla táctil: desde Target, (0,0)-(1,1) de MoveAbsolute y de Frame es
// ese monitor. Es opcional: sin él (o con un solo monitor) todo va a la
// pantalla de siempre.
type Displays interface {
	List(ctx context.Context) ([]Display, error)
	Target(ctx context.Context, name string) error
}

// Spaces hace lo que el sistema hace con los tres dedos en el panel táctil
// cuando hay un solo monitor: Hyprland cambia al escritorio de al lado (el
// de la derecha es el siguiente) y Windows cambia de app (Alt+Tab; hacia la
// izquierda, Alt+Mayús+Tab). Es opcional.
type Spaces interface {
	Switch(ctx context.Context, direction NavigationDirection) error
}

// TargetRole es la clase genérica de un objetivo de accesibilidad. Nunca
// contiene el nombre ni el texto que se ve en pantalla.
type TargetRole string

const (
	TargetRoleButton       TargetRole = "button"
	TargetRoleToggleButton TargetRole = "toggle_button"
	TargetRoleCheckBox     TargetRole = "check_box"
	TargetRoleComboBox     TargetRole = "combo_box"
	TargetRoleLink         TargetRole = "link"
	TargetRoleMenu         TargetRole = "menu"
	TargetRoleMenuItem     TargetRole = "menu_item"
	TargetRoleRadioButton  TargetRole = "radio_button"
	TargetRoleTab          TargetRole = "tab"
	TargetRoleSplitButton  TargetRole = "split_button"
	TargetRoleFrame        TargetRole = "frame"
	TargetRoleDialog       TargetRole = "dialog"
	TargetRoleWindow       TargetRole = "window"
	TargetRoleScrollBar    TargetRole = "scrollbar"
	TargetRoleSeparator    TargetRole = "separator"
)

// Target es un elemento que se puede tocar (botón, enlace, casilla…) de
// la ventana con el foco. X, Y, Width y Height van normalizados como los de
// TouchPoint: (0,0)-(1,1) es el monitor que recibe el táctil.
type Target struct {
	X, Y, Width, Height float64
	Role                TargetRole
}

// TargetBounds es un rectángulo normalizado al monitor que recibe el
// táctil. Un ancho o alto cero significa que el adaptador no pudo saberlo.
type TargetBounds struct {
	X, Y, Width, Height float64
}

// TargetSet son los objetivos de una ventana leídos de una vez.
// LogicalWidth y LogicalHeight son el tamaño de (0,0)-(1,1) en píxeles
// lógicos (los CSS a escala 1), para medir los radios del imán igual en
// cualquier escala.
type TargetSet struct {
	Window                      uint64 // la ventana con el foco cuando se leyó
	Targets                     []Target
	WindowBounds                TargetBounds
	LogicalWidth, LogicalHeight float64
}

// TargetFinder lee de la accesibilidad del sistema dónde están los
// elementos que se pueden tocar en la ventana con el foco (asistencia de
// puntería, nivel 2). FocusedWindow es barato y dice qué ventana tiene el
// foco, para leer Targets solo cuando cambia y no en cada cuadro. Targets
// es caro: recorre el árbol de accesibilidad. Es opcional: sin él no hay
// imán.
type TargetFinder interface {
	FocusedWindow(ctx context.Context) (uint64, error)
	Targets(ctx context.Context) (TargetSet, error)
}

// AccessibilityResult dice qué hizo Accessibility.Enable.
type AccessibilityResult struct {
	WasOn    bool // ya estaba encendida: no se tocó
	TurnedOn bool // Lazul la encendió ahora
}

// Accessibility prepara la accesibilidad del sistema para que las ventanas
// expongan sus botones al imán. En Linux hay que encenderla (AT-SPI:
// org.a11y.Status.IsEnabled) antes de que las apps arranquen; en Windows la
// interfaz ya está disponible sin preparación. Es opcional: sin ella el imán
// usa lo que el sistema tenga encendido.
type Accessibility interface {
	// Enable lee si la accesibilidad está encendida y, si no, la enciende.
	// Nunca la apaga: otras apps pueden estar usándola. Si no hay bus de
	// accesibilidad devuelve ErrSinAccesibilidad.
	Enable(ctx context.Context) (AccessibilityResult, error)
}

// BorderResize enciende y apaga el redimensionado al tocar el borde de una
// ventana (Hyprland: general:resize_on_border, con extend_border_grab_area).
// Con él encendido, un toque a menos de 8 px del borde redimensiona la
// ventana en vez de hacer clic; por eso Lazul lo apaga mientras el panel
// está encendido y devuelve el valor original al apagarlo. Es opcional:
// donde no exista, nil y nada cambia.
type BorderResize interface {
	// ResizeOnBorder dice si hoy está encendido.
	ResizeOnBorder(ctx context.Context) (bool, error)
	// SetResizeOnBorder lo cambia en caliente.
	SetResizeOnBorder(ctx context.Context, on bool) error
}

// Sistema junta las capacidades de un equipo. Cada paquete de sistema
// expone New() Sistema.
type Sistema struct {
	Screenshot      Screenshot
	Clipboard       Clipboard
	ActiveApp       ActiveApp
	LaunchApp       LaunchApp
	TactileActions  TactileActions
	AbsolutePointer AbsolutePointer
	TouchScreen     TouchScreen
	Displays        Displays
	Spaces          Spaces
	Targets         TargetFinder
	Accessibility   Accessibility
	BorderResize    BorderResize
}
