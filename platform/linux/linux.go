//go:build linux

package linux

import "github.com/Rafael088/lazul/platform"

// New devuelve las capacidades de este sistema.
func New() platform.Sistema {
	puntero := &punteroVirtual{}
	tactil := nuevoTactilVirtual()
	return platform.Sistema{
		Screenshot:      captura{},
		Clipboard:       portapapeles{},
		ActiveApp:       appActiva{},
		LaunchApp:       abrirApp{},
		TactileActions:  accionesTactiles{rueda: &ruedaVirtual{}},
		AbsolutePointer: puntero,
		TouchScreen:     tactil,
		Displays:        pantallasHyprland{ejecutar: ejecutarTactilReal, puntero: puntero, tactil: tactil},
		BorderResize:    bordeHyprland{ejecutar: ejecutarTactilReal},
	}
}
