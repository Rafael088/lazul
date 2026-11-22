//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Rafael088/lazul/platform"
)

// objetivosHyprland implementa platform.TargetFinder en Hyprland: la ventana
// con el foco sale de hyprctl activewindow, sus botones y enlaces del árbol
// de accesibilidad (AT-SPI por D-Bus, atspi.go) y la posición de la ventana
// y la escala del monitor de hyprctl, porque en Wayland AT-SPI solo sabe
// dónde está cada cosa dentro de su ventana. Nada de Hyprland (barra,
// bordes, fondos) está en el árbol de la app, así que nunca es objetivo.
type objetivosHyprland struct {
	hypr    func(ctx context.Context, comando string) ([]byte, error)
	a11y    fuenteA11y
	puntero *punteroVirtual
}

var _ platform.TargetFinder = objetivosHyprland{}

const (
	// maxObjetivosA11y es lo más que se espera una lectura entera (igual
	// que maxObjetivosUIA en Windows).
	maxObjetivosA11y = 800 * time.Millisecond
	// maxElementosA11y: más allá de esto se ignoran (una página enorme).
	maxElementosA11y = 800
	// maxHyprctl es el tope de una consulta a Hyprland.
	maxHyprctl = 300 * time.Millisecond
)

// rectA11y es un objetivo de AT-SPI: rol genérico y rectángulo (izquierda,
// arriba, ancho y alto). Nunca conserva el nombre accesible.
type rectA11y struct {
	x, y, ancho, alto float64
	rol               platform.TargetRole
}

// fuenteA11y lee el árbol de accesibilidad de una ventana. marco es la
// ventana misma y objetivos lo que se puede tocar, los dos en coordenadas
// de la ventana (ATSPI_COORD_TYPE_WINDOW).
type fuenteA11y interface {
	Leer(ctx context.Context, pid int, titulo string) (marco rectA11y, objetivos []rectA11y, err error)
}

// ventanaHyprland son los campos de hyprctl activewindow -j que hacen falta.
type ventanaHyprland struct {
	Direccion string     `json:"address"`
	En        [2]float64 `json:"at"`
	Tamano    [2]float64 `json:"size"`
	Monitor   int        `json:"monitor"`
	PID       int        `json:"pid"`
	Titulo    string     `json:"title"`
}

// leerVentanaActiva devuelve la ventana con el foco y su id (la dirección
// de Hyprland). Sin ventana, ErrSinVentana.
func leerVentanaActiva(datos []byte) (ventanaHyprland, uint64, error) {
	t := strings.TrimSpace(string(datos))
	if t == "" || t == "{}" || !strings.HasPrefix(t, "{") {
		return ventanaHyprland{}, 0, platform.ErrSinVentana
	}
	var v ventanaHyprland
	if err := json.Unmarshal(datos, &v); err != nil {
		return ventanaHyprland{}, 0, fmt.Errorf("leer la ventana activa de hyprctl: %w", err)
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(v.Direccion, "0x"), 16, 64)
	if err != nil || id == 0 {
		return ventanaHyprland{}, 0, platform.ErrSinVentana
	}
	return v, id, nil
}

// FocusedWindow pregunta a Hyprland por su socket (sin abrir un proceso:
// corre cada 250 ms).
func (o objetivosHyprland) FocusedWindow(ctx context.Context) (uint64, error) {
	datos, err := o.hypr(ctx, "activewindow")
	if err != nil {
		return 0, err
	}
	_, id, err := leerVentanaActiva(datos)
	return id, err
}

func (o objetivosHyprland) Targets(ctx context.Context) (platform.TargetSet, error) {
	ctx, cancel := context.WithTimeout(ctx, maxObjetivosA11y)
	defer cancel()
	datos, err := o.hypr(ctx, "activewindow")
	if err != nil {
		return platform.TargetSet{}, err
	}
	v, id, err := leerVentanaActiva(datos)
	if err != nil {
		return platform.TargetSet{}, err
	}
	datos, err = o.hypr(ctx, "monitors")
	if err != nil {
		return platform.TargetSet{}, err
	}
	ms, err := monitoresLogicos(datos)
	if err != nil {
		return platform.TargetSet{}, err
	}
	var destino *platform.Display
	if o.puntero != nil {
		destino = o.puntero.monitorDestino()
	}
	tactil, propio, ok := monitoresDeVentana(ms, v.Monitor, destino)
	if !ok {
		return platform.TargetSet{}, errors.New("hyprctl no da ningún monitor encendido")
	}
	if o.a11y == nil {
		return platform.TargetSet{}, platform.ErrNoImplementado
	}
	marco, rs, err := o.a11y.Leer(ctx, v.PID, v.Titulo)
	if err != nil {
		if ctx.Err() != nil {
			return platform.TargetSet{}, fmt.Errorf("AT-SPI no contestó a tiempo: %w", err)
		}
		return platform.TargetSet{}, err
	}
	s := aPantalla(v, propio.Escala, marco, rs, tactil.Display)
	s.Window = id
	return s, nil
}

// monitoresDeVentana elige el monitor que recibe el táctil (el de
// Displays.Target; si no, el que tiene el foco; si no, el de la ventana) y
// el monitor de la ventana, cuya escala hace falta para las apps que dan
// píxeles físicos.
func monitoresDeVentana(ms []monitorLogico, idVentana int, destino *platform.Display) (tactil, propio monitorLogico, ok bool) {
	if len(ms) == 0 {
		return monitorLogico{}, monitorLogico{}, false
	}
	propio, tactil = ms[0], ms[0]
	hayPropio := false
	for _, m := range ms {
		if m.ID == idVentana {
			propio, hayPropio = m, true
		}
	}
	switch {
	case destino != nil && porNombreLogico(ms, destino.Name, &tactil):
	case porFoco(ms, &tactil):
	case hayPropio:
		tactil = propio
	}
	return tactil, propio, true
}

func porNombreLogico(ms []monitorLogico, nombre string, m *monitorLogico) bool {
	for _, x := range ms {
		if x.Name == nombre {
			*m = x
			return true
		}
	}
	return false
}

func porFoco(ms []monitorLogico, m *monitorLogico) bool {
	for _, x := range ms {
		if x.Focused {
			*m = x
			return true
		}
	}
	return false
}

// aPantalla lleva los rectángulos de AT-SPI (coordenadas de la ventana) a
// (0,0)-(1,1) del monitor que recibe el táctil:
//
//   - Si el marco mide la ventana de Hyprland por la escala del monitor, la
//     app da píxeles físicos (XWayland sin escalar): se dividen por ella.
//   - El origen es el del marco: si el marco no empieza en (0,0) se le
//     resta. Si es más grande que la ventana de Hyprland por igual a los
//     dos lados (la sombra de una ventana GTK con decoración propia), se
//     quita ese margen.
//   - Fuera queda lo que no tiene tamaño, lo que tiene el centro fuera de la
//     ventana (desplazado fuera de la vista o en otra pestaña) y lo que
//     ocupa casi toda la ventana (el borde de la ventana no es un botón).
func aPantalla(v ventanaHyprland, escala float64, marco rectA11y, rs []rectA11y, m platform.Display) platform.TargetSet {
	s := platform.TargetSet{LogicalWidth: float64(m.Width), LogicalHeight: float64(m.Height)}
	vx, vy, vw, vh := v.En[0], v.En[1], v.Tamano[0], v.Tamano[1]
	if vw <= 1 || vh <= 1 || m.Width <= 1 || m.Height <= 1 {
		return s
	}
	s.WindowBounds = platform.TargetBounds{
		X:      (vx - float64(m.X)) / float64(m.Width),
		Y:      (vy - float64(m.Y)) / float64(m.Height),
		Width:  vw / float64(m.Width),
		Height: vh / float64(m.Height),
	}
	k := 1.0
	if escala > 1.01 && marco.ancho > 0 && math.Abs(marco.ancho/vw-escala) < 0.05*escala {
		k = escala
	}
	mx, my, mw, mh := marco.x/k, marco.y/k, marco.ancho/k, marco.alto/k
	dx, dy := mx, my
	if sx, sy := (mw-vw)/2, (mh-vh)/2; sx >= 1 && sy >= 1 && sx <= 100 && sy <= 100 && math.Abs(mx) < 1 && math.Abs(my) < 1 {
		dx, dy = mx+sx, my+sy
	}
	for _, r := range rs {
		w, h := r.ancho/k, r.alto/k
		if w <= 0 || h <= 0 {
			continue
		}
		if w >= 0.9*vw && h >= 0.9*vh {
			continue
		}
		x, y := vx+r.x/k-dx, vy+r.y/k-dy
		if cx, cy := x+w/2, y+h/2; cx < vx || cy < vy || cx > vx+vw || cy > vy+vh {
			continue
		}
		nx, ny := (x-float64(m.X))/float64(m.Width), (y-float64(m.Y))/float64(m.Height)
		nw, nh := w/float64(m.Width), h/float64(m.Height)
		if nx+nw < 0 || ny+nh < 0 || nx > 1 || ny > 1 {
			continue
		}
		s.Targets = append(s.Targets, platform.Target{X: nx, Y: ny, Width: nw, Height: nh, Role: r.rol})
	}
	return s
}

// consultarHyprland pide un JSON a Hyprland por su socket de órdenes, como
// hyprctl -j pero sin abrir un proceso. Si el socket no responde, usa
// hyprctl.
func consultarHyprland(ctx context.Context, comando string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, maxHyprctl)
	defer cancel()
	if datos, err := socketHyprland(ctx, comando); err == nil {
		return datos, nil
	}
	datos, err := exec.CommandContext(ctx, "hyprctl", comando, "-j").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("consultar %s con hyprctl: %w", comando, err)
	}
	return datos, nil
}

func socketHyprland(ctx context.Context, comando string) ([]byte, error) {
	firma := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if firma == "" || strings.ContainsAny(firma, "/\x00") {
		return nil, errors.New("sin HYPRLAND_INSTANCE_SIGNATURE")
	}
	var rutas []string
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		rutas = append(rutas, filepath.Join(d, "hypr", firma, ".socket.sock"))
	}
	rutas = append(rutas, filepath.Join("/tmp/hypr", firma, ".socket.sock"))
	var ultimo error
	for _, r := range rutas {
		var d net.Dialer
		c, err := d.DialContext(ctx, "unix", r)
		if err != nil {
			ultimo = err
			continue
		}
		if t, ok := ctx.Deadline(); ok {
			_ = c.SetDeadline(t)
		}
		_, err = io.WriteString(c, "j/"+comando)
		var datos []byte
		if err == nil {
			datos, err = io.ReadAll(io.LimitReader(c, 4<<20))
		}
		c.Close()
		if err != nil {
			return nil, err
		}
		return datos, nil
	}
	return nil, ultimo
}
