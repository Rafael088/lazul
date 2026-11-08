//go:build linux

package linux_test

import (
	"testing"

	"github.com/Rafael088/lazul/platform/contrato"
	"github.com/Rafael088/lazul/platform/linux"
)

func TestContrato(t *testing.T) {
	contrato.Exigir(t)
	contrato.Correr(t, linux.New(), contrato.Opciones{AppInofensiva: "org.gnome.Calculator"})
}
