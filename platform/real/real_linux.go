//go:build linux

package real

import (
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/linux"
)

// New devuelve las capacidades de este equipo.
func New() platform.Sistema { return linux.New() }
