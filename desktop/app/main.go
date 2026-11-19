// Lazul (v0.2.0): el indicador de escritorio.
// Por defecto la ventana es una píldora pequeña en una esquina: muestra si
// el modo táctil está activo y destella el gesto reconocido. La vista de la
// cámara queda como opción para acomodarse (botón o Ctrl+Mayús+C) y la app
// arranca con la cámara encendida aunque su vista esté apagada. En Linux abre
// por defecto el modo táctil local (el pellizco toca la pantalla virtual y
// los otros gestos actúan sobre la ventana con foco);
// el modo de transferencia anterior sigue disponible con -modo transferencia.
//
// La webview abre la cámara (Windows: getUserMedia; Linux: ffmpeg en un
// proceso aparte, dec-7tgj), corre MediaPipe Hand y Pose Landmarker y le pasa
// los puntos a Go. La zona torso+panel filtra antes de gestures.Clasificar
// (modo táctil) y el Enrutador (0.1.0). La lógica está en desktop (con
// pruebas); esto es solo la ventana y los enlaces. Cómo usarla:
// desktop/README.md.
//
//	lazul [-nombre N] [-camara auto|webview|proceso|no] [-dispositivo /dev/video0]
//	      [-modo auto|tactil|transferencia]
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
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	"github.com/wailsapp/wails/v2/pkg/options/windows"
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
	Modo       string `json:"modo"` // tactil | transferencia
}

// Solicitud de emparejamiento esperando a la persona.
type Solicitud struct {
	Nombre string `json:"nombre"`
	Huella string `json:"huella"`
	resp   chan bool
}

// Estado es lo que la página pinta cada medio segundo.
type Estado struct {
	Red       desktop.EstadoRed      `json:"red"`
	Vista     desktop.Vista          `json:"vista"`
	Indicador desktop.VistaIndicador `json:"indicador"`
	Panel     desktop.EstadoPanel    `json:"panel"`
	Permiso   *Solicitud             `json:"permiso"`
	Prueba    bool                   `json:"prueba"`
	FPS       float64                `json:"fps"`
	ConMano   float64                `json:"con_mano"`  // fracción de cuadros con mano (últimos 5 s)
	EnZona    float64                `json:"en_zona"`   // fracción con una mano aceptada por torso y panel
	CPUCore   float64                `json:"cpu_core"`  // 100 = un núcleo entero (app y descendientes)
	CPUTotal  float64                `json:"cpu_total"` // porcentaje como el monitor del sistema
	ManoMs    float64                `json:"mano_ms"`
	PoseMs    float64                `json:"pose_ms"`
	Evitadas  map[string]int         `json:"evitadas"`
	Camara    string                 `json:"camara"` // error de la cámara, si hay
}

type App struct {
	ctx    context.Context
	cfg    Config
	nucleo *desktop.Nucleo
	flujo  flujoCamara
	reg    *desktop.Registro
	cap    *captura
	panel  *desktop.Panel
	aura   *Aura

	mu            sync.Mutex
	permiso       *Solicitud
	vista         desktop.Vista
	cuadros       int
	conMano       int
	enZona        int
	fps           float64
	fracMano      float64
	fracZona      float64
	cpuCore       float64
	cpuTotal      float64
	manoMs        float64
	poseMs        float64
	sumaManoMs    float64
	sumaPoseMs    float64
	metricas      int
	evitadas      map[string]int
	rechazoActivo map[string]bool
	errCam        string
	tactil        gestures.EstadoTactil
	ind           *desktop.Indicador

	vistaCamara       bool
	posIndicadorX     int
	posIndicadorY     int
	tienePosIndicador bool
}

type flujoCamara interface {
	Cuadro(gestures.Cuadro, time.Time) desktop.Vista
	Parar(time.Time)
	Prueba(bool, time.Time)
	EnPrueba() bool
	Marcar(bool, time.Time) (int, error)
	Sesion() string
}

func (a *App) Config() Config { return a.cfg }

// Log deja en la salida lo que la página quiera contar.
func (a *App) Log(s string) { decir("[web] %s", s) }

// MediaPipeListo anota con qué delegado arrancó MediaPipe. En CPU va a
// ~7 fps (123 ms por cuadro con 2 manos y Pose) contra ~26 en GPU, así que
// el registro tiene que decirlo. El detalle del fallo solo va a la terminal.
func (a *App) MediaPipeListo(delegado string, fallosGPU int, detalle string) {
	if detalle != "" {
		decir("MediaPipe: la GPU falló %d vez/veces: %s", fallosGPU, detalle)
	}
	l := desktop.Linea{Tipo: desktop.TipoMediaPipe, Resultado: delegado, Cuadros: fallosGPU}
	if fallosGPU > 0 {
		l.Error = "gpu_fallo"
	}
	a.reg.Anotar(l)
}

// CuadroJS es un cuadro como lo manda la página (el formato de las grabaciones).
type CuadroJS struct {
	T     int64 `json:"t"`
	Manos []struct {
		Derecha bool         `json:"derecha"`
		P       [][3]float32 `json:"p"`
	} `json:"manos"`
	Cuerpo *struct {
		P [][4]float32 `json:"p"`
	} `json:"cuerpo"`
	ManoMs float64 `json:"mano_ms"`
	PoseMs float64 `json:"pose_ms"`
	Ancho  int     `json:"ancho"`
	Alto   int     `json:"alto"`
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
	if cj.Cuerpo != nil && len(cj.Cuerpo.P) == gestures.NumPuntosCuerpo {
		cuerpo := &gestures.Cuerpo{}
		for i, p := range cj.Cuerpo.P {
			cuerpo.Puntos[i] = gestures.PuntoCuerpo{X: p[0], Y: p[1], Z: p[2], Visibilidad: p[3]}
		}
		c.Cuerpo = cuerpo
	}
	a.mu.Lock()
	a.sumaManoMs += cj.ManoMs
	a.sumaPoseMs += cj.PoseMs
	a.metricas++
	a.mu.Unlock()
	return a.cuadro(c)
}

func (a *App) cuadro(c gestures.Cuadro) desktop.Vista {
	ahora := time.Now()
	crudoConMano := len(c.Manos) > 0
	filtrado := c
	var paraPanel *gestures.Cuadro
	rechazos := map[string]int{}
	// Las grabaciones históricas no contienen Pose. -reproducir es una vía
	// de prueba determinista, no cámara real; conserva su recorrido original.
	panelCalibrado := false
	if a.panel != nil {
		e := a.panel.Estado()
		panelCalibrado = e.Calibrado && !e.Calibrando
	}
	if !a.cfg.Reproducir && panelCalibrado {
		// Con el panel calibrado, el panel es la zona de intención: Pose
		// solo confirma que hay una persona delante (los dos hombros). Exigir
		// además la palma sobre el torso dejaba fuera la mitad de un panel
		// ancho, sobre todo sentado.
		if !gestures.TorsoFiable(c.Cuerpo) {
			filtrado.Manos = nil
			if len(c.Manos) > 0 {
				rechazos["sin_torso"] = len(c.Manos)
			}
		}
	} else if !a.cfg.Reproducir {
		var fueraTorso int
		filtrado, fueraTorso = gestures.FiltrarPorTorso(c)
		if fueraTorso > 0 {
			motivo := "fuera_torso"
			if !gestures.TorsoFiable(c.Cuerpo) {
				motivo = "sin_torso"
			}
			rechazos[motivo] = fueraTorso
		}
	}
	if a.panel != nil {
		var fueraPanel desktop.RechazoPanel
		cuadroPanel := filtrado
		// Calibrando, la mano va cruda al asistente: las esquinas del panel
		// pueden caer fuera del torso, y sentado frente al PC Pose no suele
		// ver las caderas (sin_torso), así que nunca llegaría una muestra.
		if a.panel.Estado().Calibrando {
			cuadroPanel = c
			delete(rechazos, "sin_torso")
			delete(rechazos, "fuera_torso")
		}
		cuadroPanel, fueraPanel = a.panel.Filtrar(cuadroPanel)
		paraPanel = &cuadroPanel
		// Una línea por motivo (borde, distancia o palma): así el registro
		// dice por qué la mano quedó fuera y no solo que quedó fuera.
		for motivo, n := range fueraPanel.PorMotivo {
			rechazos[claveRechazo("fuera_panel", motivo)] = n
		}
		// El asistente necesita ver la mano que define las esquinas, pero esa
		// mano todavía no está dentro de un panel válido y no llega al flujo
		// de gestos. Solo la consume Panel.Cuadro más abajo.
		if a.panel.Estado().Calibrando {
			filtrado.Manos = nil
		} else {
			filtrado = cuadroPanel
		}
	}
	v := a.flujo.Cuadro(filtrado, ahora)
	if paraPanel != nil {
		a.panel.Cuadro(*paraPanel, !a.flujo.EnPrueba())
	}
	a.registrarEvitadas(rechazos, ahora)
	a.actualizarAura()

	// En modo táctil el indicador lo alimenta desktop.Tactil (AlEvento), que
	// además ejecuta la acción. En modo transferencia el clasificador táctil
	// corre aparte solo para mostrar el gesto: no dispara nada.
	var evT gestures.EventoTactil
	if a.cfg.Modo != "tactil" {
		a.mu.Lock()
		a.tactil, evT = gestures.ClasificarTactil(a.tactil, filtrado)
		a.mu.Unlock()
		a.ind.Aplicar(evT, ahora)
	}
	vi := a.ind.Vista(ahora)
	v.Indicador = &vi
	a.mu.Lock()
	verCamara := a.vistaCamara
	a.mu.Unlock()
	if verCamara && a.panel != nil {
		v.PanelCamara = a.panel.Diagnostico(c)
	}

	a.mu.Lock()
	a.vista = v
	a.cuadros++
	if crudoConMano {
		a.conMano++
	}
	if len(filtrado.Manos) > 0 {
		a.enZona++
	}
	a.mu.Unlock()
	if v.Gesto != "" {
		decir("gesto %s %s (pose %s, modo %s, activación %d)", v.Gesto, v.Direccion, v.Pose, v.Modo, v.Activacion)
	}
	if evT.Accion != gestures.TactilNinguno {
		decir("táctil %s %s (activo %v)", evT.Accion, evT.Direccion, evT.Activo)
	}
	return v
}

// claveRechazo: cómo se anota una mano que quedó fuera de la zona. Resultado
// dice qué zona la dejó fuera y, cuando se sabe, Motivo dice por qué: en el
// panel, fuera_borde, cerca, lejos, sin_palma o sin_panel.
func claveRechazo(resultado, motivo string) string {
	if motivo == "" {
		return resultado
	}
	return resultado + ":" + motivo
}

// registrarEvitadas anota una sola línea al entrar en cada motivo de rechazo.
// Mientras la mano siga fuera, los cuadros siguientes no inflan el contador.
func (a *App) registrarEvitadas(rechazos map[string]int, ahora time.Time) {
	a.mu.Lock()
	if a.rechazoActivo == nil {
		a.rechazoActivo = map[string]bool{}
	}
	if a.evitadas == nil {
		a.evitadas = map[string]int{}
	}
	presentes := map[string]bool{}
	for clave, n := range rechazos {
		if n > 0 {
			presentes[clave] = true
		}
	}
	claves := make([]string, 0, len(presentes)+len(a.rechazoActivo))
	for clave := range presentes {
		claves = append(claves, clave)
	}
	// Los motivos del cuadro anterior que ya no están también se apagan.
	for clave := range a.rechazoActivo {
		if !presentes[clave] {
			claves = append(claves, clave)
		}
	}
	sort.Strings(claves)
	var nuevas []string
	for _, clave := range claves {
		if presentes[clave] && !a.rechazoActivo[clave] {
			a.evitadas[clave]++
			nuevas = append(nuevas, clave)
		}
		a.rechazoActivo[clave] = presentes[clave]
	}
	a.mu.Unlock()
	if len(nuevas) == 0 {
		return
	}
	sesion, prueba := a.flujo.Sesion(), a.flujo.EnPrueba()
	for _, clave := range nuevas {
		resultado, motivo, _ := strings.Cut(clave, ":")
		a.reg.Anotar(desktop.Linea{T: ahora, Tipo: desktop.TipoEvitada, Sesion: sesion,
			Resultado: resultado, Motivo: motivo, Prueba: prueba})
	}
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

// CamaraApagar cierra la sesión de uso y, en Linux, ffmpeg. El indicador
// vuelve a inactivo.
func (a *App) CamaraApagar() {
	if a.cap != nil {
		a.cap.apagar()
	}
	a.flujo.Parar(time.Now())
	if a.panel != nil {
		a.panel.Parar()
	}
	a.mu.Lock()
	a.tactil = gestures.NuevoEstadoTactil(false, gestures.AjustesTactilesPorDefecto)
	a.rechazoActivo = nil
	a.mu.Unlock()
	a.ind.Apagar()
	a.actualizarAura()
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
	vi := a.ind.Vista(time.Now())
	a.mu.Lock()
	e := Estado{Vista: a.vista, Indicador: vi, Permiso: a.permiso, FPS: a.fps, ConMano: a.fracMano,
		EnZona: a.fracZona, CPUCore: a.cpuCore, CPUTotal: a.cpuTotal, ManoMs: a.manoMs, PoseMs: a.poseMs,
		Evitadas: maps.Clone(a.evitadas), Camara: a.errCam}
	a.mu.Unlock()
	e.Red = a.nucleo.Estado()
	e.Prueba = a.flujo.EnPrueba()
	if a.panel != nil {
		e.Panel = a.panel.Estado()
	}
	return e
}

// actualizarAura lleva al halo la posición de la mano y la fase del pellizco
// (cotejo-l9dyjg): fuera, dentro o agarrando. Agarrando, el puntero sigue a
// la mano aunque salga del panel, así que el halo también.
func (a *App) actualizarAura() {
	a.mu.Lock()
	aura := a.aura
	activa := a.cfg.Modo == "tactil" && !a.vistaCamara
	a.mu.Unlock()
	if aura == nil {
		return
	}
	e := EstadoAura{Activa: activa}
	if a.panel != nil {
		p := a.panel.Estado()
		e.Agarrando = p.Fase == string(gestures.EmbragueAgarrando)
		e.Calibrado, e.Dentro, e.X, e.Y = p.Calibrado, p.Dentro || e.Agarrando, p.X, p.Y
	}
	aura.Actualizar(e)
}

// crearAura abre el halo encima de la pantalla. Solo en modo táctil: en
// transferencia la señal visual es la píldora del indicador. Fuera de Linux
// con cgo NuevaAura devuelve nil y no se dibuja nada; la app sigue igual.
func (a *App) crearAura() {
	if a.cfg.Modo != "tactil" {
		return
	}
	// LAZUL_SIN_AURA=1 la apaga: es una superficie a pantalla completa que
	// se repinta en cada cuadro con la mano dentro y comparte la gráfica con
	// MediaPipe (sospecha del lag del 18-nov: 60-140 ms por cuadro con la
	// mano dentro contra 20 ms sin ella).
	if os.Getenv("LAZUL_SIN_AURA") == "1" {
		decir("aura apagada (LAZUL_SIN_AURA=1)")
		if a.reg != nil {
			a.reg.Anotar(desktop.Linea{Tipo: desktop.TipoPanel, Resultado: "sin_aura"})
		}
		return
	}
	html, err := fs.ReadFile(assets, "frontend/aura.html")
	if err != nil {
		decir("aviso: no se cargó el aura: %v", err)
		return
	}
	aura := NuevaAura(string(html))
	a.mu.Lock()
	a.aura = aura
	a.mu.Unlock()
}

// cerrarAura suelta la superficie antes de que termine la app.
func (a *App) cerrarAura() {
	a.mu.Lock()
	aura := a.aura
	a.aura = nil
	a.mu.Unlock()
	if aura != nil {
		aura.Cerrar()
	}
}

// CalibrarPanel empieza (o repite) el asistente de dos esquinas.
func (a *App) CalibrarPanel() string {
	if a.panel == nil {
		return "el puntero absoluto solo está disponible en el modo táctil de Linux"
	}
	a.panel.EmpezarCalibracion()
	return ""
}

// CapturarEsquinaPanel guarda la muestra estable del paso visible.
func (a *App) CapturarEsquinaPanel() string {
	if a.panel == nil {
		return "el panel no está disponible"
	}
	if err := a.panel.CapturarEsquina(); err != nil {
		return err.Error()
	}
	return ""
}

// VistaCamara abre o cierra la vista de la cámara para acomodarse. Al
// abrirla, guarda dónde estaba el indicador y centra la ventana grande; al
// cerrarla, vuelve al tamaño del indicador y a su esquina.
func (a *App) VistaCamara(on bool) string {
	a.mu.Lock()
	if a.vistaCamara == on {
		a.mu.Unlock()
		return ""
	}
	a.vistaCamara = on
	a.mu.Unlock()
	if a.ctx == nil {
		return ""
	}
	if on {
		x, y := wruntime.WindowGetPosition(a.ctx)
		a.mu.Lock()
		a.posIndicadorX, a.posIndicadorY, a.tienePosIndicador = x, y, true
		a.mu.Unlock()
		wruntime.WindowSetSize(a.ctx, desktop.AnchoAjuste, desktop.AltoAjuste)
		if ancho, alto, ok := pantallaActual(a.ctx); ok {
			wruntime.WindowSetPosition(a.ctx, max(0, (ancho-desktop.AnchoAjuste)/2), max(0, (alto-desktop.AltoAjuste)/2))
		}
	} else {
		wruntime.WindowSetSize(a.ctx, desktop.AnchoIndicador, desktop.AltoIndicador)
		a.mu.Lock()
		x, y, ok := a.posIndicadorX, a.posIndicadorY, a.tienePosIndicador
		a.mu.Unlock()
		if ok {
			wruntime.WindowSetPosition(a.ctx, x, y)
		}
	}
	a.actualizarAura()
	decir("vista de cámara %v", on)
	return ""
}

// Salir cierra la app desde la vista de cámara.
func (a *App) Salir() {
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// pantallaActual devuelve el tamaño lógico de la pantalla donde está la
// ventana (o de la principal) para centrar la vista de cámara.
func pantallaActual(ctx context.Context) (int, int, bool) {
	pantallas, err := wruntime.ScreenGetAll(ctx)
	if err != nil {
		return 0, 0, false
	}
	for _, p := range pantallas {
		if p.IsCurrent && p.Size.Width > 0 && p.Size.Height > 0 {
			return p.Size.Width, p.Size.Height, true
		}
	}
	for _, p := range pantallas {
		if p.IsPrimary && p.Size.Width > 0 && p.Size.Height > 0 {
			return p.Size.Width, p.Size.Height, true
		}
	}
	for _, p := range pantallas {
		if p.Size.Width > 0 && p.Size.Height > 0 {
			return p.Size.Width, p.Size.Height, true
		}
	}
	return 0, 0, false
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

func (a *App) Prueba(on bool) { a.flujo.Prueba(on, time.Now()) }

func (a *App) Marcar(falsa bool) string {
	n, err := a.flujo.Marcar(falsa, time.Now())
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
	cpuAnterior, paredAnterior := cpuArbol(), time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		a.mu.Lock()
		n, m, z := a.cuadros, a.conMano, a.enZona
		sumaMano, sumaPose, nm := a.sumaManoMs, a.sumaPoseMs, a.metricas
		a.cuadros, a.conMano, a.enZona = 0, 0, 0
		a.sumaManoMs, a.sumaPoseMs, a.metricas = 0, 0, 0
		a.fps = float64(n) / 5
		a.fracMano, a.fracZona = 0, 0
		if n > 0 {
			a.fracMano = float64(m) / float64(n)
			a.fracZona = float64(z) / float64(n)
		}
		if nm > 0 {
			a.manoMs, a.poseMs = sumaMano/float64(nm), sumaPose/float64(nm)
		}
		cpuAhora, paredAhora := cpuArbol(), time.Now()
		seg := paredAhora.Sub(paredAnterior).Seconds()
		if delta := cpuAhora - cpuAnterior; delta >= 0 && seg > 0 {
			a.cpuCore = 100 * delta / seg
			a.cpuTotal = a.cpuCore / float64(runtime.NumCPU())
		}
		cpuAnterior, paredAnterior = cpuAhora, paredAhora
		core, total, manoMs, poseMs := a.cpuCore, a.cpuTotal, a.manoMs, a.poseMs
		a.mu.Unlock()
		if n > 0 {
			decir("cámara: %.1f cuadros/s, mano en %d de %d, zona en %d; CPU %.1f%% de un núcleo (%.1f%% total), inferencia mano %.1f ms + Pose %.1f ms",
				float64(n)/5, m, n, z, core, total, manoMs, poseMs)
			a.reg.Anotar(desktop.Linea{Tipo: desktop.TipoRendimiento, Sesion: a.flujo.Sesion(), Prueba: a.flujo.EnPrueba(),
				FPS: float64(n) / 5, CPUCore: core, CPUTotal: total, ManoMs: manoMs, PoseMs: poseMs})
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
	a.flujo.Parar(time.Now())
	decir("reproducción terminada (%d cuadros)", len(cs))
}

func main() {
	nombre := flag.String("nombre", "", "nombre de este equipo (por defecto «PC Linux» o «PC Windows»)")
	estado := flag.String("estado", "", "carpeta de identidad, equipos y registro (por defecto la de configuración y la de estado del usuario)")
	escucha := flag.String("escucha", fmt.Sprintf(":%d", desktop.PuertoLazul), "dirección en que escucha el receptor")
	camara := flag.String("camara", "auto", "auto | webview | proceso | no (auto: proceso en Linux, webview en Windows)")
	modoApp := flag.String("modo", "auto", "auto | tactil | transferencia (auto: táctil en Linux)")
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
	if err := correr(opciones{*nombre, *estado, *escucha, *camara, *dispositivo, *modoApp, *abrir, *usarMDNS, *adelantar, *titulo, *reproducir,
		*grabaciones, *permitir, *mostrarCodigo, *emparejarCon, *codigo, *duracion, *eco, *soloHuella, *resumen}); err != nil {
		decir("error: %v", err)
		os.Exit(1)
	}
}

type opciones struct {
	nombre, estado, escucha, camara, dispositivo, modo string
	abrir, mdns, adelantar                             bool
	titulo, reproducir, grabaciones, permitir          string
	mostrarCodigo                                      bool
	emparejarCon, codigo                               string
	duracion                                           time.Duration
	eco, soloHuella, resumen                           bool
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
	rutaPanel := filepath.Join(dirEstado, desktop.NombreCalibracionPanel)
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
	modoControl := o.modo
	if modoControl == "auto" {
		modoControl = "transferencia"
		if runtime.GOOS == "linux" {
			modoControl = "tactil"
		}
	}
	if modoControl != "tactil" && modoControl != "transferencia" {
		return fmt.Errorf("-modo: %q no es auto, tactil ni transferencia", o.modo)
	}
	app := &App{
		cfg:    Config{Camara: modo, Reproducir: len(cs) > 0, SO: runtime.GOOS, Registro: rutaReg, Modo: modoControl},
		reg:    reg,
		ind:    &desktop.Indicador{},
		tactil: gestures.NuevoEstadoTactil(false, gestures.AjustesTactilesPorDefecto),
	}
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
	if modoControl == "tactil" {
		if sis.TactileActions == nil {
			return errors.New("el modo táctil solo está disponible en Linux")
		}
		app.flujo = &desktop.Tactil{Reg: reg, Acciones: sis.TactileActions, AlEvento: app.ind.Aplicar}
		app.panel, err = desktop.NuevoPanel(rutaPanel, sis.AbsolutePointer)
		if err != nil {
			decir("aviso: no se cargó la calibración del panel: %v", err)
		}
		app.panel.Acciones = sis.TactileActions
		app.panel.Tactil = sis.TouchScreen
		app.panel.Reg = reg
		if err := app.panel.Preparar(); err != nil {
			decir("aviso: %v", err)
		}
	} else {
		app.flujo = &desktop.Vivo{Reg: reg, AlGesto: app.nucleo.Disparar}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.nucleo.Arrancar(ctx)
	go app.medir(ctx)
	e := app.nucleo.Estado()
	decir("huella %s", app.nucleo.Huella())
	decir("este equipo: «%s» %s · escucha %v · cámara %s · modo %s · registro %s", e.Nombre, e.Huella, e.Direcciones, modo, modoControl, rutaReg)
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
		app.mu.Lock()
		x, y, ok := app.posIndicadorX, app.posIndicadorY, app.tienePosIndicador
		enAjuste := app.vistaCamara
		app.mu.Unlock()
		if !enAjuste || !ok {
			x, y = wruntime.WindowGetPosition(c)
		}
		e := desktop.EstadoVentana{Ancho: desktop.AnchoIndicador, Alto: desktop.AltoIndicador, X: x, Y: y, TienePosicion: true}
		if err := desktop.GuardarEstadoVentana(rutaVentana, e); err != nil {
			decir("aviso: no se guardó la ventana: %v", err)
		}
	}
	err = wails.Run(&options.App{
		Title:       o.titulo,
		Width:       desktop.AnchoIndicador,
		Height:      desktop.AltoIndicador,
		MinWidth:    desktop.AnchoVentanaMinimo,
		MinHeight:   desktop.AltoVentanaMinimo,
		Frameless:   true,
		AlwaysOnTop: true,
		AssetServer: &assetserver.Options{Assets: sub, Handler: manejador(app.cap)},
		OnStartup: func(c context.Context) {
			app.ctx = c
			// El aura es una superficie layer-shell aparte de la ventana de
			// Wails (Wails v2 no da más de una ventana): transparente, encima
			// de todo, sin foco ni clics. Solo tiene sentido en modo táctil.
			app.crearAura()
			if ventana.TienePosicion {
				wruntime.WindowSetPosition(c, ventana.X, ventana.Y)
			} else if ancho, alto, ok := pantallaActual(c); ok {
				x, y := desktop.PosicionEnEsquina(desktop.AnchoIndicador, desktop.AltoIndicador, ancho, alto, desktop.MargenIndicador)
				wruntime.WindowSetPosition(c, x, y)
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
			app.cerrarAura()
			if app.cap != nil {
				app.cap.apagar()
			}
			app.flujo.Parar(time.Now())
			if app.panel != nil {
				app.panel.Parar()
			}
			e := app.nucleo.Estado()
			decir("fin · última acción: %s · último recibido: %s", e.Accion, e.Recibido)
			cancel()
		},
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 16, G: 20, B: 40, A: 0},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableFramelessWindowDecorations: true,
		},
		// Sin esto Wails deja WebKitGTK sin aceleración y no hay WebGL: MediaPipe no corre.
		Linux: &linux.Options{WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand, ProgramName: "lazul", WindowIsTranslucent: true},
	})
	return err
}
