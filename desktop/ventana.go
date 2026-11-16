package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	NombreEstadoVentana = "ventana.json"

	// La app abre como un indicador pequeño en una esquina. La vista de la
	// cámara es una opción para acomodarse y se abre a demanda.
	AnchoIndicador  = 236
	AltoIndicador   = 64
	AnchoAjuste     = 920
	AltoAjuste      = 660
	MargenIndicador = 16

	AnchoVentanaMinimo = AnchoIndicador
	AltoVentanaMinimo  = AltoIndicador
)

// EstadoVentana conserva, cuando el sistema lo permite, la posición del
// indicador entre arranques. El tamaño ya no se restaura: la app siempre
// abre pequeña y la vista de cámara se pide a propósito.
type EstadoVentana struct {
	Ancho         int  `json:"ancho"`
	Alto          int  `json:"alto"`
	X             int  `json:"x,omitempty"`
	Y             int  `json:"y,omitempty"`
	TienePosicion bool `json:"tiene_posicion"`
}

func VentanaInicial() EstadoVentana {
	return EstadoVentana{Ancho: AnchoIndicador, Alto: AltoIndicador}
}

// PosicionEnEsquina deja una ventana en la esquina inferior derecha de la
// pantalla, con un margen que la separa del borde. Si la pantalla es más
// pequeña que la ventana, la pega a la esquina superior izquierda.
func PosicionEnEsquina(anchoVentana, altoVentana, anchoPantalla, altoPantalla, margen int) (int, int) {
	x := anchoPantalla - anchoVentana - margen
	y := altoPantalla - altoVentana - margen
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// LeerEstadoVentana devuelve los valores iniciales si el archivo todavía no
// existe. Un archivo inválido tampoco se aplica, para no dejar una ventana
// inutilizable fuera de la pantalla.
func LeerEstadoVentana(ruta string) (EstadoVentana, error) {
	inicial := VentanaInicial()
	b, err := os.ReadFile(ruta)
	if errors.Is(err, os.ErrNotExist) {
		return inicial, nil
	}
	if err != nil {
		return inicial, err
	}
	var e EstadoVentana
	if err := json.Unmarshal(b, &e); err != nil {
		return inicial, fmt.Errorf("%s: %w", NombreEstadoVentana, err)
	}
	if !estadoVentanaValido(e) {
		return inicial, fmt.Errorf("%s: tamaño inválido %dx%d", NombreEstadoVentana, e.Ancho, e.Alto)
	}
	return e, nil
}

// GuardarEstadoVentana escribe de forma atómica y privada para el usuario.
func GuardarEstadoVentana(ruta string, e EstadoVentana) error {
	if !estadoVentanaValido(e) {
		return fmt.Errorf("%s: tamaño inválido %dx%d", NombreEstadoVentana, e.Ancho, e.Alto)
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := ruta + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, ruta); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func estadoVentanaValido(e EstadoVentana) bool {
	// El límite superior evita restaurar basura sin imponer el tamaño de una
	// pantalla concreta (también admite escritorios con varios monitores).
	return e.Ancho >= AnchoVentanaMinimo && e.Alto >= AltoVentanaMinimo && e.Ancho <= 16_384 && e.Alto <= 16_384
}
