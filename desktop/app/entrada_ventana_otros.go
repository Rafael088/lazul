//go:build !windows

package main

import "context"

const iniciarVentanaOculta = false

type entradaVentanaOtros struct{}

func nuevaEntradaVentana() entradaVentana                 { return entradaVentanaOtros{} }
func (entradaVentanaOtros) Preparar() error               { return nil }
func (entradaVentanaOtros) Mostrar(context.Context) error { return nil }
func (entradaVentanaOtros) Camara(bool) error             { return nil }
func (entradaVentanaOtros) Cerrar()                       {}
