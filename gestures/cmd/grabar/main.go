// grabar: la herramienta para grabar los gestos en fixtures/gestos.
//
// Levanta una página en http://127.0.0.1:8137 que abre la cámara en el
// navegador, corre MediaPipe Hand Landmarker (los mismos archivos que la
// prueba de cámara: prototipos/camara/traer-assets.sh) y graba toma por
// toma. Cada toma se guarda como <gesto o negativo>-<luz>-NNN.jsonl en el
// formato de gestures/grabacion.go, y al guardarla se pasa por el
// clasificador para decir en el acto si se reconoció bien. Mientras tanto
// la página muestra la pose y el modo que ve el clasificador.
//
// Con la cámara se ven las dos manos. Las tomas del modelo nuevo de
// manipulación directa (pellizco y arrastre, entrar/salir del panel, zoom a
// dos manos, movimiento normal y tres dedos a cada lado) van a -dir-pellizco, se marcan con "toma"
// en la cabecera y guardan el estado del pellizco de cada mano en cada
// cuadro (gestures/pellizco.go). El panel calibrado, cuando exista, llena
// "panel" en la cabecera; sin él, el campo queda vacío.
//
//	./dev grabar [-dir fixtures/gestos] [-dir-pellizco fixtures/pellizco] [-puerto 8137] [-vendor prototipos/camara/frontend/vendor]
//
// Solo escucha en 127.0.0.1: los puntos de la mano no salen del equipo.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Rafael088/lazul/desktop"
	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

//go:embed pagina
var pagina embed.FS

func main() {
	dir := flag.String("dir", "fixtures/gestos", "carpeta de las tomas de los gestos viejos")
	dirPellizco := flag.String("dir-pellizco", "fixtures/pellizco", "carpeta de las tomas del modelo nuevo (pellizco)")
	puerto := flag.Int("puerto", 8137, "puerto en 127.0.0.1")
	vendor := flag.String("vendor", "prototipos/camara/frontend/vendor", "carpeta con vision_bundle.mjs, wasm/ y hand_landmarker.task")
	flag.Parse()
	if _, err := os.Stat(filepath.Join(*vendor, "vision_bundle.mjs")); err != nil {
		fmt.Fprintf(os.Stderr, "falta MediaPipe en %s: corre prototipos/camara/traer-assets.sh\n", *vendor)
		os.Exit(1)
	}
	for _, d := range []string{*dir, *dirPellizco} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	s := nuevoServidor(*dir, *dirPellizco, os.DirFS(*vendor))
	s.panel = panelCalibrado()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *puerto))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("grabar: abre http://%s en el navegador (Ctrl+C para salir); las tomas van a %s y %s\n", ln.Addr(), *dir, *dirPellizco)
	if s.panel == nil {
		fmt.Println("grabar: sin panel calibrado; calibra en ./dev app (Calibrar panel) para que las tomas lo lleven")
	} else {
		fmt.Println("grabar: las tomas de pellizco llevan el panel calibrado de ./dev app")
	}
	if err := http.Serve(ln, s.rutas()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type servidor struct {
	dir, dirPellizco string
	vendor           fs.FS

	mu       sync.Mutex
	estado   gestures.Estado
	pellizco *gestures.MedidorPellizco
	panel    *gestures.CalibracionPanel // la calibración de ./dev app; nil sin calibrar
	propias  map[string]string          // nombre → carpeta: solo se descartan las de esta sesión
}

func nuevoServidor(dir, dirPellizco string, vendor fs.FS) *servidor {
	return &servidor{
		dir: dir, dirPellizco: dirPellizco, vendor: vendor,
		pellizco: gestures.NuevoMedidorPellizco(gestures.AjustesPellizcoPorDefecto),
		propias:  map[string]string{},
	}
}

func (s *servidor) rutas() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(pagina, "pagina")
	mux.Handle("GET /", http.FileServerFS(sub))
	mux.Handle("GET /vendor/", http.StripPrefix("/vendor/", http.FileServerFS(s.vendor)))
	mux.HandleFunc("POST /reiniciar", s.reiniciar)
	mux.HandleFunc("POST /cuadro", s.cuadro)
	mux.HandleFunc("POST /guardar", s.guardar)
	mux.HandleFunc("POST /descartar", s.descartar)
	mux.HandleFunc("GET /cuenta", s.cuenta)
	return mux
}

func responder(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// reiniciar pone el clasificador en vivo en el modo con que arranca una toma
// de ese gesto (?gesto=SOLTAR → agarrando) y deja las manos del pellizco
// abiertas.
func (s *servidor) reiniciar(w http.ResponseWriter, r *http.Request) {
	m, err := gestures.Cabecera{Gesto: r.URL.Query().Get("gesto"), Inicio: r.URL.Query().Get("inicio")}.ModoInicial()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.estado = gestures.NuevoEstado(m, gestures.AjustesPorDefecto)
	s.pellizco = gestures.NuevoMedidorPellizco(gestures.AjustesPellizcoPorDefecto)
	s.mu.Unlock()
	responder(w, map[string]string{"modo": m.String()})
}

// cuadroJSON es un cuadro como lo manda la página (el mismo de las grabaciones).
type cuadroJSON struct {
	T     int64 `json:"t"`
	Manos []struct {
		Derecha bool         `json:"derecha"`
		P       [][3]float32 `json:"p"`
	} `json:"manos"`
	Ancho int `json:"ancho"`
	Alto  int `json:"alto"`
}

// cuadro pasa un cuadro por el clasificador en vivo y dice qué ve.
func (s *servidor) cuadro(w http.ResponseWriter, r *http.Request) {
	var cj cuadroJSON
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&cj); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c := gestures.Cuadro{TMs: cj.T}
	if cj.Ancho > 0 && cj.Alto > 0 {
		c.Aspecto = float32(cj.Ancho) / float32(cj.Alto)
	}
	for _, m := range cj.Manos {
		if len(m.P) != gestures.NumPuntos {
			http.Error(w, "una mano tiene que traer 21 puntos", http.StatusBadRequest)
			return
		}
		mano := gestures.Mano{Derecha: m.Derecha}
		for i, p := range m.P {
			mano.Puntos[i] = gestures.Punto{X: p[0], Y: p[1], Z: p[2]}
		}
		c.Manos = append(c.Manos, mano)
	}
	pose := gestures.PoseNinguna
	if len(c.Manos) > 0 {
		pose = gestures.PoseDe(c.Manos[0], c.Aspecto)
	}
	s.mu.Lock()
	var ev gestures.Evento
	s.estado, ev = gestures.Clasificar(s.estado, c)
	estados := s.pellizco.Cuadro(c.Manos, c.Aspecto)
	modo := s.estado.Modo()
	s.mu.Unlock()
	pellizco := make([]map[string]any, len(estados))
	for i, e := range estados {
		pellizco[i] = map[string]any{
			"derecha": c.Manos[i].Derecha, "cerrado": e.Cerrado,
			"apertura": float64(int(e.Apertura*100+0.5)) / 100,
		}
	}
	res := map[string]any{"pose": pose.String(), "modo": modo.String(), "pellizco": pellizco}
	if ev.Gesto != lazulv1.Gesto_GESTO_NINGUNO {
		res["gesto"] = strings.TrimPrefix(ev.Gesto.String(), "GESTO_")
		if ev.Direccion != lazulv1.Direccion_DIRECCION_NINGUNA {
			res["direccion"] = strings.ToLower(strings.TrimPrefix(ev.Direccion.String(), "DIRECCION_"))
		}
	}
	responder(w, res)
}

// base del nombre de archivo: la toma (arrastrar-arriba, ...), el gesto
// (activar, mandar-app) o, en los negativos, su clase (pinza, senalar,
// normal).
func base(c gestures.Cabecera) string {
	b := c.Toma
	if b == "" {
		b = c.Gesto
		if strings.EqualFold(b, "NINGUNO") {
			b = c.Negativo
			if b == "" {
				b = "ninguno"
			}
		}
	}
	b = strings.ToLower(strings.ReplaceAll(b, "_", "-"))
	limpio := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return -1
	}, b)
	if limpio == "" {
		limpio = "toma"
	}
	luz := "buena"
	if c.Luz == "mala" {
		luz = "mala"
	}
	return limpio + "-" + luz
}

// guardar recibe una toma (el .jsonl entero), la valida, la escribe con el
// siguiente número libre y dice si el clasificador la reconoce.
func (s *servidor) guardar(w http.ResponseWriter, r *http.Request) {
	cuerpo, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	g, err := gestures.LeerGrabacion("toma", bytes.NewReader(cuerpo))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(g.Cuadros) == 0 {
		http.Error(w, "la toma no tiene cuadros", http.StatusBadRequest)
		return
	}
	if g.Cabecera.Luz != "buena" && g.Cabecera.Luz != "mala" {
		http.Error(w, `"luz" tiene que ser buena o mala`, http.StatusBadRequest)
		return
	}
	esToma := g.Cabecera.Toma != ""
	if esToma {
		// El estado del pellizco se calcula aquí, sobre los puntos crudos.
		g = gestures.AnotarPellizco(g)
		if g.Cabecera.Panel == nil && s.panel != nil {
			g.Cabecera.Panel = panelDe(*s.panel, g.Cabecera.Ancho, g.Cabecera.Alto)
		}
	}
	destino := s.dir
	if esToma {
		destino = s.dirPellizco
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(destino, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b := base(g.Cabecera)
	var nombre string
	var f *os.File
	for n := 1; ; n++ {
		nombre = fmt.Sprintf("%s-%03d.jsonl", b, n)
		f, err = os.OpenFile(filepath.Join(destino, nombre), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil || !os.IsExist(err) {
			break
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = gestures.EscribirGrabacion(f, g.Cabecera, g.Cuadros)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.propias[nombre] = destino
	g.Archivo = nombre
	res := map[string]any{
		"archivo": nombre, "cuadros": len(g.Cuadros), "ms": g.Duracion(),
	}
	if esToma {
		rp := gestures.ResumirPellizco(g.Cuadros)
		res["toma"] = g.Cabecera.Toma
		res["pellizco"] = map[string]any{
			"cuadros_con_pellizco": rp.CuadrosConPellizco,
			"cierres":              rp.Cierres,
			"fraccion":             rp.Fraccion(),
		}
		responder(w, res)
		return
	}
	rp := gestures.Reproducir(g)
	emitidos := []string{}
	for _, e := range rp.Emitidos {
		emitidos = append(emitidos, strings.TrimPrefix(e.String(), "GESTO_"))
	}
	res["acierto"] = rp.Acierto()
	res["emitidos"] = emitidos
	responder(w, res)
}

// descartar borra una toma de esta sesión (?archivo=activar-buena-003.jsonl).
func (s *servidor) descartar(w http.ResponseWriter, r *http.Request) {
	nombre := r.URL.Query().Get("archivo")
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, propia := s.propias[nombre]
	if !propia {
		http.Error(w, "solo se descartan tomas de esta sesión", http.StatusForbidden)
		return
	}
	if err := os.Remove(filepath.Join(dir, nombre)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	delete(s.propias, nombre)
	responder(w, map[string]string{"descartada": nombre})
}

// cuenta: cuántas tomas hay de cada clase y luz en cada carpeta (la meta es
// 20 de cada una).
func (s *servidor) cuenta(w http.ResponseWriter, r *http.Request) {
	responder(w, map[string]map[string]int{
		"gestos":   contarTomas(s.dir),
		"pellizco": contarTomas(s.dirPellizco),
	})
}

func contarTomas(dir string) map[string]int {
	rutas, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	n := map[string]int{}
	for _, ruta := range rutas {
		nombre := strings.TrimSuffix(filepath.Base(ruta), ".jsonl")
		if i := strings.LastIndex(nombre, "-"); i > 0 {
			n[nombre[:i]]++
		}
	}
	return n
}

// panelCalibrado lee la calibración que guarda ./dev app en la carpeta de
// estado. Sin calibración, nil.
func panelCalibrado() *gestures.CalibracionPanel {
	dir, err := desktop.DirEstado()
	if err != nil {
		return nil
	}
	c, err := desktop.LeerCalibracionPanel(filepath.Join(dir, desktop.NombreCalibracionPanel))
	if err != nil {
		return nil
	}
	return &c
}

// panelDe pasa las esquinas calibradas a coordenadas normalizadas de la
// imagen (0 a 1, sin espejar), que es lo que lleva la cabecera. La
// calibración las guarda centradas en el eje óptico y en unidades de alto de
// la imagen (ver gestures.MuestraPanelDe), así que hace falta el aspecto.
// Sin tamaño de imagen no se puede convertir: nil.
func panelDe(c gestures.CalibracionPanel, ancho, alto int) *gestures.Panel {
	if ancho <= 0 || alto <= 0 {
		return nil
	}
	// La imagen llega sin espejar: la esquina izquierda de la persona puede
	// quedar a la derecha de la imagen. La cabecera guarda el rectángulo en
	// el orden de la imagen, así que van el mínimo y el máximo.
	a := float64(ancho) / float64(alto)
	xa, xb := c.SuperiorIzquierda.X/a+0.5, c.InferiorDerecha.X/a+0.5
	p := gestures.Panel{
		X0: math.Min(xa, xb), Y0: c.SuperiorIzquierda.Y + 0.5,
		X1: math.Max(xa, xb), Y1: c.InferiorDerecha.Y + 0.5,
	}
	for _, v := range []float64{p.X0, p.Y0, p.X1, p.Y1} {
		if v < 0 || v > 1 {
			return nil // calibrada con la mano medio fuera de la imagen
		}
	}
	return &p
}
