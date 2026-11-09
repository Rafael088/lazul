// resumen: lee el registro de la prueba de factibilidad de la app de
// escritorio y dice cuántas activaciones hubo en los últimos 30 min de uso
// (y en la última prueba de trabajo normal), contra la meta de dec-q1p7:
// menos de 1 activación falsa en 30 min.
//
//	./dev app resumen [-registro RUTA]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rafael088/lazul/desktop"
)

func main() {
	ruta := flag.String("registro", "", "archivo del registro (por defecto el de la carpeta de estado del usuario)")
	flag.Parse()
	if *ruta == "" {
		d, err := desktop.DirEstado()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*ruta = filepath.Join(d, desktop.NombreRegistro)
	}
	ls, err := desktop.LeerRegistroDe(*ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("registro: %s (%d líneas)\n", *ruta, len(ls))
	fmt.Println(desktop.Resumir(ls).Texto())
}
