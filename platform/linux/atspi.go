//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Rafael088/lazul/platform"
	"github.com/godbus/dbus/v5"
)

// Lectura del árbol de accesibilidad por AT-SPI. Las apps se registran en
// un bus D-Bus aparte (el de org.a11y.Bus); la de la ventana con el foco se
// reconoce por su PID, su ventana es el marco con el estado ACTIVE (o con
// el título de la ventana) y los objetivos se piden de una vez con
// Collection.GetMatches. Sus rectángulos se piden todos a la vez sin
// esperar uno por uno, así que la lectura son unas pocas idas y vueltas
// aunque la página tenga cientos de enlaces.
//
// Firefox se registra solo. Chromium y las apps de Electron solo exponen el
// contenido de la página con --force-renderer-accessibility o con
// ACCESSIBILITY_ENABLED=1 en el entorno (desktop/README.md).

const (
	ifazAccesible  = "org.a11y.atspi.Accessible"
	ifazComponente = "org.a11y.atspi.Component"
	ifazColeccion  = "org.a11y.atspi.Collection"
	rutaRaizA11y   = dbus.ObjectPath("/org/a11y/atspi/accessible/root")
	busRegistro    = "org.a11y.atspi.Registry"

	coordVentana    = uint32(1) // ATSPI_COORD_TYPE_WINDOW
	coincideTodo    = int32(1)  // ATSPI_Collection_MATCH_ALL
	coincideAlguno  = int32(2)  // ATSPI_Collection_MATCH_ANY
	ordenCanonico   = uint32(1) // ATSPI_Collection_SORT_ORDER_CANONICAL
	estadoActivo    = 1         // ATSPI_STATE_ACTIVE
	estadoSensible  = 24        // ATSPI_STATE_SENSITIVE
	estadoMostrado  = 25        // ATSPI_STATE_SHOWING
	rolMarco        = 23        // ATSPI_ROLE_FRAME
	rolDialogo      = 16        // ATSPI_ROLE_DIALOG
	rolVentana      = 69        // ATSPI_ROLE_WINDOW
	maxNodosRecorre = 4000      // sin Collection: nodos que se miran a lo sumo
)

// rolesObjetivo son los AtspiRole que cuentan como objetivo
// (atspi-constants.h).
var rolesObjetivo = []int{
	43,  // BUTTON (PUSH_BUTTON)
	62,  // TOGGLE_BUTTON
	88,  // LINK
	7,   // CHECK_BOX
	44,  // RADIO_BUTTON
	37,  // PAGE_TAB
	33,  // MENU (los de la barra de menús)
	35,  // MENU_ITEM
	8,   // CHECK_MENU_ITEM
	45,  // RADIO_MENU_ITEM
	11,  // COMBO_BOX
	129, // PUSH_BUTTON_MENU
}

// rolObjetivo traduce los números propios de AT-SPI a categorías comunes
// con Windows. No se propaga ningún nombre del elemento.
func rolObjetivo(r uint32) platform.TargetRole {
	switch r {
	case 43:
		return platform.TargetRoleButton
	case 62:
		return platform.TargetRoleToggleButton
	case 88:
		return platform.TargetRoleLink
	case 7:
		return platform.TargetRoleCheckBox
	case 44:
		return platform.TargetRoleRadioButton
	case 37:
		return platform.TargetRoleTab
	case 33:
		return platform.TargetRoleMenu
	case 35, 8, 45:
		return platform.TargetRoleMenuItem
	case 11:
		return platform.TargetRoleComboBox
	case 129:
		return platform.TargetRoleSplitButton
	case rolMarco:
		return platform.TargetRoleFrame
	case rolDialogo:
		return platform.TargetRoleDialog
	case rolVentana:
		return platform.TargetRoleWindow
	case 48: // ATSPI_ROLE_SCROLL_BAR
		return platform.TargetRoleScrollBar
	case 50: // ATSPI_ROLE_SEPARATOR
		return platform.TargetRoleSeparator
	default:
		return ""
	}
}

// refA11y es un objeto de AT-SPI: el nombre de su conexión y su ruta.
type refA11y struct {
	Bus  string
	Ruta dbus.ObjectPath
}

// reglaA11y es el AtspiMatchRule de Collection.GetMatches, con la firma
// (aiia{ss}iaiiasib).
type reglaA11y struct {
	Estados        []int32
	EstadosTipo    int32
	Atributos      map[string]string
	AtributosTipo  int32
	Roles          []int32
	RolesTipo      int32
	Interfaces     []string
	InterfacesTipo int32
	Invertir       bool
}

// bits arma el conjunto de bits de AT-SPI: n palabras de 32 bits, el bit
// i%32 de la palabra i/32.
func bits(n int, valores ...int) []int32 {
	w := make([]uint32, n)
	for _, v := range valores {
		if v >= 0 && v/32 < n {
			w[v/32] |= 1 << (v % 32)
		}
	}
	r := make([]int32, n)
	for i, x := range w {
		r[i] = int32(x)
	}
	return r
}

// tieneEstado lee el estado s del au de GetState.
func tieneEstado(estados []uint32, s int) bool {
	return s/32 < len(estados) && estados[s/32]&(1<<(s%32)) != 0
}

// reglaObjetivos: mostrados y sensibles (habilitados), de alguno de los
// roles de rolesObjetivo.
func reglaObjetivos() reglaA11y {
	return reglaA11y{
		Estados:        bits(2, estadoMostrado, estadoSensible),
		EstadosTipo:    coincideTodo,
		Atributos:      map[string]string{},
		AtributosTipo:  coincideTodo,
		Roles:          bits(5, rolesObjetivo...),
		RolesTipo:      coincideAlguno,
		Interfaces:     []string{},
		InterfacesTipo: coincideTodo,
	}
}

// esRolObjetivo dice si r está en rolesObjetivo.
func esRolObjetivo(r uint32) bool {
	for _, x := range rolesObjetivo {
		if uint32(x) == r {
			return true
		}
	}
	return false
}

// marcoA11y es una ventana de nivel superior de una app.
type marcoA11y struct {
	Nombre string
	Rol    uint32
	Activo bool
}

// elegirMarco da el índice de la ventana de la app que tiene el foco: la
// que tiene ACTIVE; si ninguna (o varias), la que se llama como la ventana
// de Hyprland; si la app tiene una sola, esa. -1 si no se sabe.
func elegirMarco(ms []marcoA11y, titulo string) int {
	activas := -1
	n := 0
	for i, m := range ms {
		if m.Activo {
			activas, n = i, n+1
		}
	}
	if n == 1 {
		return activas
	}
	titulo = strings.TrimSpace(titulo)
	for i, m := range ms {
		if titulo != "" && strings.TrimSpace(m.Nombre) == titulo && (n == 0 || m.Activo) {
			return i
		}
	}
	if n > 1 {
		return activas
	}
	if len(ms) == 1 {
		return 0
	}
	return -1
}

// a11yDBus es la fuente de verdad: una conexión al bus de accesibilidad que
// se abre a la primera lectura y se rehace si falla.
type a11yDBus struct {
	mu   sync.Mutex
	conn *dbus.Conn
	apps map[int]string // PID → nombre de la app en el bus
	// opciones de la conexión (las pruebas, por TCP, usan ANONYMOUS).
	opciones []dbus.ConnOption
}

var _ fuenteA11y = (*a11yDBus)(nil)

// errSinA11y: la app de la ventana no está en el bus de accesibilidad.
var errSinA11y = errors.New("la app de la ventana no está en el bus de accesibilidad (Chromium y Electron: --force-renderer-accessibility o ACCESSIBILITY_ENABLED=1)")

func (a *a11yDBus) conexion() (*dbus.Conn, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil && a.conn.Connected() {
		return a.conn, nil
	}
	dir := os.Getenv("AT_SPI_BUS_ADDRESS")
	if dir == "" {
		s, err := dbus.ConnectSessionBus()
		if err != nil {
			return nil, fmt.Errorf("bus de la sesión: %w", err)
		}
		err = s.Object("org.a11y.Bus", "/org/a11y/bus").Call("org.a11y.Bus.GetAddress", 0).Store(&dir)
		s.Close()
		if err != nil {
			return nil, fmt.Errorf("dirección del bus de accesibilidad: %w", err)
		}
	}
	c, err := dbus.Connect(dir, a.opciones...)
	if err != nil {
		return nil, fmt.Errorf("bus de accesibilidad: %w", err)
	}
	a.conn, a.apps = c, nil
	return c, nil
}

func (a *a11yDBus) olvidar(c *dbus.Conn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == c {
		a.conn.Close()
		a.conn, a.apps = nil, nil
	}
}

func (a *a11yDBus) Leer(ctx context.Context, pid int, titulo string) (rectA11y, []rectA11y, error) {
	c, err := a.conexion()
	if err != nil {
		return rectA11y{}, nil, err
	}
	marco, rs, err := a.leer(ctx, c, pid, titulo)
	if err != nil && !c.Connected() {
		a.olvidar(c)
	}
	return marco, rs, err
}

func (a *a11yDBus) leer(ctx context.Context, c *dbus.Conn, pid int, titulo string) (rectA11y, []rectA11y, error) {
	bus, err := a.appDe(ctx, c, pid)
	if err != nil {
		return rectA11y{}, nil, err
	}
	marcos, err := hijos(ctx, c, refA11y{bus, rutaRaizA11y})
	if err != nil {
		a.mu.Lock()
		delete(a.apps, pid) // la app se cerró o cambió de nombre
		a.mu.Unlock()
		return rectA11y{}, nil, fmt.Errorf("ventanas de la app: %w", err)
	}
	datos := make([]marcoA11y, len(marcos))
	estados := lote(ctx, c, marcos, ifazAccesible+".GetState")
	roles := lote(ctx, c, marcos, ifazAccesible+".GetRole")
	nombres := lote(ctx, c, marcos, "org.freedesktop.DBus.Properties.Get", ifazAccesible, "Name")
	for i := range marcos {
		var e []uint32
		if estados[i] != nil && estados[i].Store(&e) == nil {
			datos[i].Activo = tieneEstado(e, estadoActivo)
		}
		if roles[i] != nil {
			_ = roles[i].Store(&datos[i].Rol)
		}
		var v dbus.Variant
		if nombres[i] != nil && nombres[i].Store(&v) == nil {
			datos[i].Nombre, _ = v.Value().(string)
		}
	}
	i := elegirMarco(datos, titulo)
	if i < 0 {
		return rectA11y{}, nil, fmt.Errorf("no se sabe cuál de las %d ventanas de la app tiene el foco", len(marcos))
	}
	m := marcos[i]
	var ext struct{ X, Y, W, H int32 }
	if err := objeto(c, m).CallWithContext(ctx, ifazComponente+".GetExtents", 0, coordVentana).Store(&ext); err != nil {
		return rectA11y{}, nil, fmt.Errorf("tamaño de la ventana: %w", err)
	}
	marco := rectA11y{x: float64(ext.X), y: float64(ext.Y), ancho: float64(ext.W), alto: float64(ext.H)}

	var refs []refA11y
	err = objeto(c, m).CallWithContext(ctx, ifazColeccion+".GetMatches", 0,
		reglaObjetivos(), ordenCanonico, int32(maxElementosA11y), true).Store(&refs)
	if err != nil {
		if ctx.Err() != nil {
			return rectA11y{}, nil, err
		}
		// Sin Collection (GTK 4, Qt): recorrer el árbol a mano.
		refs, err = recorrer(ctx, c, m)
		if err != nil {
			return rectA11y{}, nil, err
		}
	}
	if len(refs) > maxElementosA11y {
		refs = refs[:maxElementosA11y]
	}
	rs := make([]rectA11y, 0, len(refs))
	extensiones := lote(ctx, c, refs, ifazComponente+".GetExtents", coordVentana)
	rolesObjetivo := lote(ctx, c, refs, ifazAccesible+".GetRole")
	for i, r := range extensiones {
		if r == nil || r.Store(&ext) != nil {
			continue
		}
		var rol uint32
		if rolesObjetivo[i] != nil {
			_ = rolesObjetivo[i].Store(&rol)
		}
		rs = append(rs, rectA11y{x: float64(ext.X), y: float64(ext.Y), ancho: float64(ext.W), alto: float64(ext.H), rol: rolObjetivo(rol)})
	}
	if err := ctx.Err(); err != nil {
		return rectA11y{}, nil, err
	}
	return marco, rs, nil
}

// appDe busca la app con ese PID entre las registradas.
func (a *a11yDBus) appDe(ctx context.Context, c *dbus.Conn, pid int) (string, error) {
	if pid <= 0 {
		return "", errSinA11y
	}
	a.mu.Lock()
	bus, ok := a.apps[pid]
	a.mu.Unlock()
	if ok {
		return bus, nil
	}
	apps, err := hijos(ctx, c, refA11y{busRegistro, rutaRaizA11y})
	if err != nil {
		return "", fmt.Errorf("apps del bus de accesibilidad: %w", err)
	}
	daemon := make([]refA11y, len(apps))
	for i := range apps {
		daemon[i] = refA11y{"org.freedesktop.DBus", "/org/freedesktop/DBus"}
	}
	pids := loteCon(ctx, c, daemon, "org.freedesktop.DBus.GetConnectionUnixProcessID",
		func(i int) []any { return []any{apps[i].Bus} })
	mapa := make(map[int]string, len(apps))
	for i, call := range pids {
		var p uint32
		if call != nil && call.Store(&p) == nil && p > 0 {
			mapa[int(p)] = apps[i].Bus
		}
	}
	a.mu.Lock()
	a.apps = mapa
	a.mu.Unlock()
	if bus, ok := mapa[pid]; ok {
		return bus, nil
	}
	return "", errSinA11y
}

func objeto(c *dbus.Conn, r refA11y) dbus.BusObject { return c.Object(r.Bus, r.Ruta) }

func hijos(ctx context.Context, c *dbus.Conn, r refA11y) ([]refA11y, error) {
	var hs []refA11y
	err := objeto(c, r).CallWithContext(ctx, ifazAccesible+".GetChildren", 0).Store(&hs)
	return hs, err
}

// enVuelo es cuántas llamadas de un lote esperan respuesta a la vez: un
// dbus-daemon sin configurar corta las de más (el de accesibilidad admite
// 50 000, pero no hace falta apurarlo).
const enVuelo = 100

// lote manda el mismo método a todos los objetos sin esperar a cada uno
// (hasta enVuelo a la vez) y devuelve las respuestas en el mismo orden (nil
// si falló o no llegó). godbus cierra cada llamada una sola vez por el
// canal, también cuando ctx termina (con su error), así que esto nunca
// espera más que el contexto.
func lote(ctx context.Context, c *dbus.Conn, refs []refA11y, metodo string, args ...any) []*dbus.Call {
	return loteCon(ctx, c, refs, metodo, func(int) []any { return args })
}

// loteCon es lote con los argumentos de cada llamada.
func loteCon(ctx context.Context, c *dbus.Conn, refs []refA11y, metodo string, args func(i int) []any) []*dbus.Call {
	calls := make([]*dbus.Call, len(refs))
	ch := make(chan *dbus.Call, min(len(refs), enVuelo))
	sig := 0
	for ; sig < len(refs) && sig < enVuelo; sig++ {
		calls[sig] = objeto(c, refs[sig]).GoWithContext(ctx, metodo, 0, ch, args(sig)...)
	}
	for range refs {
		<-ch
		if sig < len(refs) {
			calls[sig] = objeto(c, refs[sig]).GoWithContext(ctx, metodo, 0, ch, args(sig)...)
			sig++
		}
	}
	for i, call := range calls {
		if call.Err != nil {
			calls[i] = nil
		}
	}
	return calls
}

// recorrer baja por el árbol nivel a nivel (cada nivel es un lote) cuando
// la app no tiene Collection: solo por los mostrados, hasta maxNodosRecorre.
func recorrer(ctx context.Context, c *dbus.Conn, raiz refA11y) ([]refA11y, error) {
	var objetivos []refA11y
	nivel := []refA11y{raiz}
	vistos := 0
	for len(nivel) > 0 && vistos < maxNodosRecorre {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		vistos += len(nivel)
		estados := lote(ctx, c, nivel, ifazAccesible+".GetState")
		roles := lote(ctx, c, nivel, ifazAccesible+".GetRole")
		var mostrados []refA11y
		for i, r := range nivel {
			var e []uint32
			if estados[i] == nil || estados[i].Store(&e) != nil || !tieneEstado(e, estadoMostrado) {
				continue
			}
			var rol uint32
			if roles[i] != nil && roles[i].Store(&rol) == nil && esRolObjetivo(rol) {
				if tieneEstado(e, estadoSensible) {
					objetivos = append(objetivos, r)
				}
				continue // dentro de un botón no hay más botones
			}
			mostrados = append(mostrados, r)
		}
		var siguiente []refA11y
		for _, h := range lote(ctx, c, mostrados, ifazAccesible+".GetChildren") {
			var hs []refA11y
			if h != nil && h.Store(&hs) == nil {
				siguiente = append(siguiente, hs...)
			}
		}
		nivel = siguiente
		if len(objetivos) >= maxElementosA11y {
			break
		}
	}
	return objetivos, nil
}
