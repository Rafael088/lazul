//go:build windows

package windows

import (
	"context"
	"errors"
	"testing"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
)

func TestTamanosWin32(t *testing.T) {
	if n := unsafe.Sizeof(pointerInfo{}); n != 96 {
		t.Errorf("POINTER_INFO mide %d; Windows espera 96", n)
	}
	if n := unsafe.Sizeof(pointerTouchInfo{}); n != 144 {
		t.Errorf("POINTER_TOUCH_INFO mide %d; Windows espera 144", n)
	}
	if n := unsafe.Sizeof(entradaRaton{}); n != 40 {
		t.Errorf("INPUT (ratón) mide %d; Windows espera 40", n)
	}
	if n := unsafe.Sizeof(entradaTeclado{}); n != 40 {
		t.Errorf("INPUT (teclado) mide %d; Windows espera 40", n)
	}
}

type contactoEsperado struct {
	id    uint32
	flags uint32
	x, y  int32
}

func revisar(t *testing.T, cs []pointerTouchInfo, quiero ...contactoEsperado) {
	t.Helper()
	if len(cs) != len(quiero) {
		t.Fatalf("contactos = %d, quiero %d: %+v", len(cs), len(quiero), cs)
	}
	for i, q := range quiero {
		c := cs[i]
		if c.info.tipo != ptTouch || c.info.id != q.id || c.info.flags != q.flags || c.info.pixel != (punto{q.x, q.y}) {
			t.Errorf("contacto %d = id %d flags %#x en %v; quiero id %d flags %#x en (%d,%d)",
				i, c.info.id, c.info.flags, c.info.pixel, q.id, q.flags, q.x, q.y)
		}
	}
}

const (
	fAbajo = punteroEnRango | punteroEnContacto | punteroAbajo
	fMover = punteroEnRango | punteroEnContacto | punteroActualizar
)

func TestEstadoTactilApoyarMoverLevantar(t *testing.T) {
	var s estadoTactil
	paso := func(points []platform.TouchPoint) []pointerTouchInfo {
		t.Helper()
		cs, err := s.cuadro(points, 1921, 1081)
		if err != nil {
			t.Fatal(err)
		}
		s.aplicar(cs)
		return cs
	}
	revisar(t, paso([]platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}}), contactoEsperado{0, fAbajo, 960, 540})
	revisar(t, paso([]platform.TouchPoint{{Slot: 0, X: .25, Y: .5, Down: true}}), contactoEsperado{0, fMover, 480, 540})
	// Segundo dedo: el primero sigue como actualización en el mismo cuadro.
	revisar(t, paso([]platform.TouchPoint{{Slot: 0, X: .25, Y: .5, Down: true}, {Slot: 1, X: .75, Y: .5, Down: true}}),
		contactoEsperado{0, fMover, 480, 540}, contactoEsperado{1, fAbajo, 1440, 540})
	revisar(t, s.sostenidos(), contactoEsperado{0, fMover, 480, 540}, contactoEsperado{1, fMover, 1440, 540})
	// Omitir la ranura 0 la levanta donde estaba.
	revisar(t, paso([]platform.TouchPoint{{Slot: 1, X: 1, Y: 1, Down: true}}),
		contactoEsperado{0, punteroArriba, 480, 540}, contactoEsperado{1, fMover, 1920, 1080})
	revisar(t, paso([]platform.TouchPoint{{Slot: 1, X: .1, Y: .1}}), contactoEsperado{1, punteroArriba, 1920, 1080})
	revisar(t, paso(nil))
	if len(s.sostenidos()) != 0 {
		t.Fatal("sin dedos no hay nada que sostener")
	}
}

func TestEstadoTactilInvalidos(t *testing.T) {
	var s estadoTactil
	if _, err := s.cuadro([]platform.TouchPoint{{Slot: 2, X: .5, Y: .5, Down: true}}, 10, 10); !errors.Is(err, platform.ErrAccionInvalida) {
		t.Fatalf("ranura 2: %v", err)
	}
	if s.abajo != [touchSlots]bool{} {
		t.Fatal("un cuadro inválido no cambia el estado")
	}
}

func TestPulsaciones(t *testing.T) {
	es := pulsaciones([]uint16{vkControl}, vkOemMas, 1)
	quiero := []struct {
		vk    uint16
		flags uint32
	}{{vkControl, 0}, {vkOemMas, 0}, {vkOemMas, teclaSoltar}, {vkControl, teclaSoltar}}
	if len(es) != len(quiero) {
		t.Fatalf("%d entradas", len(es))
	}
	for i, q := range quiero {
		if es[i].vk != q.vk || es[i].flags != q.flags || es[i].tipo != inputTeclado {
			t.Errorf("entrada %d = %+v; quiero %+v", i, es[i], q)
		}
	}
	if es := pulsaciones(nil, vkDown, 3); len(es) != 6 || es[0].flags != teclaExtendida || es[1].flags != teclaExtendida|teclaSoltar {
		t.Fatalf("flechas: %+v", es)
	}
}

func TestPunteroRechazaFuera(t *testing.T) {
	p := punteroAbsoluto{}
	for _, c := range [][2]float64{{-.1, .5}, {.5, 1.1}} {
		if err := p.MoveAbsolute(context.Background(), c[0], c[1]); !errors.Is(err, platform.ErrAccionInvalida) {
			t.Errorf("MoveAbsolute(%v): %v", c, err)
		}
	}
	if v, _ := coordenadaAbsoluta(1); v != rangoAbsoluto {
		t.Fatalf("1 → %d", v)
	}
}
