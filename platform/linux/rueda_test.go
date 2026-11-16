//go:build linux

package linux

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type eventoLeido struct {
	tipo, codigo uint16
	valor        int32
}

func leerEventos(b []byte) []eventoLeido {
	var r []eventoLeido
	for i := 0; i+24 <= len(b); i += 24 {
		r = append(r, eventoLeido{
			tipo:   binary.LittleEndian.Uint16(b[i+16:]),
			codigo: binary.LittleEndian.Uint16(b[i+18:]),
			valor:  int32(binary.LittleEndian.Uint32(b[i+20:])),
		})
	}
	return r
}

func TestRuedaAcumulaPasosYSubeConValorPositivo(t *testing.T) {
	r := &ruedaVirtual{}
	// 60 hacia abajo: medio paso, solo alta resolución.
	ev := leerEventos(r.eventos(60))
	if len(ev) != 2 || ev[0] != (eventoLeido{evRel, relWheelHiRes, -60}) || ev[1].tipo != evSyn {
		t.Fatalf("medio paso: %+v", ev)
	}
	// Otros 70: completa un paso entero de REL_WHEEL (hacia abajo = -1).
	ev = leerEventos(r.eventos(70))
	if len(ev) != 3 || ev[1] != (eventoLeido{evRel, relWheel, -1}) {
		t.Fatalf("paso completo: %+v", ev)
	}
	// Hacia arriba: valores positivos.
	ev = leerEventos(r.eventos(-250))
	if ev[0].valor != 250 || ev[1] != (eventoLeido{evRel, relWheel, 2}) {
		t.Fatalf("arriba: %+v", ev)
	}
}

// Con LAZUL_RUEDA_REAL=1 crea el dispositivo de verdad y comprueba que
// Hyprland lo ve. Mueve la rueda medio paso abajo y medio arriba (neto 0).
func TestRuedaRealHyprland(t *testing.T) {
	if os.Getenv("LAZUL_RUEDA_REAL") != "1" {
		t.Skip("LAZUL_RUEDA_REAL=1 para crear el ratón virtual de verdad")
	}
	r := &ruedaVirtual{}
	if err := r.Preparar(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	salida, err := exec.Command("hyprctl", "devices").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(string(salida)), "lazul-rueda-virtual") {
		t.Fatalf("Hyprland no ve la rueda:\n%s", salida)
	}
	if err := r.ScrollContinuo(context.Background(), 60); err != nil {
		t.Fatal(err)
	}
	if err := r.ScrollContinuo(context.Background(), -60); err != nil {
		t.Fatal(err)
	}
}
