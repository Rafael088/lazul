package gestures

import (
	"slices"
	"testing"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

const (
	gActivar = lazulv1.Gesto_GESTO_ACTIVAR
	gAgarrar = lazulv1.Gesto_GESTO_AGARRAR
	gSoltar  = lazulv1.Gesto_GESTO_SOLTAR
	gMandar  = lazulv1.Gesto_GESTO_MANDAR_APP
)

// correr pasa los tramos por el clasificador desde el modo m y devuelve los
// eventos.
func correr(m Modo, tramos ...tramo) []Evento {
	r := luzBuena
	r.semilla = 9
	e := NuevoEstado(m, AjustesPorDefecto)
	var evs []Evento
	for _, c := range sintetizar(colocacionN(0), r, tramos) {
		var ev Evento
		e, ev = Clasificar(e, c)
		if ev.Gesto != lazulv1.Gesto_GESTO_NINGUNO {
			evs = append(evs, ev)
		}
	}
	return evs
}

func gestosDe(evs []Evento) []lazulv1.Gesto {
	var gs []lazulv1.Gesto
	for _, e := range evs {
		gs = append(gs, e.Gesto)
	}
	return gs
}

func quiere(t *testing.T, nombre string, evs []Evento, gs ...lazulv1.Gesto) {
	t.Helper()
	if got := gestosDe(evs); !slices.Equal(got, gs) {
		t.Errorf("%s: salió %v, quiere %v", nombre, got, gs)
	}
}

func TestLSostenida(t *testing.T) {
	rel := tramo{f: formaRelajada(), ms: 300}
	quiere(t, "L de 250 ms", correr(ModoReposo, rel, tramo{f: formaL(), ms: 250}, rel))
	evs := correr(ModoReposo, rel, tramo{f: formaL(), ms: 600}, rel)
	quiere(t, "L de 600 ms", evs, gActivar)
	if len(evs) == 1 && evs[0].Confianza < 0.9 {
		t.Errorf("confianza %v con buena luz", evs[0].Confianza)
	}
	// Sostenerla mucho no la repite: el modo sigue activo mientras se vea.
	quiere(t, "L de 10 s", correr(ModoReposo, rel, tramo{f: formaL(), ms: 10000}, rel), gActivar)
}

func TestLOtraVezTrasCaducar(t *testing.T) {
	rel := tramo{f: formaRelajada(), ms: 300}
	l := tramo{f: formaL(), ms: 600, trans: 100}
	espera := tramo{f: formaRelajada(), ms: AjustesPorDefecto.VentanaActivo + 500, trans: 100}
	quiere(t, "dos L separadas", correr(ModoReposo, rel, l, espera, l, rel), gActivar, gActivar)
	quiere(t, "dos L seguidas (la segunda en activo)", correr(ModoReposo, rel, l, rel, l, rel), gActivar)
}

func TestGestosSinLNoCuentan(t *testing.T) {
	quiere(t, "puño en reposo", correr(ModoReposo, tramo{f: formaAbierta(), ms: 300}, tramo{f: formaPuno(), ms: 800, trans: 150}))
	quiere(t, "pinza arrastrada en reposo", correr(ModoReposo, tramo{f: formaPinza(true), ms: 300}, tramo{f: formaPinza(true), ms: 500, mov: v3{0.3, 0, 0}}))
}

func TestActivoCaduca(t *testing.T) {
	quiere(t, "puño 6 s después de la L", correr(ModoReposo,
		tramo{f: formaL(), ms: 600},
		tramo{f: formaRelajada(), ms: 6000, trans: 150},
		tramo{f: formaPuno(), ms: 600, trans: 150}), gActivar)
}

func TestFlujoCompleto(t *testing.T) {
	evs := correr(ModoReposo,
		tramo{f: formaRelajada(), ms: 300},
		tramo{f: formaL(), ms: 600, trans: 150},
		tramo{f: formaPuno(), ms: 600, trans: 150},
		tramo{f: formaAbierta(), ms: 400, trans: 200, mov: v3{-0.3, 0, 0}},
	)
	quiere(t, "L, puño y soltar", evs, gActivar, gAgarrar, gSoltar)
	// Hacia la izquierda de la imagen sin espejar es hacia la derecha de la persona.
	if len(evs) == 3 && evs[2].Direccion != lazulv1.Direccion_DIRECCION_DERECHA {
		t.Errorf("dirección %v", evs[2].Direccion)
	}
}

func TestDirecciones(t *testing.T) {
	soltar := func(dx float64) []Evento {
		return correr(ModoAgarrando, tramo{f: formaPuno(), ms: 400}, tramo{f: formaAbierta(), ms: 400, trans: 200, mov: v3{dx, 0, 0}})
	}
	for _, c := range []struct {
		dx  float64
		dir lazulv1.Direccion
	}{{0.3, lazulv1.Direccion_DIRECCION_IZQUIERDA}, {-0.3, lazulv1.Direccion_DIRECCION_DERECHA}} {
		evs := soltar(c.dx)
		if len(evs) != 1 || evs[0].Gesto != gSoltar || evs[0].Direccion != c.dir {
			t.Errorf("soltar dx=%v: %+v", c.dx, evs)
		}
	}
	quiere(t, "abrir sin moverse", soltar(0))

	mandar := func(mov v3) []Evento {
		return correr(ModoActivo, tramo{f: formaPinza(false), ms: 300}, tramo{f: formaPinza(false), ms: 500, mov: mov})
	}
	for _, c := range []struct {
		mov v3
		dir lazulv1.Direccion
	}{{v3{0, -0.25, 0}, lazulv1.Direccion_DIRECCION_ARRIBA}, {v3{0, 0.25, 0}, lazulv1.Direccion_DIRECCION_ABAJO}, {v3{0.25, 0, 0}, lazulv1.Direccion_DIRECCION_IZQUIERDA}} {
		evs := mandar(c.mov)
		if len(evs) != 1 || evs[0].Gesto != gMandar || evs[0].Direccion != c.dir {
			t.Errorf("mandar app %v: %+v", c.mov, evs)
		}
	}
	quiere(t, "pinza quieta", mandar(v3{}))
}

// Pura: el mismo estado y el mismo cuadro dan lo mismo, y el estado de
// entrada no cambia.
func TestClasificarEsPura(t *testing.T) {
	cs := sintetizar(colocacionN(0), luzBuena, []tramo{{f: formaL(), ms: 700}})
	var e Estado
	for _, c := range cs[:len(cs)-1] {
		e, _ = Clasificar(e, c)
	}
	copia := e
	e1, ev1 := Clasificar(e, cs[len(cs)-1])
	e2, ev2 := Clasificar(e, cs[len(cs)-1])
	if e != copia || e1 != e2 || ev1 != ev2 {
		t.Fatal("Clasificar no es pura")
	}
	if ev1.Gesto != gActivar && e1.Modo() != ModoActivo {
		t.Fatalf("tras 700 ms de L: %v en %v", ev1.Gesto, e1.Modo())
	}
}

// Las grabaciones sintéticas: todo acierta y no hay falsos positivos, con
// negativos de pinza y de señalar.
func TestSinteticasMiden(t *testing.T) {
	gs, err := LeerDirectorio(dirSinteticas)
	if err != nil {
		t.Fatal(err)
	}
	m, rs := Medir(gs)
	for _, r := range rs {
		if !r.Acierto() {
			t.Errorf("%s: esperaba %v, salió %v", r.Grabacion, r.Esperado, r.Emitidos)
		}
	}
	for g := gActivar; g <= gMandar; g++ {
		if m.PorGesto[g][1] == 0 {
			t.Errorf("sin grabaciones de %v", g)
		}
	}
	for _, clase := range []string{"pinza", "senalar", "normal"} {
		if m.PorNegativo[clase].Grabaciones == 0 {
			t.Errorf("sin negativos de %s", clase)
		}
	}
	if m.Precision() != 1 || m.FalsosPositivos != 0 {
		t.Errorf("precisión %.2f, %d falsos positivos", m.Precision(), m.FalsosPositivos)
	}
}

// Un sostén de la L más largo que la grabación no activa nunca: así el
// barrido de ./dev replay ve el efecto del tiempo de sostén.
func TestAjustesCambianElResultado(t *testing.T) {
	gs, err := LeerDirectorio(dirSinteticas)
	if err != nil {
		t.Fatal(err)
	}
	aj := AjustesPorDefecto
	aj.SostenerL = 5000
	m, _ := MedirCon(gs, aj)
	if v := m.PorGesto[gActivar]; v[0] != 0 {
		t.Errorf("con 5 s de sostén activó %d de %d", v[0], v[1])
	}
}
