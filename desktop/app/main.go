// Lazul (prototipo de factibilidad, cotejo-egtbal): la ventana de escritorio.
// La webview abre la cámara (Windows: getUserMedia; Linux: ffmpeg en un
// proceso aparte, dec-7tgj), corre MediaPipe Hand Landmarker y le pasa los
// puntos a Go: gestures.Clasificar, el Enrutador y las Órdenes al equipo
// emparejado. La lógica está en el paquete desktop (con pruebas); esto es
// solo la ventana y los enlaces con la página. Cómo usarla: desktop/README.md.
//
//	lazul [-nombre N] [-camara auto|webview|proceso|no] [-dispositivo /dev/video0]
//	      [-estado DIR] [-escucha :53317] [-abrir=false] [-mdns=false]
//
// Para medir sin nadie delante (./dev app e2e): -reproducir graba1,graba2 pasa
// grabaciones de fixtures/gestos en vez de la cámara; -mostrar-codigo abre el
// emparejamiento al arrancar; -emparejar-con HOST:PUERTO -codigo N empareja
// solo; -permitir HUELLA da el permiso solo a esa huella; -duracion cierra.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/desktop"
	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform/real"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend
var assets embed.FS

// salida: stdout, o el archivo de -salida (en Windows, con -H windowsgui,
// no hay consola).
var salida io.Writer = os.Stdout

var muSalida sync.Mutex

func decir(f string, a ...any) {
	muSalida.Lock()
	defer muSalida.Unlock()
	fmt.Fprintf(salida, "%s "+f+"\n", append([]any{time.Now().Format("15:04:05.000")}, a...)...)
}

// Config la lee la página al arrancar.
type Config struct {
	Camara     string `json:"camara"` // webview | proceso | no
	Reproducir bool   `json:"reproducir"`
	SO         string `json:"so"`
	Registro   string `json:"registro"`
}

// Solicitud de emparejamiento esperando a la persona.
type Solicitud struct {
	Nombre string `json:"nombre"`
	Huella string `json:"huella"`
	resp   chan bool
}

// Estado es lo que la página pinta cada medio segundo.
type Estado struct {
	Red     desktop.EstadoRed `json:"red"`
	Vista   desktop.Vista     `json:"vista"`
	Permiso *Solicitud        `json:"permiso"`
	Prueba  bool              `json:"prueba"`
	FPS     float64           `json:"fps"`
	ConMano float64           `json:"con_mano"` // fracción de cuadros con mano (últimos 5 s)
	Camara  string            `json:"camara"`   // error de la cámara, si hay
}

type App struct {
	ctx    context.Context
	cfg    Config
	nucleo *desktop.Nucleo
	vivo   *desktop.Vivo
	reg    *desktop.Registro
	cap    *captura

	mu       sync.Mutex
	permiso  *Solicitud
	vista    desktop.Vista
	cuadros  int
	conMano  int
	fps      float64
	fracMano float64
	errCam   string
}

func (a *App) Config() Config { return a.cfg }

// Log deja en la salida lo que la página quiera contar.
func (a *App) Log(s string) { decir("[web] %s", s) }

// CuadroJS es un cuadro como lo manda la página (el formato de las grabaciones).
type CuadroJS struct {
	T     int64 `json:"t"`
	Manos []struct {
		Derecha bool         `json:"derecha"`
		P       [][3]float32 `json:"p"`
	} `json:"manos"`
	Ancho int `json:"ancho"`
	Alto  int `json:"alto"`
}

// Cuadro pasa un cuadro de la cámara por el clasificador.
func (a *App) Cuadro(cj CuadroJS) desktop.Vista {
	c := gestures.Cuadro{TMs: cj.T}
	if cj.Ancho > 0 && cj.Alto > 0 {
		c.Aspecto = float32(cj.Ancho) / float32(cj.Alto)
	}
	for _, m := range cj.Manos {
		if len(m.P) != gestures.NumPuntos {
			continue
		}
		mano := gestures.Mano{Derecha: m.Derecha}
		for i, p := range m.P {
			mano.Puntos[i] = gestures.Punto{X: p[0], Y: p[1], Z: p[2]}
		}
		c.Manos = append(c.Manos, mano)
	}
	return a.cuadro(c)
}

func (a *App) cuadro(c gestures.Cuadro) desktop.Vista {
	v := a.vivo.Cuadro(c, time.Now())
	a.mu.Lock()
	a.vista = v
	a.cuadros++
	if len(c.Manos) > 0 {
		a.conMano++
	}
	a.mu.Unlock()
	if v.Gesto != "" {
		decir("gesto %s %s (pose %s, modo %s, activación %d)", v.Gesto, v.Direccion, v.Pose, v.Modo, v.Activacion)
	}
	return v
}

// CamaraEncender: en Linux arranca ffmpeg (la página pide los cuadros por
// /camara/cuadro); en Windows la página abre la cámara ella misma.
func (a *App) CamaraEncender() string {
	a.mu.Lock()
	a.errCam = ""
	a.mu.Unlock()
	if a.cap != nil {
		if err := a.cap.encender(); err != nil {
			a.mu.Lock()
			a.errCam = err.Error()
			a.mu.Unlock()
			return err.Error()
		}
	}
	decir("cámara encendida (%s)", a.cfg.Camara)
	return ""
}

// CamaraApagar cierra la sesión de uso y, en Linux, ffmpeg.
func (a *App) CamaraApagar() {
	if a.cap != nil {
		a.cap.apagar()
	}
	a.vivo.Parar(time.Now())
	decir("cámara apagada")
}

// CamaraError: la página no pudo abrir la cámara.
func (a *App) CamaraError(s string) {
	a.mu.Lock()
	a.errCam = s
	a.mu.Unlock()
	decir("cámara: %s", s)
}

func (a *App) Estado() Estado {
	a.mu.Lock()
	e := Estado{Vista: a.vista, Permiso: a.permiso, FPS: a.fps, ConMano: a.fracMano, Camara: a.errCam}
	a.mu.Unlock()
	e.Red = a.nucleo.Estado()
	e.Prueba = a.vivo.EnPrueba()
	return e
}

func (a *App) AbrirCodigo() string {
	c, err := a.nucleo.AbrirCodigo()
	if err != nil {
		return "error: " + err.Error()
	}
	decir("codigo %s (vence en %v)", c, core.DuracionCodigo)
	return c
}

func (a *App) CerrarCodigo() { a.nucleo.CerrarCodigo() }

// Emparejar con el receptor en dir; "" si salió bien, si no el motivo.
func (a *App) Emparejar(dir, codigo string) string {
	err := a.nucleo.Emparejar(a.ctx, dir, codigo, a.nucleo.Encontrado(strings.TrimSpace(dir)))
	if err != nil {
		decir("emparejar con %s: %v", dir, err)
		return err.Error()
	}
	decir("emparejado con %s", dir)
	return ""
}

func (a *App) Buscar() string {
	l, err := a.nucleo.Buscar(a.ctx)
	if err != nil && len(l) == 0 {
		return "mDNS: " + err.Error()
	}
	return fmt.Sprintf("%d equipos en la red", len(l))
}

func (a *App) Elegir(huella string) string {
	if err := a.nucleo.Elegir(huella); err != nil {
		return err.Error()
	}
	return ""
}

// ResponderPermiso contesta la solicitud de emparejamiento que está en pantalla.
func (a *App) ResponderPermiso(si bool) {
	a.mu.Lock()
	p := a.permiso
	a.permiso = nil
	a.mu.Unlock()
	if p != nil {
		p.resp <- si
	}
}

func (a *App) Prueba(on bool) { a.vivo.Prueba(on, time.Now()) }

func (a *App) Marcar(falsa bool) string {
	n, err := a.vivo.Marcar(falsa, time.Now())
	if err != nil {
		return err.Error()
	}
	que := "a propósito"
	if falsa {
		que = "sin querer"
	}
	return fmt.Sprintf("La activación %d quedó marcada como %s.", n, que)
}

func (a *App) Resumen() string {
	ls, err := desktop.LeerRegistroDe(a.reg.Ruta())
	if err != nil {
		return err.Error()
	}
	return desktop.Resumir(ls).Texto()
}

// pedirPermiso le pregunta a la persona en la ventana (o, con -permitir, solo
// deja pasar esa huella).
func (a *App) pedirPermiso(permitir string) func(context.Context, core.Solicitud) bool {
	return func(ctx context.Context, s core.Solicitud) bool {
		if permitir != "" {
			ok := strings.EqualFold(s.Huella.String(), permitir)
			decir("permiso: «%s» %s → %v (solo se acepta la huella de -permitir)", s.Nombre, s.Huella.Corta(), ok)
			return ok
		}
		p := &Solicitud{Nombre: s.Nombre, Huella: s.Huella.Corta(), resp: make(chan bool, 1)}
		a.mu.Lock()
		if a.permiso != nil {
			a.mu.Unlock()
			return false // ya hay una en pantalla
		}
		a.permiso = p
		a.mu.Unlock()
		decir("«%s» %s pide emparejarse: esperando a la persona", s.Nombre, s.Huella.Corta())
		if a.ctx != nil {
			wruntime.WindowUnminimise(a.ctx)
			wruntime.WindowShow(a.ctx)
		}
		select {
		case si := <-p.resp:
			decir("permiso para «%s»: %v", s.Nombre, si)
			return si
		case <-ctx.Done():
			a.mu.Lock()
			if a.permiso == p {
				a.permiso = nil
			}
			a.mu.Unlock()
			decir("permiso para «%s»: nadie contestó", s.Nombre)
			return false
		}
	}
}

// medir cuenta los cuadros por segundo cada 5 s.
func (a *App) medir(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		a.mu.Lock()
		n, m := a.cuadros, a.conMano
		a.cuadros, a.conMano = 0, 0
		a.fps = float64(n) / 5
		a.fracMano = 0
		if n > 0 {
			a.fracMano = float64(m) / float64(n)
		}
		a.mu.Unlock()
		if n > 0 {
			decir("cámara: %.1f cuadros/s, mano en %d de %d", float64(n)/5, m, n)
		}
	}
}

// reproducir pasa las grabaciones por el mismo camino que la cámara, a su
// ritmo real.
func (a *App) reproducir(ctx context.Context, cs []gestures.Cuadro) {
	time.Sleep(2 * time.Second)
	ini := time.Now()
	for _, c := range cs {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(ini.Add(time.Duration(c.TMs) * time.Millisecond))):
		}
		a.cuadro(c)
	}
	time.Sleep(time.Second)
	a.vivo.Parar(time.Now())
	decir("reproducción terminada (%d cuadros)", len(cs))
}

func main() {
	nombre := flag.String("nombre", "", "nombre de este equipo (por defecto «PC Linux» o «PC Windows»)")
	estado := flag.String("estado", "", "carpeta de identidad, equipos y registro (por defecto la de configuración y la de estado del usuario)")
	escucha := flag.String("escucha", fmt.Sprintf(":%d", desktop.PuertoLazul), "dirección en que escucha el receptor")
	camara := flag.String("camara", "auto", "auto | webview | proceso | no (auto: proceso en Linux, webview en Windows)")
	dispositivo := flag.String("dispositivo", "/dev/video0", "cámara para el modo proceso (ffmpeg -f v4l2)")
	abrir := flag.Bool("abrir", true, "abrir de verdad lo que llega (false: solo anotarlo)")
	usarMDNS := flag.Bool("mdns", true, "anunciar este equipo y buscar los demás por mDNS")
	adelantar := flag.Bool("adelantar", true, "subir la captura al agarrar, mientras la mano suelta")
	titulo := flag.String("titulo", "Lazul", "título de la ventana")
	reproducir := flag.String("reproducir", "", "grabaciones (separadas por comas) que se pasan en vez de la cámara")
	grabaciones := flag.String("grabaciones", "fixtures/gestos", "carpeta de las grabaciones de -reproducir")
	permitir := flag.String("permitir", "", "huella (hex) a la que se da el permiso sin preguntar (para medir sin nadie)")
	mostrarCodigo := flag.Bool("mostrar-codigo", false, "abrir el emparejamiento al arrancar")
	emparejarCon := flag.String("emparejar-con", "", "host:puerto del receptor con el que emparejarse al arrancar")
	codigo := flag.String("codigo", "", "código del receptor para -emparejar-con")
	duracion := flag.Duration("duracion", 0, "cerrar la app después de este tiempo (0: nunca)")
	archivoSalida := flag.String("salida", "", "archivo para la salida (por defecto, stdout)")
	eco := flag.Bool("eco", false, "copiar cada línea del registro a la salida")
	soloHuella := flag.Bool("solo-huella", false, "mostrar la huella de este equipo y salir")
	resumen := flag.Bool("resumen", false, "mostrar el resumen del registro y salir")
	flag.Parse()

	if *archivoSalida != "" {
		if f, err := os.Create(*archivoSalida); err == nil {
			salida = f
			defer f.Close()
		}
	}
	if err := correr(opciones{*nombre, *estado, *escucha, *camara, *dispositivo, *abrir, *usarMDNS, *adelantar, *titulo, *reproducir,
		*grabaciones, *permitir, *mostrarCodigo, *emparejarCon, *codigo, *duracion, *eco, *soloHuella, *resumen}); err != nil {
		decir("error: %v", err)
		os.Exit(1)
	}
}

type opciones struct {
	nombre, estado, escucha, camara, dispositivo string
	abrir, mdns, adelantar                       bool
	titulo, reproducir, grabaciones, permitir    string
	mostrarCodigo                                bool
	emparejarCon, codigo                         string
	duracion                                     time.Duration
	eco, soloHuella, resumen                     bool
}

func correr(o opciones) error {
	dirConf, dirEstado := o.estado, o.estado
	if o.estado == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		dirConf = filepath.Join(d, "lazul")
		if dirEstado, err = desktop.DirEstado(); err != nil {
			return err
		}
	}
	rutaReg := filepath.Join(dirEstado, desktop.NombreRegistro)
	rutaVentana := filepath.Join(dirEstado, desktop.NombreEstadoVentana)
	if o.resumen {
		ls, err := desktop.LeerRegistroDe(rutaReg)
		if err != nil {
			return err
		}
		fmt.Fprintln(salida, desktop.Resumir(ls).Texto())
		return nil
	}
	reg, err := desktop.AbrirRegistro(rutaReg)
	if err != nil {
		return err
	}
	defer reg.Cerrar()
	if o.eco {
		reg.Espejo = func(b []byte) { muSalida.Lock(); salida.Write(b); muSalida.Unlock() }
	}

	modo := o.camara
	if modo == "auto" {
		modo = "webview"
		if runtime.GOOS == "linux" {
			modo = "proceso"
		}
	}
	var cs []gestures.Cuadro
	if o.reproducir != "" {
		gs, err := desktop.LeerRecorrido(o.grabaciones, o.reproducir)
		if err != nil {
			return err
		}
		cs = desktop.Encadenar(gs, 400)
		modo = "no"
	}
	app := &App{cfg: Config{Camara: modo, Reproducir: len(cs) > 0, SO: runtime.GOOS, Registro: rutaReg}, reg: reg}
	if modo == "proceso" {
		app.cap = nuevaCaptura(o.dispositivo)
	}
	sis := real.New()
	if !o.abrir {
		sis.LaunchApp = anotador{}
	}
	app.nucleo, err = desktop.NuevoNucleo(desktop.Opciones{Dir: dirConf, Nombre: o.nombre, Escucha: o.escucha, Sistema: sis, Reg: reg,
		Permiso: app.pedirPermiso(o.permitir), Adelantar: o.adelantar, MDNS: o.mdns})
	if err != nil {
		return err
	}
	if o.soloHuella {
		fmt.Fprintln(salida, app.nucleo.Huella())
		return nil
	}
	app.vivo = &desktop.Vivo{Reg: reg, AlGesto: app.nucleo.Disparar}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.nucleo.Arrancar(ctx)
	go app.medir(ctx)
	e := app.nucleo.Estado()
	decir("huella %s", app.nucleo.Huella())
	decir("este equipo: «%s» %s · escucha %v · cámara %s · registro %s", e.Nombre, e.Huella, e.Direcciones, modo, rutaReg)
	if e.Error != "" {
		decir("aviso: %s", e.Error)
	}
	if o.mostrarCodigo {
		app.AbrirCodigo()
	}
	if o.emparejarCon != "" {
		go func() {
			if err := app.nucleo.Emparejar(ctx, o.emparejarCon, o.codigo, ""); err != nil {
				decir("emparejar con %s: %v", o.emparejarCon, err)
				return
			}
			decir("emparejado con %s", o.emparejarCon)
		}()
	}

	sub, _ := fs.Sub(assets, "frontend")
	ventana, errVentana := desktop.LeerEstadoVentana(rutaVentana)
	if errVentana != nil {
		decir("aviso: no se restauró la ventana: %v", errVentana)
	}
	guardarVentana := func(c context.Context) {
		ancho, alto := wruntime.WindowGetSize(c)
		x, y := wruntime.WindowGetPosition(c)
		e := desktop.EstadoVentana{Ancho: ancho, Alto: alto, X: x, Y: y, TienePosicion: true}
		if err := desktop.GuardarEstadoVentana(rutaVentana, e); err != nil {
			decir("aviso: no se guardó la ventana: %v", err)
		}
	}
	err = wails.Run(&options.App{
		Title:       o.titulo,
		Width:       ventana.Ancho,
		Height:      ventana.Alto,
		MinWidth:    desktop.AnchoVentanaMinimo,
		MinHeight:   desktop.AltoVentanaMinimo,
		AlwaysOnTop: true,
		AssetServer: &assetserver.Options{Assets: sub, Handler: manejador(app.cap)},
		OnStartup: func(c context.Context) {
			app.ctx = c
			if ventana.TienePosicion {
				wruntime.WindowSetPosition(c, ventana.X, ventana.Y)
			}
			if len(cs) > 0 {
				go func() {
					app.reproducir(ctx, cs)
				}()
			}
			if o.duracion > 0 {
				go func() {
					time.Sleep(o.duracion)
					wruntime.Quit(c)
				}()
			}
		},
		OnBeforeClose: func(c context.Context) bool {
			guardarVentana(c)
			return false
		},
		OnShutdown: func(context.Context) {
			if app.cap != nil {
				app.cap.apagar()
			}
			app.vivo.Parar(time.Now())
			e := app.nucleo.Estado()
			decir("fin · última acción: %s · último recibido: %s", e.Accion, e.Recibido)
			cancel()
		},
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 16, G: 20, B: 40, A: 255},
		// Sin esto Wails deja WebKitGTK sin aceleración y no hay WebGL: MediaPipe no corre.
		Linux: &linux.Options{WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand, ProgramName: "lazul"},
	})
	return err
}
