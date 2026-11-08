package main

import (
	"os"
	"strconv"
	"strings"
)

const clkTck = 100.0 // sysconf(_SC_CLK_TCK) en Linux x86_64

type proc struct {
	pid, ppid int
	nombre    string
	cpu       float64
}

func leerProcs() []proc {
	ents, _ := os.ReadDir("/proc")
	var ps []proc
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(b)
		i, j := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
		if i < 0 || j < 0 {
			continue
		}
		f := strings.Fields(s[j+2:])
		if len(f) < 15 {
			continue
		}
		ppid, _ := strconv.Atoi(f[1])
		ut, _ := strconv.ParseFloat(f[11], 64)
		st, _ := strconv.ParseFloat(f[12], 64)
		ps = append(ps, proc{pid, ppid, s[i+1 : j], (ut + st) / clkTck})
	}
	return ps
}

func arbol() []proc {
	ps := leerProcs()
	dentro := map[int]bool{os.Getpid(): true}
	for cambio := true; cambio; {
		cambio = false
		for _, p := range ps {
			if !dentro[p.pid] && dentro[p.ppid] {
				dentro[p.pid], cambio = true, true
			}
		}
	}
	var out []proc
	for _, p := range ps {
		if dentro[p.pid] {
			out = append(out, p)
		}
	}
	return out
}

// cpuArbol: segundos de CPU (usuario+sistema) de este proceso y sus descendientes vivos.
func cpuArbol() float64 {
	t := 0.0
	for _, p := range arbol() {
		t += p.cpu
	}
	return t
}

func procesosArbol() []string {
	var n []string
	for _, p := range arbol() {
		n = append(n, p.nombre)
	}
	return n
}
