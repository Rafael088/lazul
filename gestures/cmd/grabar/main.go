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
//	./dev grabar [-dir fixtures/gestos] [-puerto 8137] [-vendor prototipos/camara/frontend/vendor]
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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

//go:embed pagina
var pagina embed.FS

func main() {
	dir := flag.String("dir", "fixtures/gestos", "carpeta donde se guardan las tomas")
	puerto := flag.Int("puerto", 8137, "puerto en 127.0.0.1")
	vendor := flag.String("vendor", "prototipos/camara/frontend/vendor", "carpeta con vision_bundle.mjs, wasm/ y hand_landmarker.task")
	flag.Parse()
	if _, err := os.Stat(filepath.Join(*vendor, "vision_bundle.mjs")); err != nil {
		fmt.Fprintf(os.Stderr, "falta MediaPipe en %s: corre prototipos/camara/traer-assets.sh\n", *vendor)
		os.Exit(1)
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s := nuevoServidor(*dir, os.DirFS(*vendor))
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *puerto))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("grabar: abre http://%s en el navegador (Ctrl+C para salir); las tomas van a %s\n", ln.Addr(), *dir)
	if err := http.Serve(ln, s.rutas()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type servidor struct {
	dir    string
	vendor fs.FS

	mu      sync.Mutex
	estado  gestures.Estado
	propias map[string]bool // tomas guardadas en esta sesión (las únicas que se pueden descartar)
}

func nuevoServidor(dir string, vendor fs.FS) *servidor {
	return &servidor{dir: dir, vendor: vendor, propias: map[string]bool{}}
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
// de ese gesto (?gesto=SOLTAR → agarrando).
func (s *servidor) reiniciar(w http.ResponseWriter, r *http.Request) {
	m, err := gestures.Cabecera{Gesto: r.URL.Query().Get("gesto"), Inicio: r.URL.Query().Get("inicio")}.ModoInicial()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.estado = gestures.NuevoEstado(m, gestures.AjustesPorDefecto)
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
	modo := s.estado.Modo()
	s.mu.Unlock()
	res := map[string]string{"pose": pose.String(), "modo": modo.String()}
	if ev.Gesto != lazulv1.Gesto_GESTO_NINGUNO {
		res["gesto"] = strings.TrimPrefix(ev.Gesto.String(), "GESTO_")
		if ev.Direccion != lazulv1.Direccion_DIRECCION_NINGUNA {
			res["direccion"] = strings.ToLower(strings.TrimPrefix(ev.Direccion.String(), "DIRECCION_"))
		}
	}
	responder(w, res)
}

// base del nombre de archivo: el gesto (activar, mandar-app) o, en los
// negativos, su clase (pinza, senalar, normal).
func base(c gestures.Cabecera) string {
	b := c.Gesto
	if strings.EqualFold(b, "NINGUNO") {
		b = c.Negativo
		if b == "" {
			b = "ninguno"
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
	s.mu.Lock()
	defer s.mu.Unlock()
	b := base(g.Cabecera)
	var nombre string
	var f *os.File
	for n := 1; ; n++ {
		nombre = fmt.Sprintf("%s-%03d.jsonl", b, n)
		f, err = os.OpenFile(filepath.Join(s.dir, nombre), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
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
	s.propias[nombre] = true
	g.Archivo = nombre
	res := gestures.Reproducir(g)
	emitidos := []string{}
	for _, e := range res.Emitidos {
		emitidos = append(emitidos, strings.TrimPrefix(e.String(), "GESTO_"))
	}
	responder(w, map[string]any{
		"archivo": nombre, "cuadros": len(g.Cuadros), "ms": g.Duracion(),
		"acierto": res.Acierto(), "emitidos": emitidos,
	})
}

// descartar borra una toma de esta sesión (?archivo=activar-buena-003.jsonl).
func (s *servidor) descartar(w http.ResponseWriter, r *http.Request) {
	nombre := r.URL.Query().Get("archivo")
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.propias[nombre] {
		http.Error(w, "solo se descartan tomas de esta sesión", http.StatusForbidden)
		return
	}
	if err := os.Remove(filepath.Join(s.dir, nombre)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	delete(s.propias, nombre)
	responder(w, map[string]string{"descartada": nombre})
}

// cuenta: cuántas tomas hay de cada clase y luz (la meta es 20 de cada una).
func (s *servidor) cuenta(w http.ResponseWriter, r *http.Request) {
	rutas, _ := filepath.Glob(filepath.Join(s.dir, "*.jsonl"))
	n := map[string]int{}
	for _, ruta := range rutas {
		nombre := strings.TrimSuffix(filepath.Base(ruta), ".jsonl")
		if i := strings.LastIndex(nombre, "-"); i > 0 {
			n[nombre[:i]]++
		}
	}
	responder(w, n)
}
