package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type proc struct {
	pid, ppid uint32
	nombre    string
}

func arbol() []proc {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var ps []proc
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		ps = append(ps, proc{e.ProcessID, e.ParentProcessID, windows.UTF16ToString(e.ExeFile[:])})
	}
	dentro := map[uint32]bool{uint32(os.Getpid()): true}
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

func cpuDe(pid uint32) float64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var c, x, k, u windows.Filetime
	if windows.GetProcessTimes(h, &c, &x, &k, &u) != nil {
		return 0
	}
	t := func(f windows.Filetime) float64 {
		return float64(uint64(f.HighDateTime)<<32|uint64(f.LowDateTime)) / 1e7
	}
	return t(k) + t(u)
}

// cpuArbol: segundos de CPU de este proceso y sus descendientes vivos (WebView2 incluido).
func cpuArbol() float64 {
	s := 0.0
	for _, p := range arbol() {
		s += cpuDe(p.pid)
	}
	return s
}

func procesosArbol() []string {
	var n []string
	for _, p := range arbol() {
		n = append(n, p.nombre)
	}
	return n
}
