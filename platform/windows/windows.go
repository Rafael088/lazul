//go:build windows

package windows

import "github.com/Rafael088/lazul/platform"

// New devuelve las capacidades de este sistema.
func New() platform.Sistema {
	destino := &destinoPantalla{}
	tactil := nuevoTactilVirtual()
	tactil.destino = destino
	return platform.Sistema{
		Screenshot:      captura{},
		Clipboard:       portapapeles{},
		ActiveApp:       appActiva{},
		LaunchApp:       abrirApp{},
		TactileActions:  accionesTactiles{},
		AbsolutePointer: punteroAbsoluto{destino: destino},
		TouchScreen:     tactil,
		Displays:        pantallasWindows{destino: destino},
		Spaces:          espaciosWindows{},
		Targets:         objetivosWindows{destino: destino},
	}
}
