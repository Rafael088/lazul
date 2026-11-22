package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Tipos de línea del registro.
const (
	TipoInicio        = "inicio"        // la cámara empezó a dar cuadros (una sesión de uso)
	TipoLatido        = "latido"        // cada LatidoCada mientras la cámara da cuadros
	TipoFin           = "fin"           // la cámara se apagó o la app se cerró
	TipoPose          = "pose"          // la pose estable cambió
	TipoGesto         = "gesto"         // el clasificador reconoció un gesto
	TipoActivacion    = "activacion"    // una activación terminó: Resultado dice en qué
	TipoAccion        = "accion"        // lo que hizo el Enrutador con un gesto
	TipoRecibido      = "recibido"      // una Orden que llegó a este equipo
	TipoMarca         = "marca"         // la persona dice que la última activación fue sin querer (o no)
	TipoPrueba        = "prueba"        // empieza o termina la prueba de trabajo normal
	TipoEvitada       = "evitada"       // una mano quedó fuera de la zona; Resultado dice por qué
	TipoRendimiento   = "rendimiento"   // muestra periódica de fps, CPU e inferencia
	TipoPanel         = "panel"         // modo de entrada y apoyar/levantar dedos, nunca posiciones
	TipoPantalla      = "pantalla"      // tres dedos (disparo o por qué no) y cambios de monitor por el foco; nunca posiciones
	TipoMediaPipe     = "mediapipe"     // con qué delegado arrancó (GPU o CPU) y si la GPU falló
	TipoAccesibilidad = "accesibilidad" // accesibilidad de la sesión: si Lazul la encendió para el imán
)

// Resultados de una activación.
const (
	ActCaptura    = "captura"           // agarró y soltó
	ActGesto4     = "gesto4"            // mandó la ventana activa
	ActTactil     = "tactil"            // usó al menos una acción táctil
	ActSinGesto   = "sin_gesto"         // venció el modo activo sin ningún gesto
	ActCancelada  = "agarrar_cancelado" // agarró pero no soltó hacia un lado
	ActInterrumpe = "interrumpida"      // la cámara se apagó en medio
)

// LatidoCada: cada cuánto se anota que la cámara sigue dando cuadros. El uso
// se cuenta con estas líneas: un hueco mayor que HuecoMax no es uso.
const (
	LatidoCada = 30 * time.Second
	HuecoMax   = 90 * time.Second
)

// Linea es una línea del registro. A propósito no tiene campos para nada que
// salga de la pantalla o de la cámara: ni imágenes, ni puntos, ni URL, ni
// títulos ni rutas. Que solo dice el tipo de lo que se mandó (captura, url,
// archivo, app); Error, solo una clase (ver ClaseError).
type Linea struct {
	T         time.Time `json:"t"`
	Tipo      string    `json:"tipo"`
	Sesion    string    `json:"sesion,omitempty"`
	Pose      string    `json:"pose,omitempty"`
	Modo      string    `json:"modo,omitempty"`
	Gesto     string    `json:"gesto,omitempty"`
	Direccion string    `json:"dir,omitempty"`
	Confianza float32   `json:"conf,omitempty"`
	// Activacion numera las activaciones de la sesión (1, 2…): une el
	// gesto ACTIVAR, su resultado, las acciones y la marca de la persona.
	Activacion int    `json:"act,omitempty"`
	Resultado  string `json:"resultado,omitempty"`
	// Motivo: en una línea evitada con Resultado fuera_panel, por qué el
	// panel dejó fuera la mano (fuera_borde, cerca, lejos, sin_palma o
	// sin_panel). Nunca una posición.
	Motivo  string `json:"motivo,omitempty"`
	Que     string `json:"que,omitempty"`
	Destino string `json:"destino,omitempty"` // nombre del equipo (el que dio al emparejar)
	Error   string `json:"error,omitempty"`
	// App: en el gesto 4, el id del catálogo de la app mandada o
	// "fuera_del_catalogo". Nunca nombres, ejecutables, URL ni rutas.
	App     string `json:"app,omitempty"`
	Ms      int64  `json:"ms,omitempty"`
	Cuadros int    `json:"cuadros,omitempty"`
	ConMano int    `json:"con_mano,omitempty"`
	Dedos   int    `json:"dedos,omitempty"` // cantidad apoyada; nunca posiciones
	// Rol, Ancho y Alto describen el objetivo genérico que activó el imán.
	// No incluyen su texto ni su posición en la pantalla.
	Rol   string `json:"rol,omitempty"`
	Ancho int    `json:"ancho,omitempty"`
	Alto  int    `json:"alto,omitempty"`
	// Palmas: en un intento de los tres dedos, el recorrido horizontal
	// alcanzado desde donde empezó la pose, en palmas. Nunca una posición.
	Palmas   float64 `json:"palmas,omitempty"`
	FPS      float64 `json:"fps,omitempty"`
	CPUCore  float64 `json:"cpu_core,omitempty"`
	CPUTotal float64 `json:"cpu_total,omitempty"`
	ManoMs   float64 `json:"mano_ms,omitempty"`
	PoseMs   float64 `json:"pose_ms,omitempty"`
	Manos    float64 `json:"manos_media,omitempty"` // cantidad media detectada; nunca puntos
	PoseCada float64 `json:"pose_cada,omitempty"`   // cuadros por ejecución real de Pose
	// ModoManos: en rendimiento, cómo se siguieron las manos (busca o dos).
	// Busquedas: cuántas veces se buscó la segunda en esa muestra.
	ModoManos string `json:"modo_manos,omitempty"`
	Busquedas int    `json:"busquedas,omitempty"`
	// Prueba: en la prueba de trabajo normal (toda activación cuenta como
	// falsa y no se manda nada).
	Prueba bool `json:"prueba,omitempty"`
	// Falsa, en una marca: true = sin querer, false = a propósito.
	Falsa *bool `json:"falsa,omitempty"`
}

// Registro añade líneas a un archivo JSONL (0600). Es seguro usarlo desde
// varias gorrutinas; un Registro nil no anota nada.
type Registro struct {
	// Espejo, si no es nil, recibe una copia de cada línea (con su \n).
	Espejo func([]byte)

	mu    sync.Mutex
	f     *os.File
	ruta  string
	reloj func() time.Time
}

// DirEstado es la carpeta de estado del usuario para Lazul: en Linux
// $XDG_STATE_HOME/lazul (o ~/.local/state/lazul), en Windows
// %LOCALAPPDATA%\lazul.
func DirEstado() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "lazul"), nil
		}
		d, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "lazul"), nil
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "lazul"), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".local", "state", "lazul"), nil
}

// NombreRegistro es el archivo del registro dentro de la carpeta de estado.
const NombreRegistro = "factibilidad.jsonl"

// AbrirRegistro abre (o crea) el registro en ruta, para añadir al final.
func AbrirRegistro(ruta string) (*Registro, error) {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(ruta, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Registro{f: f, ruta: ruta}, nil
}

// Ruta del archivo.
func (r *Registro) Ruta() string {
	if r == nil {
		return ""
	}
	return r.ruta
}

func (r *Registro) ahora() time.Time {
	if r.reloj != nil {
		return r.reloj()
	}
	return time.Now()
}

// Anotar escribe l; si l.T es cero le pone la hora de ahora.
func (r *Registro) Anotar(l Linea) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return
	}
	if l.T.IsZero() {
		l.T = r.ahora()
	}
	l.T = l.T.Round(time.Millisecond)
	b, err := json.Marshal(l)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = r.f.Write(b)
	if r.Espejo != nil {
		r.Espejo(b)
	}
}

func (r *Registro) Cerrar() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}

// LeerRegistro lee todas las líneas; las que no se entienden se saltan (un
// corte a medio escribir no estropea el resto).
func LeerRegistro(rd io.Reader) ([]Linea, error) {
	var ls []Linea
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var l Linea
		if json.Unmarshal(sc.Bytes(), &l) == nil && l.Tipo != "" {
			ls = append(ls, l)
		}
	}
	return ls, sc.Err()
}

// LeerRegistroDe lee el archivo en ruta (vacío si no existe).
func LeerRegistroDe(ruta string) ([]Linea, error) {
	f, err := os.Open(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LeerRegistro(f)
}
