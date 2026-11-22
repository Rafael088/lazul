package main

import (
	"context"

	"github.com/Rafael088/lazul/desktop"
)

// entradaVentana controla la parte nativa de la ventana principal. En
// Windows el indicador es una ventana auxiliar: no debe activar Lazul y solo
// la pildora visible puede recibir el raton. La vista de camara, abierta a
// proposito, vuelve a ser una ventana normal.
type entradaVentana interface {
	Preparar() error
	Mostrar(context.Context) error
	Camara(bool) error
	Cerrar()
}

const (
	claseVentanaLazul = "LazulIndicador"
	margenPildora     = 6
)

// rectPildora devuelve la zona visible del indicador dentro de la ventana.
// Se calcula desde la esquina inferior derecha porque asi esta colocado el
// elemento CSS. Si Wails deja por error la ventana grande, todo lo demas
// queda fuera de la region nativa y los clics pasan a la app de abajo.
func rectPildora(ancho, alto int) (izquierda, arriba, derecha, abajo int) {
	anchoPildora := desktop.AnchoIndicador - 2*margenPildora
	altoPildora := desktop.AltoIndicador - 2*margenPildora
	derecha = max(0, ancho-margenPildora)
	abajo = max(0, alto-margenPildora)
	izquierda = max(0, derecha-anchoPildora)
	arriba = max(0, abajo-altoPildora)
	return
}
