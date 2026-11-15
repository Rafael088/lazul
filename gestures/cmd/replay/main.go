// replay: pasa las grabaciones por el clasificador y dice precisión y falsos
// positivos. Por defecto mide las reales (fixtures/gestos) y las sintéticas
// (gestures/testdata/sinteticas), cada una por su lado.
//
//	./dev replay [-dir D1,D2] [-v] [-sostener-l MS] [-barrer] [-tactil]
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
	detalle := flag.Bool("v", false, "una línea por grabación fallida")
	tactil := flag.Bool("tactil", false, "mide modo táctil: L, scroll, zoom y barrido")
	sostenerL := flag.Int64("sostener-l", gestures.AjustesPorDefecto.SostenerL, "ms que se sostiene la L para activar")
	barrer := flag.Bool("barrer", false, "mide la L con varios tiempos de sostén (200 a 800 ms)")
	flag.Parse()
	if *tactil && *dirs == dirsPredeterminados {
		*dirs += ",gestures/testdata/tactiles"
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
		if *tactil {
			informeTactil(dir, gs, *detalle)
		} else {
			informe(dir, gs, aj, *detalle)
		}
		if *barrer && !*tactil {
			barrido(gs, aj)
		}
	}
	if fallo {
		os.Exit(1)
	}
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
