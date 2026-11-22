//go:build linux

package linux

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
	"github.com/godbus/dbus/v5"
)

// busPrueba arranca un dbus-daemon propio por TCP (sin tocar el bus del
// escritorio) y da su dirección. Sin dbus-daemon, la prueba se salta.
func busPrueba(t testing.TB) string {
	t.Helper()
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("sin dbus-daemon")
	}
	conf := filepath.Join(t.TempDir(), "bus.conf")
	if err := os.WriteFile(conf, []byte(`<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig><type>session</type><listen>tcp:host=127.0.0.1,bind=127.0.0.1,port=0</listen>
<auth>ANONYMOUS</auth><allow_anonymous/>
<policy context="default"><allow send_destination="*" eavesdrop="true"/><allow eavesdrop="true"/><allow own="*"/></policy></busconfig>`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("dbus-daemon", "--config-file="+conf, "--print-address", "--nofork")
	salida, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("dbus-daemon no arranca: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	linea, err := bufio.NewReader(salida).ReadString('\n')
	if err != nil {
		t.Skipf("dbus-daemon no da dirección: %v", err)
	}
	return strings.TrimSpace(linea)
}

func conectarPrueba(t testing.TB, dir string) *dbus.Conn {
	t.Helper()
	c, err := dbus.Connect(dir, dbus.WithAuth(dbus.AuthAnonymous()))
	if err != nil {
		t.Skipf("no se conecta al bus de prueba: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

type extension struct{ X, Y, W, H int32 }

// nodoFalso es un objeto de AT-SPI de la app falsa.
type nodoFalso struct {
	hijos   []refA11y
	estado  []uint32
	rol     uint32
	nombre  string
	ext     extension
	matches []refA11y

	mu    *sync.Mutex
	regla *reglaA11y
}

func (n *nodoFalso) GetChildren() ([]refA11y, *dbus.Error) { return n.hijos, nil }
func (n *nodoFalso) GetState() ([]uint32, *dbus.Error)     { return n.estado, nil }
func (n *nodoFalso) GetRole() (uint32, *dbus.Error)        { return n.rol, nil }
func (n *nodoFalso) GetExtents(coord uint32) (extension, *dbus.Error) {
	if coord != coordVentana {
		return extension{}, dbus.MakeFailedError(os.ErrInvalid)
	}
	return n.ext, nil
}
func (n *nodoFalso) Get(_, prop string) (dbus.Variant, *dbus.Error) {
	return dbus.MakeVariant(n.nombre), nil
}
func (n *nodoFalso) GetMatches(r reglaA11y, orden uint32, cuantos int32, recorrer bool) ([]refA11y, *dbus.Error) {
	n.mu.Lock()
	n.regla = &r
	n.mu.Unlock()
	return n.matches, nil
}

func estados(ss ...int) []uint32 {
	b := bits(2, ss...)
	return []uint32{uint32(b[0]), uint32(b[1])}
}

// appFalsa publica en c un árbol con dos ventanas: «otra» y «Pista»
// (activa), con dos botones y un enlace. conColeccion decide si la ventana
// activa contesta Collection.GetMatches o hay que recorrerla.
func appFalsa(t testing.TB, c *dbus.Conn, conColeccion bool) *nodoFalso {
	t.Helper()
	yo := c.Names()[0]
	ref := func(r string) refA11y { return refA11y{yo, dbus.ObjectPath(r)} }
	mu := &sync.Mutex{}
	visible := estados(estadoMostrado, estadoSensible)
	nodos := map[string]*nodoFalso{
		"/org/a11y/atspi/accessible/root": {hijos: []refA11y{ref("/f1"), ref("/f2")}, rol: 75},
		"/f1":                             {nombre: "otra", rol: rolMarco, estado: visible, ext: extension{0, 0, 500, 400}},
		"/f2": {nombre: "Pista — Mozilla Firefox", rol: rolMarco, estado: estados(estadoActivo, estadoMostrado, estadoSensible),
			ext: extension{0, 0, 800, 600}, hijos: []refA11y{ref("/doc")},
			matches: []refA11y{ref("/b1"), ref("/b2"), ref("/l1")}},
		"/doc":    {rol: 95, estado: visible, hijos: []refA11y{ref("/b1"), ref("/b2"), ref("/sec"), ref("/oculto")}},
		"/sec":    {rol: 85, estado: visible, hijos: []refA11y{ref("/l1"), ref("/gris")}},
		"/b1":     {rol: 43, estado: visible, ext: extension{10, 10, 30, 20}},
		"/b2":     {rol: 62, estado: visible, ext: extension{50, 10, 30, 20}},
		"/l1":     {rol: 88, estado: visible, ext: extension{10, 100, 200, 18}},
		"/gris":   {rol: 43, estado: estados(estadoMostrado), ext: extension{10, 200, 30, 20}},
		"/oculto": {rol: 85, estado: estados(estadoSensible), hijos: []refA11y{ref("/b3")}},
		"/b3":     {rol: 43, estado: visible, ext: extension{10, 300, 30, 20}},
	}
	for ruta, n := range nodos {
		n.mu = mu
		p := dbus.ObjectPath(ruta)
		for _, ifaz := range []string{ifazAccesible, ifazComponente, "org.freedesktop.DBus.Properties"} {
			if err := c.Export(n, p, ifaz); err != nil {
				t.Fatal(err)
			}
		}
		if conColeccion && ruta == "/f2" {
			if err := c.Export(n, p, ifazColeccion); err != nil {
				t.Fatal(err)
			}
		}
	}
	return nodos["/f2"]
}

func TestA11yDBusContraUnaAppFalsa(t *testing.T) {
	for _, conColeccion := range []bool{true, false} {
		nombre := "con Collection"
		if !conColeccion {
			nombre = "recorriendo el árbol"
		}
		t.Run(nombre, func(t *testing.T) {
			dir := busPrueba(t)
			registro := conectarPrueba(t, dir)
			app := conectarPrueba(t, dir)
			activa := appFalsa(t, app, conColeccion)
			if r, err := registro.RequestName(busRegistro, dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
				t.Fatalf("RequestName: %v %v", r, err)
			}
			raiz := &nodoFalso{hijos: []refA11y{{app.Names()[0], rutaRaizA11y}}, mu: &sync.Mutex{}}
			if err := registro.Export(raiz, rutaRaizA11y, ifazAccesible); err != nil {
				t.Fatal(err)
			}

			t.Setenv("AT_SPI_BUS_ADDRESS", dir)
			a := &a11yDBus{opciones: []dbus.ConnOption{dbus.WithAuth(dbus.AuthAnonymous())}}
			c, err := a.conexion()
			if err != nil {
				t.Fatal(err)
			}
			// Por TCP el bus no sabe el PID de nadie: se le dice cuál es.
			a.apps = map[int]string{4242: app.Names()[0]}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			marco, rs, err := a.Leer(ctx, 4242, "Pista — Mozilla Firefox")
			if err != nil {
				t.Fatal(err)
			}
			if marco != (rectA11y{x: 0, y: 0, ancho: 800, alto: 600}) {
				t.Errorf("marco = %+v, quiero la ventana activa", marco)
			}
			quiero := map[rectA11y]bool{
				{x: 10, y: 10, ancho: 30, alto: 20, rol: platform.TargetRoleButton}:       true,
				{x: 50, y: 10, ancho: 30, alto: 20, rol: platform.TargetRoleToggleButton}: true,
				{x: 10, y: 100, ancho: 200, alto: 18, rol: platform.TargetRoleLink}:       true,
			}
			if len(rs) != len(quiero) {
				t.Fatalf("objetivos = %+v, quiero los 2 botones y el enlace", rs)
			}
			for _, r := range rs {
				if !quiero[r] {
					t.Errorf("objetivo de más: %+v (deshabilitado u oculto)", r)
				}
			}
			if conColeccion {
				activa.mu.Lock()
				r := activa.regla
				activa.mu.Unlock()
				if r == nil || r.RolesTipo != coincideAlguno || !tieneEstado([]uint32{uint32(r.Estados[0])}, estadoMostrado) {
					t.Errorf("regla recibida = %+v", r)
				}
			}

			// Otro PID no está en el bus.
			a.apps[1] = "no-existe"
			if _, _, err := a.Leer(ctx, 99, ""); err == nil {
				t.Error("un PID que no está debería fallar")
			}
			_ = c
		})
	}
}

// appFalsa800 publica en c la app falsa con 800 enlaces en la ventana activa.
func appFalsa800(t testing.TB, c *dbus.Conn) {
	marco := appFalsa(t, c, true)
	yo := c.Names()[0]
	marco.matches = nil
	for i := range 800 {
		r := dbus.ObjectPath(fmt.Sprintf("/x%d", i))
		n := &nodoFalso{rol: 88, estado: estados(estadoMostrado, estadoSensible),
			ext: extension{int32(i % 40 * 20), int32(i / 40 * 25), 18, 18}, mu: marco.mu}
		for _, ifaz := range []string{ifazAccesible, ifazComponente} {
			if err := c.Export(n, r, ifaz); err != nil {
				t.Fatal(err)
			}
		}
		marco.matches = append(marco.matches, refA11y{yo, r})
	}
}

// TestAyudanteAppFalsa no es una prueba: es la app falsa de
// BenchmarkA11yDBus800 en otro proceso, para que la CPU medida sea solo la
// de Lazul. Dice su nombre en el bus y espera a que se cierre su entrada.
func TestAyudanteAppFalsa(t *testing.T) {
	dir := os.Getenv("LAZUL_APP_FALSA")
	if dir == "" {
		t.Skip("solo la lanza BenchmarkA11yDBus800")
	}
	c := conectarPrueba(t, dir)
	appFalsa800(t, c)
	fmt.Println("NOMBRE", c.Names()[0])
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

// BenchmarkA11yDBus800 lee 800 objetivos de una app falsa (en otro proceso)
// por un bus propio por TCP. ns/op es lo que tarda una lectura; cpu-ms/op,
// la CPU que gasta Lazul en ella (la app de verdad gasta aparte lo suyo en
// contestar).
func BenchmarkA11yDBus800(b *testing.B) {
	dir := busPrueba(b)
	registro := conectarPrueba(b, dir)
	cmd := exec.Command(os.Args[0], "-test.run=^TestAyudanteAppFalsa$")
	cmd.Env = append(os.Environ(), "LAZUL_APP_FALSA="+dir)
	entrada, _ := cmd.StdinPipe()
	salida, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { entrada.Close(); _ = cmd.Wait() })
	var yo string
	lector := bufio.NewScanner(salida)
	for lector.Scan() {
		if n, ok := strings.CutPrefix(lector.Text(), "NOMBRE "); ok {
			yo = n
			break
		}
	}
	if yo == "" {
		b.Fatal("la app falsa no arrancó")
	}
	go func() {
		for lector.Scan() {
		}
	}()
	raiz := &nodoFalso{hijos: []refA11y{{yo, rutaRaizA11y}}, mu: &sync.Mutex{}}
	if err := registro.Export(raiz, rutaRaizA11y, ifazAccesible); err != nil {
		b.Fatal(err)
	}
	if _, err := registro.RequestName(busRegistro, dbus.NameFlagDoNotQueue); err != nil {
		b.Fatal(err)
	}
	b.Setenv("AT_SPI_BUS_ADDRESS", dir)
	a := &a11yDBus{opciones: []dbus.ConnOption{dbus.WithAuth(dbus.AuthAnonymous())}}
	if _, err := a.conexion(); err != nil {
		b.Fatal(err)
	}
	a.apps = map[int]string{1: yo}
	var r0, r1 syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &r0)
	b.ResetTimer()
	for b.Loop() {
		_, rs, err := a.Leer(context.Background(), 1, "Pista — Mozilla Firefox")
		if err != nil || len(rs) != 800 {
			b.Fatalf("%d objetivos, %v", len(rs), err)
		}
	}
	b.StopTimer()
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &r1)
	cpu := r1.Utime.Nano() + r1.Stime.Nano() - r0.Utime.Nano() - r0.Stime.Nano()
	b.ReportMetric(float64(cpu)/1e6/float64(b.N), "cpu-ms/op")
}
