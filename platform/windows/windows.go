//go:build windows

package windows

import "github.com/Rafael088/lazul/platform"

// New devuelve las capacidades de este sistema.
func New() platform.Sistema {
	return platform.Sistema{
		Screenshot: captura{},
		Clipboard:  portapapeles{},
		ActiveApp:  appActiva{},
		LaunchApp:  abrirApp{},
	}
}
