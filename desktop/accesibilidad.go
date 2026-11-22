package desktop

import (
	"context"
	"errors"

	"github.com/Rafael088/lazul/platform"
)

// AvisoReiniciarNavegadores es lo que se dice una sola vez, en la píldora y
// en la salida, cuando Lazul enciende la accesibilidad de la sesión: los
// navegadores que ya estaban abiertos no se registran en AT-SPI hacia atrás
// (lo miran al arrancar), así que el imán no los verá hasta reiniciarlos.
const AvisoReiniciarNavegadores = "accesibilidad encendida: reinicia los navegadores abiertos para que el imán los vea"

// ResultadoAccesibilidad dice qué pasó al preparar la accesibilidad para el
// imán.
type ResultadoAccesibilidad struct {
	// Usable es false si no hay con qué leer objetivos: el imán queda
	// apagado. Con la capacidad ausente (otros sistemas) es true.
	Usable bool
	// Encendida: Lazul la encendió ahora (antes estaba en false).
	Encendida bool
	// Aviso es lo que hay que enseñar a la persona; vacío si no hay nada.
	Aviso string
}

// PrepararAccesibilidad enciende la accesibilidad de la sesión si hace falta
// para que el imán vea los botones (Linux: org.a11y.Status.IsEnabled; ver
// platform.Accessibility). Nunca la apaga. Anota en el registro si la
// encendió Lazul; el aviso a la persona lo decide quien llama.
func PrepararAccesibilidad(ctx context.Context, a platform.Accessibility, reg *Registro) ResultadoAccesibilidad {
	if a == nil {
		// Este sistema no necesita preparación (p. ej. Windows).
		return ResultadoAccesibilidad{Usable: true}
	}
	res, err := a.Enable(ctx)
	switch {
	case errors.Is(err, platform.ErrSinAccesibilidad):
		reg.Anotar(Linea{Tipo: TipoAccesibilidad, Resultado: "sin_bus", Error: "sin_accesibilidad"})
		return ResultadoAccesibilidad{Aviso: "sin accesibilidad de la sesión (¿falta at-spi2-core?): el imán queda apagado"}
	case err != nil:
		// El bus existe pero no se pudo leer o encender: el imán puede
		// funcionar igual (la lectura va por otra conexión), así que sigue.
		reg.Anotar(Linea{Tipo: TipoAccesibilidad, Resultado: "error", Error: ClaseError(err)})
		return ResultadoAccesibilidad{Usable: true, Aviso: "no se pudo preparar la accesibilidad de la sesión: " + err.Error()}
	case res.TurnedOn:
		reg.Anotar(Linea{Tipo: TipoAccesibilidad, Resultado: "encendida"})
		return ResultadoAccesibilidad{Usable: true, Encendida: true, Aviso: AvisoReiniciarNavegadores}
	}
	reg.Anotar(Linea{Tipo: TipoAccesibilidad, Resultado: "ya_estaba"})
	return ResultadoAccesibilidad{Usable: true}
}
