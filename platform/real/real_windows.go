//go:build windows

package real

import (
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/windows"
)

// New devuelve las capacidades de este equipo.
func New() platform.Sistema { return windows.New() }
