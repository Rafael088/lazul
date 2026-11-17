//go:build !linux || !cgo

package main

// Aura sin layer-shell: Windows no tiene gtk-layer-shell y un binario sin cgo
// no puede dibujar la superficie. El halo se apaga y la app sigue igual; la
// señal visual sigue siendo la píldora del indicador.
type Aura struct{}

// NuevaAura no crea nada fuera de Linux con cgo.
func NuevaAura(string) *Aura { return nil }

// Actualizar no hace nada: no hay superficie que pintar.
func (a *Aura) Actualizar(EstadoAura) {}

// Cerrar no hace nada: no hay superficie que liberar.
func (a *Aura) Cerrar() {}
