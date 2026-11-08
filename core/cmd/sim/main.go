// sim: un controlador y N receptores falsos en local. Uso: ./dev sim [-receptores N] [-mdns]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Rafael088/lazul/core/descubre"
	"github.com/Rafael088/lazul/core/sim"
)

func main() {
	n := flag.Int("receptores", 2, "cuántos receptores falsos")
	pings := flag.Int("pings", 50, "idas y vueltas por receptor para medir latencia")
	mdns := flag.Bool("mdns", false, "descubrir por mDNS en la red local de verdad (si no, en memoria)")
	flag.Parse()
	o := sim.Opciones{Receptores: *n, Pings: *pings}
	if *mdns {
		o.Red = descubre.MDNS{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := sim.Correr(ctx, o, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
