//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type procesoCPU struct{ pid, ppid uint32 }

func procesosCPU() []procesoCPU {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)
	var entrada windows.ProcessEntry32
	entrada.Size = uint32(unsafe.Sizeof(entrada))
	var procesos []procesoCPU
	for err = windows.Process32First(snapshot, &entrada); err == nil; err = windows.Process32Next(snapshot, &entrada) {
		procesos = append(procesos, procesoCPU{entrada.ProcessID, entrada.ParentProcessID})
	}
	return procesos
}

func cpuProceso(pid uint32) float64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var creada, salida, kernel, usuario windows.Filetime
	if windows.GetProcessTimes(h, &creada, &salida, &kernel, &usuario) != nil {
		return 0
	}
	segundos := func(f windows.Filetime) float64 {
		return float64(uint64(f.HighDateTime)<<32|uint64(f.LowDateTime)) / 1e7
	}
	return segundos(kernel) + segundos(usuario)
}

// cpuArbol suma Lazul y los procesos WebView2 donde corre MediaPipe.
func cpuArbol() float64 {
	procesos := procesosCPU()
	dentro := map[uint32]bool{uint32(os.Getpid()): true}
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
			total += cpuProceso(p.pid)
		}
	}
	return total
}
