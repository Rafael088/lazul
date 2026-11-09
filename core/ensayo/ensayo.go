// Package ensayo pasa las grabaciones reales de gestos (fixtures/gestos) por
// el clasificador y, cuando reconoce el gesto, se lo da al Enrutador del
// controlador, que manda la Orden de verdad al otro equipo. Así mide de
// punta a punta sin cámara: qué tanto se reconoce y cuánto tarda la Orden.
//
// Qué mide y qué no: el reloj arranca en el cuadro en que Clasificar
// reconoce el gesto y para cuando llega la Respuesta ok del otro equipo
// (que ya ejecutó la orden). No cuenta la cámara ni MediaPipe (las
// grabaciones ya traen los puntos) ni el tiempo propio del gesto (los 400 ms
// de la L, el arrastre). Las grabaciones se pasan sin esperar entre cuadros.
package ensayo

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Caso es un tipo de prueba de punta a punta.
type Caso struct {
	Nombre      string
	Intentos    int
	Reconocidos int // el clasificador dio el gesto (todos los del intento)
	Ejecutados  int // y el otro equipo respondió ok
	Fallas      []string
	Hechos      []core.Hecho // los ejecutados
}

// Opciones de Correr.
type Opciones struct {
	Max int // intentos por caso; 0 = todas las grabaciones que haya
	// Casos a correr: "captura" (agarrar + soltar) y "gesto4" (mandar app).
	Casos []string
	// Antes, si no es nil, se llama antes de cada intento del gesto 4 (para
	// poner la ventana activa en un sistema falso).
	Antes func(caso string, i int)
	// Ritmo pasa los cuadros a su ritmo real desde que aparece la mano (lo
	// que dura el gesto de verdad), para que lo que el Enrutador adelanta
	// mientras tanto (la subida de la captura) cuente como en la vida real.
	Ritmo bool
}

// pasar reproduce g y llama a f en el cuadro del primer evento con el gesto
// esperado. Devuelve si fue acierto (lo mismo que cuenta ./dev replay).
func pasar(g gestures.Grabacion, ritmo bool, f func(gestures.Evento)) bool {
	e := gestures.NuevoEstado(g.Inicio, gestures.AjustesPorDefecto)
	var emitidos []lazulv1.Gesto
	var t0 int64 = -1
	var ini time.Time
	for _, c := range g.Cuadros {
		if ritmo && len(c.Manos) > 0 {
			if t0 < 0 {
				t0, ini = c.TMs, time.Now()
			}
			time.Sleep(time.Until(ini.Add(time.Duration(c.TMs-t0) * time.Millisecond)))
		}
		var ev gestures.Evento
		e, ev = gestures.Clasificar(e, c)
		if ev.Gesto == lazulv1.Gesto_GESTO_NINGUNO {
			continue
		}
		if ev.Gesto == g.Esperado && !slices.Contains(emitidos, g.Esperado) {
			f(ev)
		}
		emitidos = append(emitidos, ev.Gesto)
	}
	r := gestures.Resultado{Esperado: g.Esperado, Emitidos: emitidos}
	return r.Acierto()
}

func evento(ev gestures.Evento) *lazulv1.EventoGesto {
	return &lazulv1.EventoGesto{Gesto: ev.Gesto, Confianza: ev.Confianza, Direccion: ev.Direccion}
}

func de(gs []gestures.Grabacion, g lazulv1.Gesto) []gestures.Grabacion {
	var r []gestures.Grabacion
	for _, x := range gs {
		if x.Esperado == g {
			r = append(r, x)
		}
	}
	return r
}

func nombre(ruta string) string { return ruta[strings.LastIndexAny(ruta, `/\`)+1:] }

// Correr hace los casos con e y escribe una línea por intento fallido en out.
func Correr(ctx context.Context, e *core.Enrutador, gs []gestures.Grabacion, o Opciones, out io.Writer) []*Caso {
	var casos []*Caso
	for _, nom := range o.Casos {
		c := &Caso{Nombre: nom}
		casos = append(casos, c)
		switch nom {
		case "captura":
			ag, so := de(gs, lazulv1.Gesto_GESTO_AGARRAR), de(gs, lazulv1.Gesto_GESTO_SOLTAR)
			n := min(len(ag), len(so))
			if o.Max > 0 {
				n = min(n, o.Max)
			}
			for i := range n {
				c.Intentos++
				c.intento(ctx, e, out, o.Ritmo, ag[i], so[i])
			}
		case "gesto4":
			ma := de(gs, lazulv1.Gesto_GESTO_MANDAR_APP)
			n := len(ma)
			if o.Max > 0 {
				n = min(n, o.Max)
			}
			for i := range n {
				if o.Antes != nil {
					o.Antes(nom, i)
				}
				c.Intentos++
				c.intento(ctx, e, out, o.Ritmo, ma[i])
			}
		}
	}
	return casos
}

// intento pasa las grabaciones en orden (agarrar y después soltar); cuenta
// solo si todas se reconocen y el último gesto se ejecuta en el otro equipo.
func (c *Caso) intento(ctx context.Context, e *core.Enrutador, out io.Writer, ritmo bool, gs ...gestures.Grabacion) {
	var nombres []string
	for _, g := range gs {
		nombres = append(nombres, nombre(g.Archivo))
	}
	quien := strings.Join(nombres, " + ")
	var hechos []core.Hecho
	for i, g := range gs {
		var err error
		// Ritmo real solo en los gestos que siguen a otro (soltar tras agarrar).
		ok := pasar(g, ritmo && i > 0, func(ev gestures.Evento) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			var h core.Hecho
			h, err = e.Gesto(ctx, evento(ev))
			hechos = append(hechos, h)
		})
		if !ok {
			// Si dio el gesto pero también otro, no cuenta (como en ./dev replay).
			c.falla(out, "%s: el clasificador no reconoció %s", quien, nombre(g.Archivo))
			return
		}
		if err != nil {
			c.Reconocidos += boolInt(i == len(gs)-1)
			c.falla(out, "%s: %v", quien, err)
			return
		}
	}
	// El tiempo es el del último gesto (soltar, mandar app); la pantalla se
	// lee al agarrar, así que ese tramo viene del primero.
	h := hechos[len(hechos)-1]
	if h.Tomar == 0 {
		h.Tomar = hechos[0].Tomar
	}
	c.Reconocidos++
	c.Ejecutados++
	c.Hechos = append(c.Hechos, h)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (c *Caso) falla(out io.Writer, f string, a ...any) {
	s := fmt.Sprintf(f, a...)
	c.Fallas = append(c.Fallas, s)
	fmt.Fprintf(out, "  %s · %s\n", c.Nombre, s)
}

// Percentil q (0 a 1) de los tiempos d (los ordena).
func Percentil(d []time.Duration, q float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	slices.Sort(d)
	return d[int(q*float64(len(d)-1)+0.5)]
}

// Tiempos saca un tramo de los Hechos.
func Tiempos(hs []core.Hecho, f func(core.Hecho) time.Duration) []time.Duration {
	var r []time.Duration
	for _, h := range hs {
		r = append(r, f(h))
	}
	return r
}

func ms(d time.Duration) string { return fmt.Sprintf("%.2f ms", float64(d.Microseconds())/1000) }

// Resumen escribe una línea por caso: reconocidos, ejecutados y la latencia
// (p50, p95, máx) del gesto a la Respuesta, y sus tramos.
func Resumen(out io.Writer, casos []*Caso) {
	for _, c := range casos {
		fmt.Fprintf(out, "%s: %d intentos, %d reconocidos, %d ejecutados en el otro equipo\n", c.Nombre, c.Intentos, c.Reconocidos, c.Ejecutados)
		if len(c.Hechos) == 0 {
			continue
		}
		tramo := func(n string, f func(core.Hecho) time.Duration) {
			t := Tiempos(c.Hechos, f)
			fmt.Fprintf(out, "  %-28s p50 %s · p95 %s · máx %s\n", n, ms(Percentil(t, .5)), ms(Percentil(t, .95)), ms(Percentil(t, 1)))
		}
		tramo("gesto → Respuesta ok:", func(h core.Hecho) time.Duration { return h.Total })
		tramo("  leer aquí (pantalla/app):", func(h core.Hecho) time.Duration { return h.Tomar })
		if slices.ContainsFunc(c.Hechos, func(h core.Hecho) bool { return h.Adelantada }) {
			tramo("  esperar la subida adelantada:", func(h core.Hecho) time.Duration { return h.Subir })
		} else if c.Nombre == "captura" || slices.ContainsFunc(c.Hechos, func(h core.Hecho) bool { return h.Subir > 0 }) {
			tramo("  subir por LocalSend:", func(h core.Hecho) time.Duration { return h.Subir })
		}
		tramo("  Orden Entregar:", func(h core.Hecho) time.Duration { return h.Orden })
		var que []string
		cuenta := map[string]int{}
		for _, h := range c.Hechos {
			if cuenta[h.Que] == 0 {
				que = append(que, h.Que)
			}
			cuenta[h.Que]++
		}
		fmt.Fprintf(out, "  por nivel (app incluye la que cae a su web), gesto → Respuesta ok:\n")
		for _, q := range que {
			var hs []core.Hecho
			for _, h := range c.Hechos {
				if h.Que == q {
					hs = append(hs, h)
				}
			}
			t := Tiempos(hs, func(h core.Hecho) time.Duration { return h.Total })
			fmt.Fprintf(out, "    %-9s %2d · p50 %s · p95 %s · máx %s\n", q, len(hs), ms(Percentil(t, .5)), ms(Percentil(t, .95)), ms(Percentil(t, 1)))
		}
	}
}
