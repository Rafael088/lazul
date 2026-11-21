//go:build windows

package windows

import (
	"context"
	"errors"
	"math"
	"testing"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
)

func TestTamanosWin32(t *testing.T) {
	if n := unsafe.Sizeof(monitorInfoEx{}); n != 104 {
		t.Fatalf("MONITORINFOEXW mide %d, quiere 104", n)
	}
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
		cs, err := s.cuadro(points, zonaTactil{ancho: 1921, alto: 1081})
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
	if _, err := s.cuadro([]platform.TouchPoint{{Slot: 2, X: .5, Y: .5, Down: true}}, zonaTactil{ancho: 10, alto: 10}); !errors.Is(err, platform.ErrAccionInvalida) {
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

func TestEstadoTactilEnOtroMonitor(t *testing.T) {
	// La externa (1920×1080) a la derecha de una principal de 2880 px: el
	// centro del monitor elegido cae en su centro, no en el de la principal.
	var s estadoTactil
	externa := zonaTactil{x: 2880, y: 0, ancho: 1921, alto: 1081}
	cs, err := s.cuadro([]platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}}, externa)
	if err != nil {
		t.Fatal(err)
	}
	revisar(t, cs, contactoEsperado{0, fAbajo, 2880 + 960, 540})
	// Una a la izquierda de la principal tiene X negativa.
	izquierda := zonaTactil{x: -1920, y: 120, ancho: 1921, alto: 1081}
	s = estadoTactil{}
	cs, _ = s.cuadro([]platform.TouchPoint{{Slot: 0, X: 0, Y: 1, Down: true}}, izquierda)
	revisar(t, cs, contactoEsperado{0, fAbajo, -1920, 120 + 1080})
}

func TestZonaDe(t *testing.T) {
	principal := func() (int32, int32) { return 2880, 1800 }
	if z := zonaDe(nil, principal); z != (zonaTactil{0, 0, 2880, 1800}) {
		t.Errorf("sin destino = %+v", z)
	}
	d := &destinoPantalla{}
	if z := zonaDe(d, principal); z != (zonaTactil{0, 0, 2880, 1800}) {
		t.Errorf("destino vacío = %+v", z)
	}
	d.poner(&platform.Display{Name: `\\.\DISPLAY2`, X: 2880, Y: 0, Width: 1920, Height: 1080})
	if z := zonaDe(d, principal); z != (zonaTactil{2880, 0, 1920, 1080}) {
		t.Errorf("con destino = %+v", z)
	}
}

func TestAbsolutoVirtual(t *testing.T) {
	virtual := zonaTactil{x: -1920, y: 0, ancho: 1920 + 2880, alto: 1800}
	casos := []struct {
		p      punto
		vx, vy int32
	}{
		{punto{-1920, 0}, 0, 0},
		{punto{2879, 1799}, rangoAbsoluto, rangoAbsoluto},
		{punto{0, 0}, int32(math.Round(1920 * rangoAbsoluto / 4799.0)), 0},
		{punto{-5000, 9000}, 0, rangoAbsoluto}, // fuera: se limita
	}
	for _, c := range casos {
		if vx, vy := absolutoVirtual(c.p, virtual); vx != c.vx || vy != c.vy {
			t.Errorf("absolutoVirtual(%v) = (%d,%d), quiere (%d,%d)", c.p, vx, vy, c.vx, c.vy)
		}
	}
}

func TestEstadoTactilAreaDeContacto(t *testing.T) {
	for _, c := range []struct {
		dpi   uint32
		medio int32
	}{{0, 18}, {96, 18}, {144, 27}, {192, 36}} {
		if m := medioLado(c.dpi); m != c.medio {
			t.Errorf("medioLado(%d) = %d, quiero %d", c.dpi, m, c.medio)
		}
	}
	// El DPI se pide al apoyar, en el píxel del dedo; mover y levantar
	// conservan el área aunque el monitor de debajo tenga otra escala.
	dpi := uint32(192)
	s := estadoTactil{dpi: func(punto) uint32 { return dpi }}
	paso := func(points []platform.TouchPoint) pointerTouchInfo {
		t.Helper()
		cs, err := s.cuadro(points, zonaTactil{ancho: 1001, alto: 1001})
		if err != nil || len(cs) != 1 {
			t.Fatalf("cuadro: %v %+v", err, cs)
		}
		s.aplicar(cs)
		return cs[0]
	}
	if c := paso([]platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}}); c.contacto != (rect{464, 464, 536, 536}) {
		t.Fatalf("área al apoyar a 200 %%: %+v", c.contacto)
	}
	dpi = 96
	if c := paso([]platform.TouchPoint{{Slot: 0, X: .6, Y: .5, Down: true}}); c.contacto != (rect{564, 464, 636, 536}) {
		t.Fatalf("área al mover: %+v", c.contacto)
	}
	if cs := s.sostenidos(); cs[0].contacto != (rect{564, 464, 636, 536}) {
		t.Fatalf("área sostenida: %+v", cs[0].contacto)
	}
	if c := paso(nil); c.contacto != (rect{564, 464, 636, 536}) || c.info.flags != punteroArriba {
		t.Fatalf("área al levantar: %+v", c)
	}
	if c := paso([]platform.TouchPoint{{Slot: 0, X: .5, Y: .5, Down: true}}); c.contacto != (rect{482, 482, 518, 518}) {
		t.Fatalf("un toque nuevo toma la escala de su monitor: %+v", c.contacto)
	}
}
