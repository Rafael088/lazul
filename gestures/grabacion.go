package gestures

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Grabación (fixtures/gestos/*.jsonl): una línea JSON por renglón.
//
//	{"gesto":"ACTIVAR","luz":"buena","ancho":640,"alto":480,"nota":"..."}  ← cabecera
//	{"t":0,"manos":[{"derecha":true,"p":[[x,y,z],...21]}]}                ← un cuadro
//
// "gesto" es el que la grabación contiene (ACTIVAR, AGARRAR, SOLTAR,
// MANDAR_APP) o NINGUNO para negativos y movimiento normal. "luz" es buena
// o mala. Un cuadro sin manos lleva "manos":[]. "t" en ms desde el primer
// cuadro; los puntos, tal como los da MediaPipe (normalizados, sin espejar).
//
// Opcionales: "ancho" y "alto" de la imagen (sin ellos se supone 4:3);
// "negativo" dice qué clase de negativo es una grabación NINGUNO (pinza,
// senalar, normal, ...) para contar los falsos positivos por clase;
// "inicio" es el modo en que arranca el clasificador (reposo, activo,
// agarrando). Si falta: AGARRAR y MANDAR_APP arrancan en activo (la L ya
// se hizo), SOLTAR en agarrando (el puño ya se hizo) y el resto en reposo.
// Así cada repetición se graba sola, sin hacer antes los gestos previos.
type Cabecera struct {
	Gesto    string `json:"gesto"`
	Luz      string `json:"luz"`
	Ancho    int    `json:"ancho,omitempty"`
	Alto     int    `json:"alto,omitempty"`
	Negativo string `json:"negativo,omitempty"`
	Inicio   string `json:"inicio,omitempty"`
	Nota     string `json:"nota,omitempty"`
}

// ModoInicial es el modo en que arranca el clasificador para esta cabecera.
func (c Cabecera) ModoInicial() (Modo, error) {
	if c.Inicio != "" {
		m, ok := ModoDeNombre(c.Inicio)
		if !ok {
			return 0, fmt.Errorf("inicio desconocido %q", c.Inicio)
		}
		return m, nil
	}
	switch strings.ToUpper(c.Gesto) {
	case "AGARRAR", "MANDAR_APP":
		return ModoActivo, nil
	case "SOLTAR":
		return ModoAgarrando, nil
	}
	return ModoReposo, nil
}

type manoJSON struct {
	Derecha bool         `json:"derecha"`
	P       [][3]float32 `json:"p"`
}

type cuadroJSON struct {
	T     int64      `json:"t"`
	Manos []manoJSON `json:"manos"`
}

// Grabacion es un archivo leído.
type Grabacion struct {
	Archivo  string
	Cabecera Cabecera
	Esperado lazulv1.Gesto
	Inicio   Modo
	Cuadros  []Cuadro
}

// Duracion en ms, del primer al último cuadro.
func (g Grabacion) Duracion() int64 {
	if len(g.Cuadros) < 2 {
		return 0
	}
	return g.Cuadros[len(g.Cuadros)-1].TMs - g.Cuadros[0].TMs
}

// GestoDeNombre traduce "ACTIVAR" a GESTO_ACTIVAR.
func GestoDeNombre(s string) (lazulv1.Gesto, error) {
	v, ok := lazulv1.Gesto_value["GESTO_"+strings.ToUpper(s)]
	if !ok {
		return 0, fmt.Errorf("gesto desconocido %q", s)
	}
	return lazulv1.Gesto(v), nil
}

// LeerGrabacion lee una grabación de r.
func LeerGrabacion(nombre string, r io.Reader) (Grabacion, error) {
	g := Grabacion{Archivo: nombre}
	var aspecto float32
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	linea := 0
	for sc.Scan() {
		linea++
		b := sc.Bytes()
		if len(strings.TrimSpace(string(b))) == 0 {
			continue
		}
		if linea == 1 {
			if err := json.Unmarshal(b, &g.Cabecera); err != nil {
				return g, fmt.Errorf("%s:1: cabecera: %w", nombre, err)
			}
			var err error
			if g.Esperado, err = GestoDeNombre(g.Cabecera.Gesto); err != nil {
				return g, fmt.Errorf("%s:1: %w", nombre, err)
			}
			if g.Inicio, err = g.Cabecera.ModoInicial(); err != nil {
				return g, fmt.Errorf("%s:1: %w", nombre, err)
			}
			if g.Cabecera.Ancho > 0 && g.Cabecera.Alto > 0 {
				aspecto = float32(g.Cabecera.Ancho) / float32(g.Cabecera.Alto)
			}
			continue
		}
		var cj cuadroJSON
		if err := json.Unmarshal(b, &cj); err != nil {
			return g, fmt.Errorf("%s:%d: %w", nombre, linea, err)
		}
		c := Cuadro{TMs: cj.T, Aspecto: aspecto}
		for _, m := range cj.Manos {
			if len(m.P) != NumPuntos {
				return g, fmt.Errorf("%s:%d: una mano tiene %d puntos, no %d", nombre, linea, len(m.P), NumPuntos)
			}
			var mano Mano
			mano.Derecha = m.Derecha
			for i, p := range m.P {
				mano.Puntos[i] = Punto{p[0], p[1], p[2]}
			}
			c.Manos = append(c.Manos, mano)
		}
		g.Cuadros = append(g.Cuadros, c)
	}
	if linea == 0 {
		return g, fmt.Errorf("%s: vacío", nombre)
	}
	return g, sc.Err()
}

// LeerDirectorio lee todas las grabaciones *.jsonl de dir, en orden.
func LeerDirectorio(dir string) ([]Grabacion, error) {
	rutas, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(rutas)
	var gs []Grabacion
	for _, ruta := range rutas {
		f, err := os.Open(ruta)
		if err != nil {
			return nil, err
		}
		g, err := LeerGrabacion(filepath.Base(ruta), f)
		f.Close()
		if err != nil {
			return nil, err
		}
		gs = append(gs, g)
	}
	return gs, nil
}

// redondear deja 4 decimales (0,1 px en una imagen de 640): las
// grabaciones pesan la mitad y no se pierde nada que MediaPipe dé de verdad.
func redondear(v float32) float32 {
	r := float32(math.Round(float64(v)*1e4) / 1e4)
	if r == 0 {
		return 0 // sin «-0»
	}
	return r
}

// EscribirGrabacion escribe una grabación en el formato de arriba.
func EscribirGrabacion(w io.Writer, cab Cabecera, cuadros []Cuadro) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cab); err != nil {
		return err
	}
	for _, c := range cuadros {
		cj := cuadroJSON{T: c.TMs, Manos: []manoJSON{}}
		for _, m := range c.Manos {
			mj := manoJSON{Derecha: m.Derecha, P: make([][3]float32, NumPuntos)}
			for i, p := range m.Puntos {
				mj.P[i] = [3]float32{redondear(p.X), redondear(p.Y), redondear(p.Z)}
			}
			cj.Manos = append(cj.Manos, mj)
		}
		if err := enc.Encode(cj); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// Resultado de pasar una grabación por el clasificador.
type Resultado struct {
	Grabacion string
	Esperado  lazulv1.Gesto
	Emitidos  []lazulv1.Gesto
}

// Acierto: con un gesto esperado, lo emitió exactamente una vez y nada más.
// Con NINGUNO, no emitió nada.
func (r Resultado) Acierto() bool {
	if r.Esperado == lazulv1.Gesto_GESTO_NINGUNO {
		return len(r.Emitidos) == 0
	}
	return len(r.Emitidos) == 1 && r.Emitidos[0] == r.Esperado
}

// Reproducir pasa la grabación por Clasificar desde un estado limpio, en
// el modo inicial de la grabación y con los ajustes por defecto.
func Reproducir(g Grabacion) Resultado { return ReproducirCon(g, AjustesPorDefecto) }

// ReproducirCon es Reproducir con otros ajustes.
func ReproducirCon(g Grabacion, aj Ajustes) Resultado {
	r := Resultado{Grabacion: g.Archivo, Esperado: g.Esperado}
	e := NuevoEstado(g.Inicio, aj)
	for _, c := range g.Cuadros {
		var ev Evento
		e, ev = Clasificar(e, c)
		if ev.Gesto != lazulv1.Gesto_GESTO_NINGUNO {
			r.Emitidos = append(r.Emitidos, ev.Gesto)
		}
	}
	return r
}

// Medicion resume un conjunto de grabaciones.
type Medicion struct {
	PorGesto        map[lazulv1.Gesto][2]int // aciertos, total (solo gestos 1 a 4)
	Positivas       int                      // grabaciones con un gesto esperado
	AciertosPos     int
	FalsosPositivos int // gestos emitidos en grabaciones NINGUNO
	Negativas       int
	// Por clase de negativo ("negativo" de la cabecera; "otro" si falta):
	// falsos positivos, grabaciones y ms grabados.
	PorNegativo map[string]Negativos
}

// Negativos de una clase.
type Negativos struct {
	Falsos, Grabaciones int
	Ms                  int64
}

// Por30Min son los falsos positivos escalados a 30 minutos de grabación (el
// criterio de dec-q1p7 para el movimiento normal: menos de 1).
func (n Negativos) Por30Min() float64 {
	if n.Ms == 0 {
		return 0
	}
	return float64(n.Falsos) * 30 * 60 * 1000 / float64(n.Ms)
}

// Medir reproduce todas las grabaciones con los ajustes por defecto.
func Medir(gs []Grabacion) (Medicion, []Resultado) { return MedirCon(gs, AjustesPorDefecto) }

// MedirCon es Medir con otros ajustes.
func MedirCon(gs []Grabacion, aj Ajustes) (Medicion, []Resultado) {
	m := Medicion{PorGesto: map[lazulv1.Gesto][2]int{}, PorNegativo: map[string]Negativos{}}
	var rs []Resultado
	for _, g := range gs {
		r := ReproducirCon(g, aj)
		rs = append(rs, r)
		if r.Esperado == lazulv1.Gesto_GESTO_NINGUNO {
			m.Negativas++
			m.FalsosPositivos += len(r.Emitidos)
			clase := g.Cabecera.Negativo
			if clase == "" {
				clase = "otro"
			}
			n := m.PorNegativo[clase]
			n.Falsos += len(r.Emitidos)
			n.Grabaciones++
			n.Ms += g.Duracion()
			m.PorNegativo[clase] = n
			continue
		}
		m.Positivas++
		v := m.PorGesto[r.Esperado]
		v[1]++
		if r.Acierto() {
			v[0]++
			m.AciertosPos++
		}
		m.PorGesto[r.Esperado] = v
	}
	return m, rs
}

// Precision de las grabaciones positivas, de 0 a 1 (meta de la fase: 0,95).
func (m Medicion) Precision() float64 {
	if m.Positivas == 0 {
		return 0
	}
	return float64(m.AciertosPos) / float64(m.Positivas)
}
