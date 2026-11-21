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
		eventoLeido{evAbs, absMtMayor, 72},
		eventoLeido{evAbs, absMtMenor, 72},
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
		ev[8] != (eventoLeido{evAbs, absMtSlot, 1}) || ev[9] != (eventoLeido{evAbs, absMtTracking, 2}) {
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

func TestTactilAreaDeContacto(t *testing.T) {
	// 36 px lógicos en la principal (1440 lógicos) son 72 unidades; en un
	// monitor de 1920 lógicos, 54: el dedo mide lo mismo en pantalla.
	for _, c := range []struct{ ancho, quiero int }{{0, 72}, {1440, 72}, {1920, 54}, {960, 108}} {
		if u := unidadesContacto(c.ancho); int(u) != c.quiero {
			t.Errorf("unidadesContacto(%d) = %d, quiero %d", c.ancho, u, c.quiero)
		}
	}
	tv := &tactilVirtual{estado: nuevoEstadoTactil()}
	if err := tv.Recrear(1920); err != nil {
		t.Fatal(err)
	}
	ev := leerEventos(mustEventos(t, &tv.estado, []platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}}))
	if ev[2] != (eventoLeido{evAbs, absMtMayor, 54}) || ev[3] != (eventoLeido{evAbs, absMtMenor, 54}) {
		t.Fatalf("área al apoyar: %+v", ev)
	}
	// Mover el dedo apoyado no vuelve a mandar el área: el arrastre ve el
	// mismo contacto.
	for _, e := range leerEventos(mustEventos(t, &tv.estado, []platform.TouchPoint{{Slot: 0, X: .6, Y: .5, Down: true}})) {
		if e.codigo == absMtMayor || e.codigo == absMtMenor {
			t.Fatalf("el área se repite al mover: %+v", e)
		}
	}
}

func mustEventos(t *testing.T, e *estadoTactil, p []platform.TouchPoint) []byte {
	t.Helper()
	b, err := e.eventos(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
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
