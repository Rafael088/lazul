//go:build linux

package linux

import (
	"errors"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

func TestTactilCodificaApoyarMoverYLevantar(t *testing.T) {
	e := nuevoEstadoTactil()

	b, err := e.eventos([]platform.TouchPoint{{Slot: 0, X: .25, Y: .75, Down: true}})
	if err != nil {
		t.Fatal(err)
	}
	quererEventos(t, b,
		eventoLeido{evAbs, absMtSlot, 0},
		eventoLeido{evAbs, absMtTracking, 1},
		eventoLeido{evAbs, absMtX, 720},
		eventoLeido{evAbs, absMtY, 1350},
		eventoLeido{evAbs, absX, 720},
		eventoLeido{evAbs, absY, 1350},
		eventoLeido{evAbs, absMtSlot, 1},
		eventoLeido{evKey, btnTouch, 1},
		eventoLeido{evKey, btnTool1, 1},
		eventoLeido{evKey, btnTool2, 0},
		eventoLeido{evSyn, synReport, 0},
	)

	b, err = e.eventos([]platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}})
	if err != nil {
		t.Fatal(err)
	}
	quererEventos(t, b,
		eventoLeido{evAbs, absMtSlot, 0},
		eventoLeido{evAbs, absMtX, 1440},
		eventoLeido{evAbs, absMtY, 900},
		eventoLeido{evAbs, absX, 1440},
		eventoLeido{evAbs, absY, 900},
		eventoLeido{evAbs, absMtSlot, 1},
		eventoLeido{evKey, btnTool1, 1},
		eventoLeido{evKey, btnTool2, 0},
		eventoLeido{evSyn, synReport, 0},
	)

	b, err = e.eventos(nil)
	if err != nil {
		t.Fatal(err)
	}
	quererEventos(t, b,
		eventoLeido{evAbs, absMtSlot, 0},
		eventoLeido{evAbs, absMtTracking, -1},
		eventoLeido{evAbs, absMtSlot, 1},
		eventoLeido{evKey, btnTouch, 0},
		eventoLeido{evKey, btnTool1, 0},
		eventoLeido{evKey, btnTool2, 0},
		eventoLeido{evSyn, synReport, 0},
	)
}

func TestTactilUsaDosRanurasYTrackingIDsNuevos(t *testing.T) {
	e := nuevoEstadoTactil()
	b, err := e.eventos([]platform.TouchPoint{
		{Slot: 1, X: .75, Y: .5, Down: true},
		{Slot: 0, X: .25, Y: .5, Down: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	ev := leerEventos(b)
	// Las ranuras se serializan siempre en orden estable, aunque el llamador
	// entregue los dedos en otro orden.
	if ev[0] != (eventoLeido{evAbs, absMtSlot, 0}) || ev[1] != (eventoLeido{evAbs, absMtTracking, 1}) ||
		ev[6] != (eventoLeido{evAbs, absMtSlot, 1}) || ev[7] != (eventoLeido{evAbs, absMtTracking, 2}) {
		t.Fatalf("ranuras o tracking IDs: %+v", ev)
	}
	if ev[len(ev)-3] != (eventoLeido{evKey, btnTool1, 0}) || ev[len(ev)-2] != (eventoLeido{evKey, btnTool2, 1}) {
		t.Fatalf("herramienta de dos dedos: %+v", ev)
	}

	if _, err := e.eventos(nil); err != nil {
		t.Fatal(err)
	}
	b, err = e.eventos([]platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}})
	if err != nil {
		t.Fatal(err)
	}
	ev = leerEventos(b)
	if ev[1] != (eventoLeido{evAbs, absMtTracking, 3}) {
		t.Fatalf("el nuevo contacto debe estrenar tracking ID: %+v", ev)
	}
}

func TestTactilRechazaCuadrosInvalidos(t *testing.T) {
	casos := [][]platform.TouchPoint{
		{{Slot: 0, X: 0, Y: 0}, {Slot: 1, X: 0, Y: 0}, {Slot: 0, X: 0, Y: 0}},
		{{Slot: -1, X: .5, Y: .5, Down: true}},
		{{Slot: 2, X: .5, Y: .5, Down: true}},
		{{Slot: 0, X: .5, Y: .5}, {Slot: 0, X: .5, Y: .5}},
		{{Slot: 0, X: -.01, Y: .5, Down: true}},
		{{Slot: 0, X: .5, Y: 1.01, Down: true}},
	}
	for _, puntos := range casos {
		e := nuevoEstadoTactil()
		if _, err := e.eventos(puntos); !errors.Is(err, platform.ErrAccionInvalida) {
			t.Errorf("%+v: error %v", puntos, err)
		}
	}
}

func quererEventos(t *testing.T, b []byte, esperados ...eventoLeido) {
	t.Helper()
	recibidos := leerEventos(b)
	if len(recibidos) != len(esperados) {
		t.Fatalf("eventos:\n recibidos: %+v\n esperados: %+v", recibidos, esperados)
	}
	for i := range esperados {
		if recibidos[i] != esperados[i] {
			t.Fatalf("evento %d:\n recibido: %+v\n esperado: %+v\n todos: %+v", i, recibidos[i], esperados[i], recibidos)
		}
	}
}
