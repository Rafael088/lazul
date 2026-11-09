//go:build !linux && !windows

package real

import (
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/vacio"
)

// New devuelve las capacidades de este equipo (ninguna fuera de Linux y Windows).
func New() platform.Sistema { return vacio.New() }
