//go:build linux && cgo

package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 gtk-layer-shell-0
#include <stdlib.h>
#include "aura.h"
*/
import "C"

import (
	"encoding/json"
	"sync"
	"unsafe"
)

// Aura es la ventana layer-shell transparente que sigue a la mano. Es
// opcional: si el sistema no tiene gtk-layer-shell, la app sigue sin aura.
type Aura struct {
	mu  sync.Mutex
	ptr *C.Aura
}

// NuevaAura crea la superficie encima de todas las ventanas. La página HTML
// viene del frontend embebido y solo dibuja; no recibe clics ni foco.
func NuevaAura(html string) *Aura {
	h := C.CString(html)
	defer C.free(unsafe.Pointer(h))
	return &Aura{ptr: C.lazul_aura_crear(h)}
}

// Actualizar manda el estado a la página. App deduplica y limita el movimiento;
// esta capa conserva además solo el último estado si GTK todavía no lo pintó.
func (a *Aura) Actualizar(e EstadoAura) {
	if a == nil {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	js := C.CString(string(b))
	defer C.free(unsafe.Pointer(js))
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ptr == nil {
		return
	}
	C.lazul_aura_actualizar(a.ptr, js, C.double(e.X), C.double(e.Y))
}

// Cerrar destruye la superficie. Wails ya está cerrando: solo libera.
func (a *Aura) Cerrar() {
	if a == nil {
		return
	}
	a.mu.Lock()
	ptr := a.ptr
	a.ptr = nil
	a.mu.Unlock()
	if ptr != nil {
		C.lazul_aura_cerrar(ptr)
	}
}
