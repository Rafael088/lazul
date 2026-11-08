package gestures

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Grabación (fixtures/gestos/*.jsonl): una línea JSON por renglón.
//
//	{"gesto":"ACTIVAR","luz":"buena","nota":"..."}          ← cabecera
//	{"t":0,"manos":[{"derecha":true,"p":[[x,y,z],...21]}]}  ← un cuadro
//
// "gesto" es el que la grabación contiene (ACTIVAR, AGARRAR, SOLTAR,
// MANDAR_APP) o NINGUNO para negativos y movimiento normal. "luz" es buena
// o mala. Un cuadro sin manos lleva "manos":[].
type Cabecera struct {
	Gesto string `json:"gesto"`
	Luz   string `json:"luz"`
	Nota  string `json:"nota,omitempty"`
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
	Cuadros  []Cuadro
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
			continue
		}
		var cj cuadroJSON
		if err := json.Unmarshal(b, &cj); err != nil {
			return g, fmt.Errorf("%s:%d: %w", nombre, linea, err)
		}
		c := Cuadro{TMs: cj.T}
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

// Reproducir pasa la grabación por Clasificar desde un estado limpio.
func Reproducir(g Grabacion) Resultado {
	r := Resultado{Grabacion: g.Archivo, Esperado: g.Esperado}
	var e Estado
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
}

// Medir reproduce todas las grabaciones.
func Medir(gs []Grabacion) (Medicion, []Resultado) {
	m := Medicion{PorGesto: map[lazulv1.Gesto][2]int{}}
	var rs []Resultado
	for _, g := range gs {
		r := Reproducir(g)
		rs = append(rs, r)
		if r.Esperado == lazulv1.Gesto_GESTO_NINGUNO {
			m.Negativas++
			m.FalsosPositivos += len(r.Emitidos)
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
