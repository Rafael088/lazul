// sim: un controlador y N receptores falsos en local. Uso: ./dev sim [-receptores N]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Rafael088/lazul/core/sim"
)

func main() {
	n := flag.Int("receptores", 2, "cuántos receptores falsos")
	pings := flag.Int("pings", 50, "idas y vueltas por receptor para medir latencia")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := sim.Correr(ctx, *n, *pings, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
