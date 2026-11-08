//go:build windows

package windows_test

import (
	"testing"

	"github.com/Rafael088/lazul/platform/contrato"
	"github.com/Rafael088/lazul/platform/windows"
)

func TestContrato(t *testing.T) {
	contrato.Exigir(t)
	contrato.Correr(t, windows.New(), contrato.Opciones{AppInofensiva: "calc.exe"})
}
