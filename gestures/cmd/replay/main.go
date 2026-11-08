// replay: pasa las grabaciones de fixtures/gestos por el clasificador y dice
// precisión y falsos positivos. Uso: ./dev replay [-dir fixtures/gestos] [-v]
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

func main() {
	dir := flag.String("dir", "fixtures/gestos", "carpeta con las grabaciones *.jsonl")
	detalle := flag.Bool("v", false, "una línea por grabación fallida")
	flag.Parse()
	gs, err := gestures.LeerDirectorio(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(gs) == 0 {
		fmt.Printf("sin grabaciones en %s (las graba cotejo-x2hyu0)\n", *dir)
		return
	}
	m, rs := gestures.Medir(gs)
	fmt.Printf("grabaciones: %d (%d con gesto, %d sin gesto)\n", len(gs), m.Positivas, m.Negativas)
	for g := lazulv1.Gesto_GESTO_ACTIVAR; g <= lazulv1.Gesto_GESTO_MANDAR_APP; g++ {
		if v, ok := m.PorGesto[g]; ok {
			fmt.Printf("  %-10s %d/%d\n", strings.TrimPrefix(g.String(), "GESTO_"), v[0], v[1])
		}
	}
	fmt.Printf("precisión: %.1f %% (meta ≥ 95 %%)\n", 100*m.Precision())
	fmt.Printf("falsos positivos: %d en %d grabaciones sin gesto\n", m.FalsosPositivos, m.Negativas)
	if *detalle {
		for _, r := range rs {
			if !r.Acierto() {
				fmt.Printf("  falla %s: esperaba %v, salió %v\n", r.Grabacion, r.Esperado, r.Emitidos)
			}
		}
	}
}
