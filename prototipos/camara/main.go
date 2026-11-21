// Prueba de cámara de Lazul (cotejo-cphace): una ventana Wails que corre
// MediaPipe Hand Landmarker en la webview y mide fps, latencia y CPU.
//
// Uso: camara -modo camara|camara+mano|proceso|proceso+mano|imagen -segundos 30 -delegado GPU|CPU -salida resultado.json
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend
var assets embed.FS

type Config struct {
	Modo           string  `json:"modo"`
	Segundos       int     `json:"segundos"`
	Calentar       int     `json:"calentar"`
	Delegado       string  `json:"delegado"`
	Variante       string  `json:"variante"`
	Manos          int     `json:"manos"`
	PoseCada       int     `json:"pose_cada"`
	PoseAdaptativa bool    `json:"pose_adaptativa"`
	UmbralManoMs   float64 `json:"umbral_mano_ms"`
	SO             string  `json:"so"`
}

var variantes = map[string]bool{
	"base": true, "envio": true, "respuesta": true, "indicador": true,
	"aura": true, "copia": true, "antes": true, "despues": true,
}

// CuadroBanco imita el JSON que la app manda a Go en cada fotograma.
type CuadroBanco struct {
	T     float64 `json:"t"`
	Manos []struct {
		Derecha bool         `json:"derecha"`
		P       [][3]float64 `json:"p"`
	} `json:"manos"`
	Cuerpo *struct {
		P [][4]float64 `json:"p"`
	} `json:"cuerpo,omitempty"`
}

// VistaBanco tiene suficiente estructura para medir el viaje de vuelta y su
// deserialización sin depender del clasificador real.
type VistaBanco struct {
	Pose      string         `json:"pose"`
	Modo      string         `json:"modo"`
	Ultimo    string         `json:"ultimo"`
	Indicador map[string]any `json:"indicador"`
	Panel     map[string]any `json:"panel_camara"`
}

type App struct {
	ctx    context.Context
	cfg    Config
	salida string

	mu        sync.Mutex
	alCerrar  func()
	cpuInicio float64
	tInicio   time.Time
	cuadrosGo int
	ultima    VistaBanco
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// Config la lee la página al arrancar (antes de pedir la cámara).
func (a *App) Config() Config { parcheListo(); return a.cfg }

// Log deja en la terminal lo que la página quiera contar.
func (a *App) Log(s string) { fmt.Fprintln(registro, "[web]", s) }

// registro: stderr, o un archivo con -registro (en Windows, con -H windowsgui, no hay consola).
var registro = os.Stderr

// MedirInicio marca el comienzo de la ventana de medida (después de calentar).
func (a *App) MedirInicio() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cpuInicio = cpuArbol()
	a.tInicio = time.Now()
}

func vistaDe(c CuadroBanco) VistaBanco {
	conMano := len(c.Manos) > 0
	return VistaBanco{
		Pose: "MANO", Modo: "activo", Ultimo: "PELLIZCO",
		Indicador: map[string]any{"activo": conMano, "gesto": "toque", "direccion": ""},
		Panel:     map[string]any{"calibrado": []float64{.15, .12, .86, .9}, "dentro": conMano},
	}
}

// Enviar mide solo JSON+puente hacia Go, sin una respuesta por cuadro.
func (a *App) Enviar(c CuadroBanco) {
	a.mu.Lock()
	a.cuadrosGo++
	a.ultima = vistaDe(c)
	a.mu.Unlock()
}

// Responder añade la vista completa que la app devolvía antes en cada cuadro.
func (a *App) Responder(c CuadroBanco) VistaBanco {
	a.Enviar(c)
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ultima
}

// Estado imita el sondeo del indicador que queda después de la optimización.
func (a *App) Estado() VistaBanco {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ultima
}

// Reportar recibe las métricas de la página, añade la CPU del árbol de
// procesos (app + procesos de la webview), escribe el JSON y cierra.
func (a *App) Reportar(web map[string]any) {
	a.mu.Lock()
	cpu := cpuArbol() - a.cpuInicio
	pared := time.Since(a.tInicio).Seconds()
	a.mu.Unlock()
	res := map[string]any{
		"config":         a.cfg,
		"web":            web,
		"cpu_seg":        cpu,
		"pared_seg":      pared,
		"cpu_pct_nucleo": 100 * cpu / pared,                             // 100 = un núcleo entero
		"cpu_pct_total":  100 * cpu / pared / float64(runtime.NumCPU()), // como el administrador de tareas
		"nucleos":        runtime.NumCPU(),
		"procesos":       procesosArbol(),
		"fecha":          time.Now().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	if a.salida != "" {
		_ = os.WriteFile(a.salida, b, 0o644)
	}
	fmt.Println(string(b))
	if a.alCerrar != nil {
		a.alCerrar()
	}
	wruntime.Quit(a.ctx)
}

func manejador(c *captura) http.Handler {
	if c == nil {
		return nil
	}
	return c
}

func enteroEntorno(nombre string, porDefecto int) int {
	if n, err := strconv.Atoi(os.Getenv(nombre)); err == nil && n > 0 {
		return n
	}
	return porDefecto
}

func decimalEntorno(nombre string, porDefecto float64) float64 {
	if n, err := strconv.ParseFloat(os.Getenv(nombre), 64); err == nil && n >= 0 {
		return n
	}
	return porDefecto
}

func main() {
	app := &App{}
	variante := os.Getenv("LAZUL_VARIANTE")
	if variante == "" {
		variante = "base"
	}
	flag.StringVar(&app.cfg.Modo, "modo", "camara", "camara | camara+mano | proceso | proceso+mano | imagen")
	flag.IntVar(&app.cfg.Segundos, "segundos", 30, "segundos de medida")
	flag.IntVar(&app.cfg.Calentar, "calentar", 5, "segundos de calentamiento sin medir")
	flag.StringVar(&app.cfg.Delegado, "delegado", "GPU", "GPU | CPU")
	flag.StringVar(&app.cfg.Variante, "variante", variante, "base | envio | respuesta | indicador | aura | copia | antes | despues")
	flag.IntVar(&app.cfg.Manos, "manos", enteroEntorno("LAZUL_NUM_HANDS", 1), "máximo de manos de MediaPipe (1 | 2)")
	flag.IntVar(&app.cfg.PoseCada, "pose-cada", enteroEntorno("LAZUL_POSE_CADA", 0), "correr Pose cada N cuadros (0: no correr Pose)")
	flag.BoolVar(&app.cfg.PoseAdaptativa, "pose-adaptativa", os.Getenv("LAZUL_POSE_ADAPTATIVA") == "1", "duplicar pose-cada cuando Hand supera el umbral")
	flag.Float64Var(&app.cfg.UmbralManoMs, "umbral-mano-ms", decimalEntorno("LAZUL_UMBRAL_MANO_MS", 24), "umbral de Hand para espaciar Pose")
	flag.StringVar(&app.salida, "salida", "", "archivo JSON con el resultado")
	dispositivo := flag.String("dispositivo", "/dev/video0", "cámara para los modos proceso (ffmpeg -f v4l2)")
	reg := flag.String("registro", "", "archivo para el registro (por defecto, stderr)")
	flag.Parse()
	if !variantes[app.cfg.Variante] {
		fmt.Fprintf(os.Stderr, "variante desconocida %q\n", app.cfg.Variante)
		os.Exit(2)
	}
	if app.cfg.Manos < 1 || app.cfg.Manos > 2 || app.cfg.PoseCada < 0 {
		fmt.Fprintln(os.Stderr, "-manos debe ser 1 o 2 y -pose-cada no puede ser negativo")
		os.Exit(2)
	}
	if *reg != "" {
		if f, err := os.Create(*reg); err == nil {
			registro = f
		}
	}
	app.cfg.SO = runtime.GOOS
	parcheWebview() // Linux: deja pedir la cámara (ver parche_linux.go)

	// Si la página se cuelga, que la prueba no quede abierta para siempre.
	go func() {
		time.Sleep(time.Duration(app.cfg.Calentar+app.cfg.Segundos+90) * time.Second)
		fmt.Fprintln(os.Stderr, "tiempo agotado: la página no reportó")
		os.Exit(3)
	}()

	sub, _ := fs.Sub(assets, "frontend")
	var cap *captura
	if strings.HasPrefix(app.cfg.Modo, "proceso") {
		cap = nuevaCaptura(*dispositivo)
		app.alCerrar = cap.cerrar
	}
	err := wails.Run(&options.App{
		Title:            "Lazul · prueba de cámara",
		Width:            720,
		Height:           620,
		AssetServer:      &assetserver.Options{Assets: sub, Handler: manejador(cap)},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 16, G: 20, B: 40, A: 255},
		// Sin esto Wails deja WebKitGTK sin aceleración y no hay WebGL: MediaPipe no corre.
		// (Ojo: el 0 de Wails, «OnDemand», llega a WebKit como ALWAYS.)
		Linux: &linux.Options{WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
