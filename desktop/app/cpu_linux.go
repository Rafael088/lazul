//go:build linux

package main

import (
	"os"
	"strconv"
	"strings"
)

const clkTck = 100.0

type procesoCPU struct {
	pid, ppid int
	cpu       float64
}

func procesosCPU() []procesoCPU {
	entradas, _ := os.ReadDir("/proc")
	var procesos []procesoCPU
	for _, entrada := range entradas {
		pid, err := strconv.Atoi(entrada.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + entrada.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(b)
		fin := strings.LastIndexByte(s, ')')
		if fin < 0 {
			continue
		}
		campos := strings.Fields(s[fin+2:])
		if len(campos) < 15 {
			continue
		}
		ppid, _ := strconv.Atoi(campos[1])
		usuario, _ := strconv.ParseFloat(campos[11], 64)
		sistema, _ := strconv.ParseFloat(campos[12], 64)
		procesos = append(procesos, procesoCPU{pid: pid, ppid: ppid, cpu: (usuario + sistema) / clkTck})
	}
	return procesos
}

// cpuArbol suma la CPU de Lazul, ffmpeg y la webview donde corre MediaPipe.
func cpuArbol() float64 {
	procesos := procesosCPU()
	dentro := map[int]bool{os.Getpid(): true}
	for cambio := true; cambio; {
		cambio = false
		for _, p := range procesos {
			if !dentro[p.pid] && dentro[p.ppid] {
				dentro[p.pid], cambio = true, true
			}
		}
	}
	var total float64
	for _, p := range procesos {
		if dentro[p.pid] {
			total += p.cpu
		}
	}
	return total
}
