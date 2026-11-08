package gestures

// Modelo sintético de la mano: arma los 21 puntos de MediaPipe a partir de
// una forma (cuánto se dobla cada dedo y dónde queda el pulgar), la coloca
// en la imagen (tamaño, inclinación, mano izquierda o derecha) y le pone
// ruido. Con él se generan las grabaciones de gestures/testdata/sinteticas,
// que ./dev replay mide mientras no haya grabaciones reales:
//
//	go test ./gestures -run TestSinteticasAlDia -actualizar

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

var actualizar = flag.Bool("actualizar", false, "reescribe gestures/testdata/sinteticas")

const dirSinteticas = "testdata/sinteticas"

type v3 struct{ u, v, z float64 } // coordenadas de la mano: u hacia el pulgar, v hacia los dedos, en palmas

// forma de la mano en coordenadas propias.
type forma struct {
	flex   [4]float64 // índice, medio, anular, meñique: 0 recto, 1 puño
	phi    [4]float64 // hacia dónde apunta cada dedo, en grados desde v (+ hacia el pulgar)
	pulgar [3]v3      // MCP, IP y punta del pulgar (la base es fija)
}

var (
	nudillos = [4]v3{{0.32, 0.93, 0}, {0, 1, 0}, {-0.24, 0.95, 0}, {-0.45, 0.82, 0}}
	falanges = [4][3]float64{{0.45, 0.27, 0.22}, {0.50, 0.30, 0.24}, {0.46, 0.28, 0.22}, {0.36, 0.21, 0.19}}
	baseP    = v3{0.28, 0.18, 0}
	abanico  = [4]float64{8, 0, -8, -16}
)

func (f forma) puntos() [NumPuntos]v3 {
	var p [NumPuntos]v3
	p[PulgarMCP-1] = baseP
	copy(p[PulgarMCP:PulgarPunta+1], f.pulgar[:])
	for d := 0; d < 4; d++ {
		i := IndiceMCP + 4*d
		p[i] = nudillos[d]
		phi := f.phi[d] * math.Pi / 180
		theta := 0.0
		for j, flexion := range [3]float64{80, 100, 70} {
			theta += f.flex[d] * flexion * math.Pi / 180
			l := falanges[d][j]
			p[i+j+1] = v3{
				p[i+j].u + l*math.Sin(phi)*math.Cos(theta),
				p[i+j].v + l*math.Cos(phi)*math.Cos(theta),
				p[i+j].z - l*math.Sin(theta),
			}
		}
	}
	return p
}

// pulgarHacia arma el pulgar desde la base hasta la punta t, algo curvo.
func pulgarHacia(t v3) [3]v3 {
	du, dv := t.u-baseP.u, t.v-baseP.v
	pu, pv := -dv*0.08, du*0.08 // perpendicular
	return [3]v3{
		{baseP.u + du*0.4 + pu, baseP.v + dv*0.4 + pv, t.z * 0.4},
		{baseP.u + du*0.72 + pu*0.7, baseP.v + dv*0.72 + pv*0.7, t.z * 0.7},
		t,
	}
}

func dedos(flex [4]float64) forma { return forma{flex: flex, phi: abanico} }

func conPulgar(f forma, t v3) forma { f.pulgar = pulgarHacia(t); return f }

// Las formas que se usan.
func formaL() forma {
	f := conPulgar(dedos([4]float64{0, 1, 1, 0.9}), v3{1.0, 0.30, 0})
	f.phi[0] = 3
	return f
}
func formaPuno() forma    { return conPulgar(dedos([4]float64{1, 1, 1, 1}), v3{0.05, 0.72, -0.15}) }
func formaAbierta() forma { return conPulgar(dedos([4]float64{0, 0, 0, 0}), v3{0.95, 0.72, 0}) }
func formaRelajada() forma {
	return conPulgar(dedos([4]float64{0.3, 0.35, 0.4, 0.45}), v3{0.6, 0.55, -0.05})
}
func formaSenalar() forma { return conPulgar(dedos([4]float64{0, 1, 1, 1}), v3{0.05, 0.72, -0.15}) }
func formaSenalarMedio() forma { // pulgar a medio salir, casi paralelo al índice
	return conPulgar(dedos([4]float64{0, 1, 1, 1}), v3{0.74, 0.66, -0.05})
}

// formaSenalarSinPulgar: MediaPipe pierde el pulgar y lo deja pegado al índice.
func formaSenalarSinPulgar() forma {
	f := dedos([4]float64{0, 1, 1, 1})
	f.pulgar = [3]v3{{0.30, 0.55, 0}, {0.31, 0.8, 0}, {0.32, 1.02, 0}}
	return f
}

// formaPinza: punta del pulgar contra la punta del índice; dedoLibres
// dice si los otros tres quedan estirados (como un «ok») o recogidos.
func formaPinza(otrosEstirados bool) forma {
	otros := 0.9
	if otrosEstirados {
		otros = 0.1
	}
	f := dedos([4]float64{0.45, otros, otros, otros})
	tip := f.puntos()[IndicePunta]
	f.pulgar = pulgarHacia(v3{tip.u + 0.04, tip.v - 0.03, tip.z})
	return f
}

// formaC: la pinza a medio cerrar (ni L ni pinza).
func formaC() forma {
	f := dedos([4]float64{0.35, 0.9, 0.9, 0.9})
	tip := f.puntos()[IndicePunta]
	f.pulgar = pulgarHacia(v3{tip.u + 0.25, tip.v - 0.5, tip.z})
	return f
}

// colocacion de la mano en la imagen.
type colocacion struct {
	cx, cy    float64 // muñeca, en altos de imagen (x hasta el aspecto)
	escala    float64 // largo de la palma en altos de imagen
	inclina   float64 // grados, + en sentido horario en la imagen
	izquierda bool    // espeja la mano
	aspecto   float64
}

func (c colocacion) mano(p [NumPuntos]v3, d v3) Mano {
	s, co := math.Sincos(c.inclina * math.Pi / 180)
	m := Mano{Derecha: !c.izquierda}
	for i, q := range p {
		u := q.u
		if c.izquierda {
			u = -u
		}
		// a la imagen: x a la derecha, y hacia abajo; la mano apunta hacia arriba
		x, y := u*c.escala, -q.v*c.escala
		x, y = x*co-y*s, x*s+y*co
		m.Puntos[i] = Punto{
			X: float32((c.cx + d.u + x) / c.aspecto),
			Y: float32(c.cy + d.v + y),
			Z: float32(q.z * c.escala),
		}
	}
	return m
}

// tramo de una grabación: va hacia la forma en trans ms y la sostiene hasta
// ms, moviendo la mano mov (en altos de imagen) a lo largo de todo el tramo.
type tramo struct {
	f       forma
	ms      int64
	trans   int64
	mov     v3
	sinMano bool
}

type ruido struct {
	sigma   float64 // por punto, en unidades normalizadas
	huecos  float64 // fracción de cuadros sin mano
	pierde  float64 // fracción de cuadros en que MediaPipe pierde el pulgar
	vaiven  float64 // oscilación lenta de la mano
	semilla uint64
}

var (
	luzBuena = ruido{sigma: 0.0025, vaiven: 0.006}
	luzMala  = ruido{sigma: 0.006, huecos: 0.05, pierde: 0.03, vaiven: 0.01}
)

const pasoMs = 33 // ~30 fps

func sintetizar(col colocacion, rd ruido, tramos []tramo) []Cuadro {
	rng := rand.New(rand.NewPCG(rd.semilla, 0x1a2b))
	var cuadros []Cuadro
	t := int64(0)
	prev := tramos[0].f.puntos()
	pos := v3{}
	for _, tr := range tramos {
		dest := tr.f.puntos()
		ini, posIni := prev, pos
		for dt := int64(0); dt < tr.ms; dt += pasoMs {
			a := 1.0
			if tr.trans > 0 && dt < tr.trans {
				a = float64(dt) / float64(tr.trans)
			}
			var p [NumPuntos]v3
			for i := range p {
				p[i] = v3{ini[i].u + (dest[i].u-ini[i].u)*a, ini[i].v + (dest[i].v-ini[i].v)*a, ini[i].z + (dest[i].z-ini[i].z)*a}
			}
			b := float64(dt) / float64(tr.ms)
			pos = v3{posIni.u + tr.mov.u*b, posIni.v + tr.mov.v*b, 0}
			c := Cuadro{TMs: t, Aspecto: float32(col.aspecto)}
			t += pasoMs
			if tr.sinMano || rng.Float64() < rd.huecos {
				cuadros = append(cuadros, c)
				continue
			}
			if rng.Float64() < rd.pierde {
				p[PulgarMCP], p[PulgarMCP+1], p[PulgarPunta] = v3{p[5].u, p[5].v - 0.3, 0}, p[6], p[7]
			}
			va := v3{rd.vaiven * math.Sin(float64(t)/700), rd.vaiven * math.Cos(float64(t)/900), 0}
			m := col.mano(p, v3{pos.u + va.u, pos.v + va.v, 0})
			for i := range m.Puntos {
				m.Puntos[i].X += float32(rng.NormFloat64() * rd.sigma)
				m.Puntos[i].Y += float32(rng.NormFloat64() * rd.sigma)
				m.Puntos[i].Z += float32(rng.NormFloat64() * rd.sigma)
			}
			c.Manos = []Mano{m}
			cuadros = append(cuadros, c)
		}
		prev = dest
	}
	return cuadros
}

type sintetica struct {
	nombre string
	cab    Cabecera
	col    colocacion
	rd     ruido
	tramos []tramo
}

// variante i: alterna mano, inclinación, tamaño y sitio.
func colocacionN(i int) colocacion {
	inclina := []float64{0, 12, -15, 6, -8, 18}[i%6]
	escala := []float64{0.20, 0.16, 0.24, 0.18}[i%4]
	return colocacion{cx: 0.55 + 0.1*float64(i%3), cy: 0.85 - 0.05*float64(i%2), escala: escala, inclina: inclina, izquierda: i%2 == 1, aspecto: 4.0 / 3}
}

func luzN(i int, semilla uint64) (string, ruido) {
	if i%2 == 0 {
		r := luzBuena
		r.semilla = semilla
		return "buena", r
	}
	r := luzMala
	r.semilla = semilla
	return "mala", r
}

func sinteticas() []sintetica {
	var out []sintetica
	agregar := func(base string, n int, gesto, negativo, inicio, nota string, tramos func(i int) []tramo) {
		for i := 0; i < n; i++ {
			luz, rd := luzN(i, uint64(len(out)+1))
			out = append(out, sintetica{
				nombre: fmt.Sprintf("%s-%s-%03d", base, luz, i+1),
				cab:    Cabecera{Gesto: gesto, Luz: luz, Ancho: 640, Alto: 480, Negativo: negativo, Inicio: inicio, Nota: "sintética: " + nota},
				col:    colocacionN(i), rd: rd, tramos: tramos(i),
			})
		}
	}
	lado := func(i int) float64 { return []float64{-1, 1}[i%2] }

	agregar("activar", 8, "ACTIVAR", "", "", "la L sostenida casi un segundo", func(i int) []tramo {
		return []tramo{{f: formaRelajada(), ms: 400}, {f: formaL(), ms: 900, trans: 150}, {f: formaRelajada(), ms: 400, trans: 150}}
	})
	agregar("agarrar", 6, "AGARRAR", "", "", "de la mano abierta al puño", func(i int) []tramo {
		return []tramo{{f: formaAbierta(), ms: 400}, {f: formaPuno(), ms: 600, trans: 150}}
	})
	agregar("soltar", 6, "SOLTAR", "", "", "puño y abrir la mano yendo a un lado", func(i int) []tramo {
		return []tramo{{f: formaPuno(), ms: 500}, {f: formaAbierta(), ms: 350, trans: 200, mov: v3{0.28 * lado(i), 0, 0}}, {f: formaAbierta(), ms: 300}}
	})
	agregar("mandar-app", 6, "MANDAR_APP", "", "", "pinza y arrastrar", func(i int) []tramo {
		mov := []v3{{0.25, 0, 0}, {-0.25, 0, 0}, {0, -0.22, 0}}[i%3]
		return []tramo{{f: formaRelajada(), ms: 300}, {f: formaPinza(i%2 == 0), ms: 300, trans: 150}, {f: formaPinza(i%2 == 0), ms: 450, mov: mov}, {f: formaRelajada(), ms: 300, trans: 150}}
	})

	// Negativos de la L (dec-rwrs): pinzas y señalar con el índice.
	agregar("pinza", 6, "NINGUNO", "pinza", "", "pinza sostenida, sin la L", func(i int) []tramo {
		f := formaPinza(i%2 == 0)
		if i >= 4 {
			f = formaC()
		}
		return []tramo{{f: formaRelajada(), ms: 300}, {f: f, ms: 1500, trans: 150}, {f: formaRelajada(), ms: 300, trans: 150}}
	})
	agregar("senalar", 9, "NINGUNO", "senalar", "", "señalar con el índice, pulgar pegado, a medias o perdido", func(i int) []tramo {
		f := []forma{formaSenalar(), formaSenalarMedio(), formaSenalarSinPulgar()}[i%3]
		return []tramo{{f: formaRelajada(), ms: 300}, {f: f, ms: 1500, trans: 150}, {f: formaRelajada(), ms: 300, trans: 150}}
	})
	agregar("l-breve", 4, "NINGUNO", "l-breve", "", "la L de pasada, más corta que el sostén", func(i int) []tramo {
		return []tramo{{f: formaRelajada(), ms: 300}, {f: formaL(), ms: 200, trans: 100}, {f: formaRelajada(), ms: 400, trans: 100}}
	})
	agregar("sin-l", 4, "NINGUNO", "sin-l", "reposo", "puño y pinza arrastrada sin hacer antes la L", func(i int) []tramo {
		return []tramo{{f: formaAbierta(), ms: 300}, {f: formaPuno(), ms: 600, trans: 150}, {f: formaPinza(true), ms: 300, trans: 150}, {f: formaPinza(true), ms: 450, mov: v3{0.25, 0, 0}}}
	})
	agregar("soltar-quieto", 4, "NINGUNO", "soltar-quieto", "agarrando", "abrir el puño sin ir a ningún lado", func(i int) []tramo {
		return []tramo{{f: formaPuno(), ms: 500}, {f: formaAbierta(), ms: 900, trans: 200}}
	})
	agregar("normal", 2, "NINGUNO", "normal", "", "30 s de manos sueltas: relajada, saludo, puño, pinza, señalar, la L de pasada", func(i int) []tramo {
		return tramosNormales(uint64(100+i), 30000)
	})
	return out
}

// tramosNormales: una mezcla al azar de lo que hace una mano sin querer dar
// órdenes.
func tramosNormales(semilla uint64, total int64) []tramo {
	rng := rand.New(rand.NewPCG(semilla, 7))
	formas := []func() forma{formaRelajada, formaAbierta, formaPuno, func() forma { return formaPinza(true) }, formaC, formaSenalar, formaSenalarMedio, formaSenalarSinPulgar}
	var ts []tramo
	for t := int64(0); t < total; {
		var tr tramo
		switch k := rng.IntN(12); {
		case k < len(formas):
			tr = tramo{f: formas[k](), ms: 300 + rng.Int64N(1700), trans: 150 + rng.Int64N(150)}
		case k == 8:
			tr = tramo{f: formaL(), ms: 120 + rng.Int64N(100), trans: 100} // L de pasada
		case k == 9: // saludo: abierta yendo y viniendo
			for j := 0; j < 4; j++ {
				ts = append(ts, tramo{f: formaAbierta(), ms: 250, trans: 100, mov: v3{0.12 * float64(1-2*(j%2)), 0, 0}})
			}
			t += 1000
			continue
		case k == 10:
			tr = tramo{f: formaRelajada(), ms: 500 + rng.Int64N(1500), sinMano: true}
		default:
			tr = tramo{f: formaRelajada(), ms: 800, trans: 200, mov: v3{rng.Float64()*0.2 - 0.1, rng.Float64()*0.1 - 0.05, 0}}
		}
		ts = append(ts, tr)
		t += tr.ms
	}
	return ts
}

func (s sintetica) generar() []Cuadro {
	col := s.col
	if s.cab.Negativo == "normal" {
		col.escala, col.inclina = 0.18, 10
	}
	return sintetizar(col, s.rd, s.tramos)
}

func TestSinteticasAlDia(t *testing.T) {
	ss := sinteticas()
	if *actualizar {
		os.RemoveAll(dirSinteticas)
		if err := os.MkdirAll(dirSinteticas, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range ss {
		var b bytes.Buffer
		if err := EscribirGrabacion(&b, s.cab, s.generar()); err != nil {
			t.Fatal(err)
		}
		ruta := filepath.Join(dirSinteticas, s.nombre+".jsonl")
		if *actualizar {
			if err := os.WriteFile(ruta, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		f, err := os.Open(ruta)
		if err != nil {
			t.Fatalf("%v (go test ./gestures -run TestSinteticasAlDia -actualizar)", err)
		}
		hay, err := LeerGrabacion(s.nombre, f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		quiere, _ := LeerGrabacion(s.nombre, &b)
		if !parecidas(hay, quiere) {
			t.Errorf("%s no coincide con el modelo: go test ./gestures -run TestSinteticasAlDia -actualizar", ruta)
		}
	}
	if !*actualizar {
		rutas, _ := filepath.Glob(filepath.Join(dirSinteticas, "*.jsonl"))
		if len(rutas) != len(ss) {
			t.Errorf("%d archivos en %s y el modelo da %d", len(rutas), dirSinteticas, len(ss))
		}
	}
}

// parecidas: iguales salvo el último decimal (las cuentas de coma flotante
// pueden variar un pelo entre arquitecturas).
func parecidas(a, b Grabacion) bool {
	if a.Cabecera != b.Cabecera || len(a.Cuadros) != len(b.Cuadros) {
		return false
	}
	for i := range a.Cuadros {
		ca, cb := a.Cuadros[i], b.Cuadros[i]
		if ca.TMs != cb.TMs || len(ca.Manos) != len(cb.Manos) {
			return false
		}
		for j := range ca.Manos {
			for k := range ca.Manos[j].Puntos {
				pa, pb := ca.Manos[j].Puntos[k], cb.Manos[j].Puntos[k]
				if math.Abs(float64(pa.X-pb.X)) > 2e-4 || math.Abs(float64(pa.Y-pb.Y)) > 2e-4 || math.Abs(float64(pa.Z-pb.Z)) > 2e-4 {
					return false
				}
			}
		}
	}
	return true
}
