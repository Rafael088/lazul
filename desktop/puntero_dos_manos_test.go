package desktop

import (
	"context"
	"math"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// El puntero tiene una mano dueña (cotejo-goem2w). Todo pasa por Filtrar y
// Cuadro, como en la app, y se mira lo que sale hacia el sistema.

type eventoCursor struct {
	p       gestures.PuntoPanel
	boton   bool
	pulsado bool
}

type cursorPrueba struct{ eventos []eventoCursor }

func (*cursorPrueba) Prepare(context.Context) error { return nil }
func (c *cursorPrueba) MoveAbsolute(_ context.Context, x, y float64) error {
	c.eventos = append(c.eventos, eventoCursor{p: gestures.PuntoPanel{X: x, Y: y}})
	return nil
}
func (c *cursorPrueba) Button(_ context.Context, abajo bool) error {
	c.eventos = append(c.eventos, eventoCursor{p: c.donde(), boton: true, pulsado: abajo})
	return nil
}
func (c *cursorPrueba) donde() gestures.PuntoPanel {
	if len(c.eventos) == 0 {
		return gestures.PuntoPanel{X: -1, Y: -1}
	}
	return c.eventos[len(c.eventos)-1].p
}

type dosManos struct {
	t      *testing.T
	p      *Panel
	cursor *cursorPrueba
	tactil *tactilPanelPrueba
	ms, dt int64
}

func nuevasDosManos(t *testing.T, dt int64, conTactil bool) *dosManos {
	t.Helper()
	c, err := gestures.NuevaCalibracionPanel(muestraPanelPrueba(-1, -1, .2), muestraPanelPrueba(1, 1, .2))
	if err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(t.TempDir(), NombreCalibracionPanel)
	if err := GuardarCalibracionPanel(ruta, c); err != nil {
		t.Fatal(err)
	}
	d := &dosManos{t: t, cursor: &cursorPrueba{}, tactil: &tactilPanelPrueba{}, dt: dt}
	if d.p, err = NuevoPanel(ruta, d.cursor); err != nil {
		t.Fatal(err)
	}
	if conTactil {
		d.p.Tactil = d.tactil
	}
	_ = d.p.Preparar() // sin táctil es el modo de respaldo
	d.p.Encender(true)
	return d
}

// cuadro devuelve los dedos que salieron en él.
func (d *dosManos) cuadro(manos ...gestures.Mano) []platform.TouchPoint {
	d.ms += d.dt
	n := len(d.tactil.frames)
	f, _ := d.p.Filtrar(cuadroPanelPrueba(d.ms, manos...))
	d.p.Cuadro(f, true)
	if len(d.tactil.frames) == n {
		return nil
	}
	return d.tactil.frames[len(d.tactil.frames)-1]
}

func (d *dosManos) varios(n int, manos ...gestures.Mano) {
	for range n {
		d.cuadro(manos...)
	}
}

func manoDe(derecha bool, x, y float64, cerrada bool) gestures.Mano {
	m := manoPanelPrueba(x, y, .2, cerrada)
	m.Derecha = derecha
	return m
}

func izq(x, y float64, cerrada bool) gestures.Mano { return manoDe(false, x, y, cerrada) }
func der(x, y float64, cerrada bool) gestures.Mano { return manoDe(true, x, y, cerrada) }

func enPantalla(x, y float64) gestures.PuntoPanel {
	f := func(v float64) float64 {
		return limitar01(((v+1)/2 - gestures.MargenGananciaPanel) / (1 - 2*gestures.MargenGananciaPanel))
	}
	return gestures.PuntoPanel{X: f(x), Y: f(y)}
}

func exigirCerca(t *testing.T, que string, hay, quiere gestures.PuntoPanel, margen float64) {
	t.Helper()
	if d := math.Hypot(hay.X-quiere.X, hay.Y-quiere.Y); d > margen {
		t.Fatalf("%s: %.3f,%.3f y tenía que estar en %.3f,%.3f", que, hay.X, hay.Y, quiere.X, quiere.Y)
	}
}

func puntoDe(p platform.TouchPoint) gestures.PuntoPanel {
	return gestures.PuntoPanel{X: p.X, Y: p.Y}
}

var ritmos = []int64{33, 67, 125}

// 1. Quieta es apuntando.
func TestPunteroLaQuietaNoLoPierde(t *testing.T) {
	for _, dt := range ritmos {
		for _, dura := range []int{1, 2, 3, 30} {
			for _, primera := range []bool{false, true} {
				for _, pellizca := range []bool{false, true} {
					d := nuevasDosManos(t, dt, true)
					quieta := der(.6, -.2, false)
					d.varios(10, quieta)
					for i := range dura {
						otra := izq(-.6+float64(i%5)*.04, .3, pellizca)
						if primera {
							d.cuadro(otra, quieta)
						} else {
							d.cuadro(quieta, otra)
						}
						exigirCerca(t, "llega otra", d.cursor.donde(), enPantalla(.6, -.2), 1e-4)
					}
					d.varios(3, quieta)
					exigirCerca(t, "se fue la otra", d.cursor.donde(), enPantalla(.6, -.2), 1e-4)
				}
			}
		}
	}
}

func TestPunteroFalsoJustoCuandoLaDuenaParpadea(t *testing.T) {
	for _, dt := range ritmos {
		for falta := 1; falta <= 2; falta++ {
			d := nuevasDosManos(t, dt, true)
			d.varios(10, der(.6, -.2, false))
			for range falta {
				d.cuadro(izq(-.6, .3, false))
				exigirCerca(t, "un falso sin la dueña", d.cursor.donde(), enPantalla(.6, -.2), 1e-4)
			}
			d.cuadro(izq(-.6, .3, false), der(.6, -.2, false))
			exigirCerca(t, "volvió la dueña", d.cursor.donde(), enPantalla(.6, -.2), 1e-4)
		}
	}
}

// 2. La que pellizca sola toca donde apuntaba ella.
func TestPunteroLaOtraTocaEnSuAncla(t *testing.T) {
	ancla := enPantalla(.6, .4)
	casi := gestures.PuntoPanel{X: ancla.X + 22.0/anchoPrueba, Y: ancla.Y}
	for _, dt := range ritmos {
		for _, conIman := range []bool{false, true} {
			for _, primera := range []bool{false, true} {
				d := nuevasDosManos(t, dt, true)
				quiere := ancla
				if conIman {
					const w, h = 24, 24
					m := NuevoIman(&buscadorPrueba{ventana: 1, conjunto: conjuntoPrueba(0, obj(casi.X*anchoPrueba-w/2, casi.Y*altoPrueba-h/2, w, h))})
					ahora := time.Unix(2000, 0)
					m.Pedir(true)
					m.Paso(context.Background(), ahora)
					d.p.Iman, d.p.ahora = m, func() time.Time { return ahora }
					var res ResultadoAjuste
					if quiere, res = m.Objetivos(ahora).AjustarToque(ancla); res != AjusteMovido {
						t.Fatalf("el control del imán no mueve el ancla: %s", res)
					}
				}
				cuadro := func(x float64, otra gestures.Mano) []platform.TouchPoint {
					if primera {
						return d.cuadro(otra, izq(x, -.5, false))
					}
					return d.cuadro(izq(x, -.5, false), otra)
				}
				d.cuadro(izq(-.7, -.5, false))
				x := -.7
				// La dueña viene moviéndose: su One Euro va con retraso.
				for range 8 {
					x += .03
					cuadro(x, der(.6, .4, false))
				}
				antes := d.cursor.donde()
				dedos := cuadro(x, der(.64, .43, true)) // cerrar mueve la palma
				if len(dedos) != 1 || !dedos[0].Down {
					t.Fatalf("no apoyó un dedo: %+v", dedos)
				}
				exigirCerca(t, "contacto de la otra", puntoDe(dedos[0]), quiere, 1e-5)
				exigirCerca(t, "puntero de la dueña", d.cursor.donde(), antes, .03)
			}
		}
	}
}

func TestPunteroElToqueSiguienteDeLaDuenaEsSuyo(t *testing.T) {
	d := nuevasDosManos(t, 33, true)
	d.varios(10, izq(-.6, -.4, false))
	d.varios(3, izq(-.6, -.4, false), der(.6, .4, false))
	d.varios(8, izq(-.6, -.4, false), der(.6, .4, true))
	d.varios(12, izq(-.6, -.4, false), der(.6, .4, false))
	dedos := d.cuadro(izq(-.57, -.38, true), der(.6, .4, false))
	if len(dedos) != 1 || !dedos[0].Down {
		t.Fatalf("no apoyó: %+v", dedos)
	}
	exigirCerca(t, "toque de la dueña", puntoDe(dedos[0]), enPantalla(-.6, -.4), 1e-4)
}

// 3. El arrastre sigue con su dedo, agarre la dueña o la otra.
func TestPunteroElArrastreNoCambiaDeDedo(t *testing.T) {
	for _, dt := range ritmos {
		for _, arrastraDuena := range []bool{false, true} {
			d := nuevasDosManos(t, dt, true)
			a, b := -.6, .6 // a arrastra, b estorba
			lado, otro := izq, der
			if arrastraDuena {
				d.varios(5, lado(a, 0, false))
			} else {
				d.varios(5, otro(b, 0, false))
				d.varios(3, otro(b, 0, false), lado(a, 0, false))
			}
			var ranura = -1
			var previo gestures.PuntoPanel
			for i := range 100 {
				y := .4 * math.Sin(float64(i)/30)
				manos := []gestures.Mano{lado(a, y, true)}
				switch {
				case i < 10:
					if !arrastraDuena {
						manos = append(manos, otro(b, 0, false))
					}
				case i < 25: // entra
					manos = append([]gestures.Mano{otro(b, 0, false)}, manos...)
				case i < 40: // pellizca
					manos = append(manos, otro(b, 0, true))
				case i < 60: // suelta
					manos = append(manos, otro(b, 0, false))
				} // y sale
				visto := false
				for _, p := range d.cuadro(manos...) {
					if ranura < 0 && p.Down {
						ranura = p.Slot
						previo = puntoDe(p)
					}
					if p.Slot != ranura {
						continue
					}
					if !p.Down {
						t.Fatalf("se levantó en el cuadro %d", i)
					}
					visto = true
					exigirCerca(t, "paso del dedo", puntoDe(p), previo, .05)
					previo = puntoDe(p)
				}
				if !visto {
					t.Fatalf("sin su dedo en el cuadro %d", i)
				}
			}
		}
	}
}

// 4 y 3. El zoom son los dos dedos de siempre y al soltar una sigue la otra.
func TestPunteroZoomYSoltarUna(t *testing.T) {
	for _, dt := range ritmos {
		for suelta := range 2 {
			d := nuevasDosManos(t, dt, true)
			d.varios(5, izq(-.4, 0, false))
			d.varios(5, izq(-.4, 0, false), der(.4, 0, false))
			dedos := d.cuadro(izq(-.4, 0, true), der(.4, 0, true))
			if len(dedos) != 2 || dedos[0].Slot != 0 || dedos[1].Slot != 1 {
				t.Fatalf("dedos del zoom: %+v", dedos)
			}
			exigirCerca(t, "dedo 0", puntoDe(dedos[0]), enPantalla(-.4, 0), 1e-5)
			exigirCerca(t, "dedo 1", puntoDe(dedos[1]), enPantalla(.4, 0), 1e-5)
			x := .4
			for range 10 {
				x += .03
				dedos = d.cuadro(izq(-x, 0, true), der(x, 0, true))
				if d.cursor.donde().X > .5 {
					t.Fatal("el puntero cambió de mano en el zoom")
				}
			}
			previo := puntoDe(dedos[1-suelta])
			for range 25 {
				dedos = d.cuadro(izq(-x, 0, suelta != 0), der(x, 0, suelta != 1))
				for _, p := range dedos {
					if p.Slot == suelta {
						continue
					}
					if !p.Down {
						t.Fatalf("se levantó el que seguía: %+v", dedos)
					}
					exigirCerca(t, "el que queda", puntoDe(p), previo, .05)
					previo = puntoDe(p)
				}
				if d.cursor.donde().X > .5 {
					t.Fatal("el puntero cambió de mano al soltar una")
				}
			}
		}
	}
}

// 5. La que se va cede al cumplir cuadros y tiempo, de un solo paso.
func TestPunteroRelevoSinArrastrarElSalto(t *testing.T) {
	for _, dt := range ritmos {
		for _, salida := range []string{"desaparece", "sale del panel"} {
			for _, agarradas := range []bool{false, true} {
				d := nuevasDosManos(t, dt, true)
				d.varios(5, izq(-.6, -.3, false))
				d.varios(5, izq(-.6, -.3, false), der(.6, .3, false))
				d.varios(6, izq(-.6, -.3, agarradas), der(.6, .3, agarradas))
				aj := gestures.AjustesEmbraguePorDefecto
				plazo := max(aj.CuadrosParaSoltar, int(math.Ceil(float64(aj.MsParaSoltar)/float64(d.dt))))
				for i := 1; i <= plazo; i++ {
					manos := []gestures.Mano{der(.6, .3, agarradas)}
					if salida == "sale del panel" {
						manos = append([]gestures.Mano{izq(-1.6, -.3, agarradas)}, manos...)
					}
					d.cuadro(manos...)
					quiere := enPantalla(-.6, -.3)
					if i == plazo {
						quiere = enPantalla(.6, .3) // exacto: el filtro empieza ahí
					}
					exigirCerca(t, salida, d.cursor.donde(), quiere, 1e-4)
				}
				if !agarradas {
					dedos := d.cuadro(der(.62, .32, true))
					if len(dedos) != 1 {
						t.Fatalf("no apoyó tras el relevo: %+v", dedos)
					}
					exigirCerca(t, "toque tras el relevo", puntoDe(dedos[0]), enPantalla(.6, .3), 1e-4)
				}
			}
		}
	}
}

// 6. La L lee la primera mano, sea o no la dueña del puntero.
func TestPunteroLaLSigueLeyendoLaPrimeraMano(t *testing.T) {
	d := nuevasDosManos(t, 33, true)
	apunta := der(.6, 0, false)
	d.varios(5, apunta)
	reg, err := AbrirRegistro(filepath.Join(t.TempDir(), "r.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Cerrar()
	v := &Tactil{Reg: reg, Acciones: &accionesTactilesPrueba{}}
	for _, c := range recorridoTactil(t, "modo-activar-buena-001") {
		if len(c.Manos) == 0 {
			continue
		}
		primera := c.Manos[0]
		c.Manos = []gestures.Mano{primera, apunta}
		d.p.Cuadro(c, true)
		if c.Manos[0] != primera {
			t.Fatal("el panel reordenó las manos de la L")
		}
		v.Cuadro(c, time.Unix(0, 0).Add(time.Duration(c.TMs)*time.Millisecond))
	}
	if !v.Activo() {
		t.Fatal("la L de la primera mano no encendió")
	}
}

// Sin pantalla táctil el botón pulsa bajo el cursor.
func TestPunteroRespaldoPulsaBajoLaQuePellizca(t *testing.T) {
	for _, final := range []string{"suelta", "la dueña se va"} {
		d := nuevasDosManos(t, 33, false)
		d.varios(8, izq(-.6, -.3, false))
		d.varios(5, izq(-.6, -.3, false), der(.6, .3, false))
		if final == "suelta" {
			d.varios(10, izq(-.6, -.3, false), der(.6, .3, true))
			d.varios(12, izq(-.6, -.3, false), der(.6, .3, false))
			exigirCerca(t, "vuelve a la dueña", d.cursor.donde(), enPantalla(-.6, -.3), 1e-4)
		} else {
			d.varios(5, izq(-.6, -.3, false), der(.6, .3, true))
			d.varios(20, der(.6, .3, true))
			d.varios(12, der(.6, .3, false))
		}
		var botones []bool
		pulsado := false
		for _, e := range d.cursor.eventos {
			if e.boton {
				botones = append(botones, e.pulsado)
				pulsado = e.pulsado
				exigirCerca(t, final+": el botón", e.p, enPantalla(.6, .3), 1e-4)
			} else if pulsado {
				exigirCerca(t, final+": cursor pulsado", e.p, enPantalla(.6, .3), 1e-4)
			}
		}
		if len(botones) != 2 || !botones[0] || botones[1] {
			t.Fatalf("%s: botones %v", final, botones)
		}
	}
}

// Al azar: pellizcos, parpadeos y faltas de un cuadro en las dos manos. Ni el
// puntero cruza ni un dedo cae a medio camino.
func TestPunteroAlAzarNingunDedoAMedioCamino(t *testing.T) {
	for semilla := range uint64(300) {
		azar := rand.New(rand.NewPCG(semilla, 7))
		d := nuevasDosManos(t, ritmos[semilla%3], true)
		d.varios(3, izq(-.6, 0, false))
		var cerrada, falto [2]bool
		for i := range 150 {
			var manos []gestures.Mano
			for m, x := range []float64{-.6, .6} {
				if azar.IntN(7) == 0 {
					cerrada[m] = !cerrada[m]
				}
				// A lo sumo un cuadro seguido sin verla.
				if falto[m] = !falto[m] && azar.IntN(9) == 0; falto[m] {
					continue
				}
				manos = append(manos, manoDe(m == 1, x+.05*math.Sin(float64(i)/9), .04*math.Cos(float64(i)/7), cerrada[m]))
			}
			if len(manos) == 2 && azar.IntN(2) == 0 {
				manos[0], manos[1] = manos[1], manos[0]
			}
			for _, p := range d.cuadro(manos...) {
				if p.X > .3 && p.X < .7 {
					t.Fatalf("semilla %d, cuadro %d: dedo a medio camino %+v", semilla, i, p)
				}
			}
			if x := d.cursor.donde().X; x > .3 {
				t.Fatalf("semilla %d, cuadro %d: el puntero dejó a la dueña (%.3f)", semilla, i, x)
			}
		}
	}
}

func TestPunteroRespaldoHeredaElArrastreSinTiron(t *testing.T) {
	d := nuevasDosManos(t, 33, false)
	d.varios(8, izq(-.6, -.3, false))
	d.varios(5, izq(-.6, -.3, false), der(.6, .3, false))
	previo, paso := enPantalla(.6, .3), 0.0
	for i := range 28 {
		y := .3 - float64(i)*.025
		manos := []gestures.Mano{der(.6, y, true)}
		if i < 6 { // y la dueña se va a mitad del arrastre
			manos = append(manos, izq(-.6, -.3, false))
		}
		d.cuadro(manos...)
		p := d.cursor.donde()
		if hay := math.Hypot(p.X-previo.X, p.Y-previo.Y); i > 4 && hay > paso*1.5+.002 {
			t.Fatalf("tirón de %.3f tras pasos de %.3f en el cuadro %d", hay, paso, i)
		} else {
			previo, paso = p, hay
		}
	}
}
