// replay: pasa las grabaciones por el clasificador y dice precisión y falsos
// positivos. Por defecto mide las reales (fixtures/gestos) y las sintéticas
// (gestures/testdata/sinteticas), cada una por su lado.
//
//	./dev replay [-dir D1,D2] [-v] [-sostener-l MS] [-barrer] [-tactil] [-toques] [-pantalla]
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

func main() {
	const dirsPredeterminados = "fixtures/gestos,gestures/testdata/sinteticas"
	dirs := flag.String("dir", dirsPredeterminados, "carpetas con grabaciones *.jsonl, separadas por comas")
	detalle := flag.Bool("v", false, "una línea por grabación fallida (con -toques, una por toque)")
	tactil := flag.Bool("tactil", false, "mide modo táctil: L, scroll, zoom y barrido")
	toques := flag.Bool("toques", false, "mide los toques de fixtures/pellizco: embrague y ToquesDe")
	pantalla := flag.Bool("pantalla", false, "mide los tres dedos (cambiar de pantalla): disparos e intentos por motivo")
	sostenerL := flag.Int64("sostener-l", gestures.AjustesPorDefecto.SostenerL, "ms que se sostiene la L para activar")
	barrer := flag.Bool("barrer", false, "mide la L con varios tiempos de sostén (200 a 800 ms)")
	flag.Parse()
	if n := cuantas(*tactil, *toques, *pantalla); n > 1 {
		fmt.Fprintln(os.Stderr, "error: -tactil, -toques y -pantalla miden cosas distintas; elige una")
		os.Exit(2)
	}
	if *pantalla && *dirs == dirsPredeterminados {
		*dirs = "fixtures/gestos,fixtures/pellizco"
	}
	if *tactil && *dirs == dirsPredeterminados {
		*dirs += ",gestures/testdata/tactiles"
	}
	if *toques && *dirs == dirsPredeterminados {
		*dirs = "fixtures/pellizco"
	}
	aj := gestures.AjustesPorDefecto
	aj.SostenerL = *sostenerL
	fallo := false
	for _, dir := range strings.Split(*dirs, ",") {
		gs, err := gestures.LeerDirectorio(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			fallo = true
			continue
		}
		if len(gs) == 0 {
			fmt.Printf("%s: sin grabaciones (se graban con ./dev grabar, tarea cotejo-x2hyu0)\n", dir)
			continue
		}
		if *pantalla {
			informePantalla(dir, gs, *detalle)
		} else if *toques {
			informeToques(dir, gs, *detalle)
		} else if *tactil {
			informeTactil(dir, gs, *detalle)
		} else {
			informe(dir, gs, aj, *detalle)
		}
		if *barrer && !*tactil && !*toques && !*pantalla {
			barrido(gs, aj)
		}
	}
	if fallo {
		os.Exit(1)
	}
}

func cuantas(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

// informePantalla: disparos de los tres dedos e intentos por motivo. En las
// grabaciones sin el gesto todo disparo es falso.
func informePantalla(dir string, gs []gestures.Grabacion, detalle bool) {
	ms := gestures.MedirTresDedos(gs)
	motivos := map[string]int{}
	disparos, conPose, cuadros := 0, 0, 0
	positivas, aciertos, falsos := 0, 0, 0
	for _, m := range ms {
		if m.Quiere != gestures.PantallaNinguna {
			positivas++
			if m.Acierto() {
				aciertos++
			} else if detalle {
				fmt.Printf("  %s: quiere %s y hubo %d disparos (intentos %v)\n", m.Archivo, m.Quiere, len(m.Disparos), m.Motivos)
			}
		} else {
			falsos += len(m.Disparos)
		}
		for k, v := range m.Motivos {
			motivos[k] += v
		}
		disparos += len(m.Disparos)
		cuadros += m.Cuadros
		if m.Cuadros > 0 {
			conPose++
		}
		if detalle {
			for _, in := range m.Disparos {
				fmt.Printf("  %s: disparo a la %s, %.2f palmas en %d ms (%d cuadros)\n", m.Archivo, in.Lado, in.Palmas, in.Ms, in.Cuadros)
			}
		}
	}
	claves := make([]string, 0, len(motivos))
	for k := range motivos {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	var partes []string
	for _, k := range claves {
		partes = append(partes, fmt.Sprintf("%s %d", k, motivos[k]))
	}
	fmt.Printf("%s: %d grabaciones · tres dedos en %d (%d cuadros) · disparos %d · intentos: %s\n",
		dir, len(gs), conPose, cuadros, disparos, strings.Join(partes, ", "))
	fmt.Printf("  PANTALLA aciertos %d/%d tomas del gesto · FALSOS %d en %d sin el gesto\n", aciertos, positivas, falsos, len(ms)-positivas)
}

// informeToques: una línea por toma con los toques que salen y cómo se
// reparten, más el total. "fuera" son los pellizcos que empiezan con la mano
// fuera del panel: el embrague los descarta, así que no son toque.
func informeToques(dir string, gs []gestures.Grabacion, detalle bool) {
	tomas, err := gestures.MedirToques(gs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	total := gestures.ContarToques(tomas)
	fmt.Printf("%s: %d tomas · %d toques (tap < %d ms · larga > %d ms · quieto ≤ %.2f del panel)\n",
		dir, len(tomas), total.Toques, gestures.ToqueTapMs, gestures.ToqueLargaMs, gestures.ToqueQuieto)
	fmt.Printf("  %-36s %6s %4s %5s %8s %9s %6s\n", "toma", "toques", "tap", "larga", "arrastre", "sin clase", "fuera")
	for _, t := range tomas {
		c := gestures.ContarToques([]gestures.TomaToques{t})
		fmt.Printf("  %-36s %6d %4d %5d %8d %9d %6d\n",
			t.Archivo, c.Toques, c.Tap, c.Larga, c.Arrastre, c.SinClase, c.FueraDelPanel)
		if detalle {
			for _, toque := range t.Toques {
				fmt.Printf("      ranura %d: %s %d ms, recorrido %.3f del panel\n",
					toque.Ranura, toque.Tipo, toque.DuracionMs(), toque.Recorrido)
			}
		}
	}
	fmt.Printf("  %-36s %6d %4d %5d %8d %9d %6d\n",
		"TOTAL", total.Toques, total.Tap, total.Larga, total.Arrastre, total.SinClase, total.FueraDelPanel)
	fmt.Printf("  sin clase = 250-500 ms quieto · fuera = pellizcos que empiezan fuera del panel y no son toque\n")
	fmt.Printf("  la duración va del apoyo al levantar, con el soltar del embrague (%d cuadros/%d ms) dentro\n",
		gestures.AjustesEmbraguePorDefecto.CuadrosParaSoltar, gestures.AjustesEmbraguePorDefecto.MsParaSoltar)
	fmt.Printf("  toques en %d de %d tomas\n", total.TomasConToques, total.Tomas)
	prec := gestures.PrecisionDe(tomas)
	fmt.Println(lineaPrecision(prec))
	fmt.Printf("TEMBLOR con la mano quieta p50 %.5f p90 %.5f (n=%d) · RETRASO al mover rápido p50 %.4f p90 %.4f (n=%d)\n",
		gestures.Percentil(prec.Temblor, .5), gestures.Percentil(prec.Temblor, .9), len(prec.Temblor),
		gestures.Percentil(prec.Retraso, .5), gestures.Percentil(prec.Retraso, .9), len(prec.Retraso))
	fmt.Println(lineaUnaMano(tomas))
	fmt.Println(lineaSaltos(gestures.SaltosPunteroDe(tomas)))
}

// lineaUnaMano repite lo anterior solo con las tomas sin segunda mano: ahí no
// hay a quién pasarle el puntero, así que ninguna regla de dos manos lo mueve.
func lineaUnaMano(tomas []gestures.TomaToques) string {
	var solas []gestures.TomaToques
	for _, t := range tomas {
		if t.UnaMano {
			solas = append(solas, t)
		}
	}
	p := gestures.PrecisionDe(solas)
	return fmt.Sprintf("UNA MANO (%d tomas) aciertos %d/%d · TEMBLOR p50 %.5f p90 %.5f (n=%d) · RETRASO p50 %.4f p90 %.4f (n=%d)",
		len(solas), p.Aciertos, p.Intentos,
		gestures.Percentil(p.Temblor, .5), gestures.Percentil(p.Temblor, .9), len(p.Temblor),
		gestures.Percentil(p.Retraso, .5), gestures.Percentil(p.Retraso, .9), len(p.Retraso))
}

// lineaSaltos es la métrica de cotejo-goem2w: el puntero no cambia de mano
// mientras la suya siga a la vista.
func lineaSaltos(s gestures.SaltosPuntero) string {
	return fmt.Sprintf("SALTOS del puntero con dos manos (> %.1f de pantalla, %d cuadros): por cambio de mano %d · relevo al irse la suya %d · misma clave %d · ya filtrado %d",
		gestures.SaltoPunteroMin, s.Cuadros, s.CambioDeMano, s.Relevo, s.MismaClave, s.Filtrado)
}

// lineaPrecision es la métrica de cotejo-uy84og en una línea: aciertos en
// las tomas que piden un gesto, toques falsos en las que no, toques de más,
// alcance de bordes y el salto del puntero al apoyar (mediana y p90, en
// unidades del panel).
func lineaPrecision(p gestures.PrecisionToques) string {
	pct := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return 100 * float64(a) / float64(b)
	}
	return fmt.Sprintf("PRECISIÓN aciertos %d/%d (%.0f %%) · falsos %d en %d tomas sin pellizco · de más %d · bordes %d/%d · salto al apoyar p50 %.4f p90 %.4f (n=%d)",
		p.Aciertos, p.Intentos, pct(p.Aciertos, p.Intentos), p.Falsos, p.SinPellizco, p.DeMas,
		p.Bordes, p.BordesPosibles, p.SaltoPercentil(0.5), p.SaltoPercentil(0.9), len(p.Saltos))
}

func informeTactil(dir string, gs []gestures.Grabacion, detalle bool) {
	m, rs := gestures.MedirTactil(gs)
	fmt.Printf("%s: %d grabaciones táctiles (%d con gesto, %d sin gesto)\n", dir, len(gs), m.Positivas, m.Negativas)
	var partes []string
	for a := gestures.TactilModo; a <= gestures.TactilBarrido; a++ {
		if v, ok := m.PorAccion[a]; ok {
			partes = append(partes, fmt.Sprintf("%s %d/%d", strings.ToUpper(a.String()), v[0], v[1]))
		}
	}
	if len(partes) > 0 {
		fmt.Printf("  aciertos: %s\n", strings.Join(partes, " · "))
		fmt.Printf("  precisión: %.1f %%\n", 100*m.Precision())
	}
	fmt.Printf("  falsos positivos: %d en %d grabaciones sin gesto\n", m.FalsosPositivos, m.Negativas)

	// Las tomas NINGUNO son negativas reales del modo. Se fuerzan activas
	// para que la puerta de la L no esconda un scroll, zoom o barrido falso.
	falsosActivos, negativasActivas := 0, 0
	var duracionActiva int64
	var fallasActivas []string
	for _, g := range gs {
		if !strings.EqualFold(g.Cabecera.Gesto, "NINGUNO") || g.Cabecera.Tactil != "" {
			continue
		}
		negativasActivas++
		duracionActiva += g.Duracion()
		evs := gestures.ReproducirTactilDesde(g, true).Emitidos
		falsosActivos += len(evs)
		if len(evs) > 0 {
			fallasActivas = append(fallasActivas, fmt.Sprintf("%s: %v", g.Archivo, evs))
		}
	}
	if negativasActivas > 0 {
		fmt.Printf("  grabaciones NINGUNO con modo forzado activo: %d falsos en %d grabaciones (%.1f min)\n",
			falsosActivos, negativasActivas, float64(duracionActiva)/60000)
		if detalle {
			for _, falla := range fallasActivas {
				fmt.Printf("    falso activo %s\n", falla)
			}
		}
	}
	if detalle {
		for _, r := range rs {
			if !r.Acierto() {
				fmt.Printf("  falla %s: esperaba %s/%s, salió %v\n", r.Grabacion, r.Esperado, r.Direccion, r.Emitidos)
			}
		}
	}
}

func nombre(g lazulv1.Gesto) string { return strings.TrimPrefix(g.String(), "GESTO_") }

func informe(dir string, gs []gestures.Grabacion, aj gestures.Ajustes, detalle bool) {
	m, rs := gestures.MedirCon(gs, aj)
	fmt.Printf("%s: %d grabaciones (%d con gesto, %d sin gesto)\n", dir, len(gs), m.Positivas, m.Negativas)
	var partes []string
	for g := lazulv1.Gesto_GESTO_ACTIVAR; g <= lazulv1.Gesto_GESTO_MANDAR_APP; g++ {
		if v, ok := m.PorGesto[g]; ok {
			partes = append(partes, fmt.Sprintf("%s %d/%d", nombre(g), v[0], v[1]))
		}
	}
	if len(partes) > 0 {
		fmt.Printf("  aciertos: %s\n", strings.Join(partes, " · "))
		fmt.Printf("  precisión: %.1f %% (meta ≥ 95 %%)\n", 100*m.Precision())
	}
	fmt.Printf("  falsos positivos: %d en %d grabaciones sin gesto\n", m.FalsosPositivos, m.Negativas)
	clases := make([]string, 0, len(m.PorNegativo))
	for c := range m.PorNegativo {
		clases = append(clases, c)
	}
	sort.Strings(clases)
	partes = partes[:0]
	for _, c := range clases {
		n := m.PorNegativo[c]
		partes = append(partes, fmt.Sprintf("%s %d/%d", c, n.Falsos, n.Grabaciones))
	}
	if len(partes) > 0 {
		fmt.Printf("    por clase (falsos/grabaciones): %s\n", strings.Join(partes, " · "))
	}
	if n, ok := m.PorNegativo["normal"]; ok {
		fmt.Printf("    movimiento normal: %d en %.1f min → %.1f por 30 min (meta < 1)\n", n.Falsos, float64(n.Ms)/60000, n.Por30Min())
	}
	if detalle {
		for _, r := range rs {
			if !r.Acierto() {
				fmt.Printf("  falla %s: esperaba %s, salió %v\n", r.Grabacion, nombre(r.Esperado), r.Emitidos)
			}
		}
	}
}

// barrido: con cada tiempo de sostén de la L, cuántas L se reconocen y
// cuántas activaciones falsas hay. Es lo que fija el sostén (dec-rwrs).
func barrido(gs []gestures.Grabacion, aj gestures.Ajustes) {
	fmt.Println("  sostén de la L → L reconocidas · activaciones falsas")
	for ms := int64(200); ms <= 800; ms += 100 {
		aj.SostenerL = ms
		m, rs := gestures.MedirCon(gs, aj)
		falsas := 0
		for _, r := range rs {
			if r.Esperado == lazulv1.Gesto_GESTO_NINGUNO {
				for _, e := range r.Emitidos {
					if e == lazulv1.Gesto_GESTO_ACTIVAR {
						falsas++
					}
				}
			}
		}
		v := m.PorGesto[lazulv1.Gesto_GESTO_ACTIVAR]
		fmt.Printf("    %4d ms → %d/%d · %d\n", ms, v[0], v[1], falsas)
	}
}
