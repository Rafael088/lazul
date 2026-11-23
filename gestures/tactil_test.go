package gestures

import (
	"path/filepath"
	"slices"
	"testing"
)

func formaDosDedos() forma {
	return conPulgar(dedos([4]float64{0, 0, 1, 1}), v3{0.05, 0.72, -0.15})
}

// formaZoom mantiene índice, medio, anular y meñique en la misma postura y
// solo cambia la separación entre las puntas del índice y el pulgar.
func formaZoom(separacion float64) forma {
	f := dedos([4]float64{0.45, 1, 1, 1})
	f.phi[0] = 48 // no se parece a la L aunque la pinza esté abierta
	tip := f.puntos()[IndicePunta]
	f.pulgar = pulgarHacia(v3{tip.u + separacion, tip.v, tip.z})
	return f
}

func correrTactil(activo bool, tramos ...tramo) []EventoTactil {
	r := luzBuena
	r.semilla = 91
	e := NuevoEstadoTactil(activo, AjustesTactilesPorDefecto)
	var eventos []EventoTactil
	for _, c := range sintetizar(colocacionN(0), r, tramos) {
		var ev EventoTactil
		e, ev = ClasificarTactil(e, c)
		if ev.Accion != TactilNinguno {
			eventos = append(eventos, ev)
		}
	}
	return eventos
}

func formaTactilDeSintetica(f forma) (formaTactil, rasgos) {
	r := luzBuena
	r.semilla = 91
	cs := sintetizar(colocacionN(0), r, []tramo{{f: f, ms: 200}})
	c := cs[len(cs)-1]
	rs := medir(c.Manos[0], c.Aspecto)
	return formaTactilDe(rs), rs
}

func TestTactilAgarreSeDistingue(t *testing.T) {
	if f, r := formaTactilDeSintetica(formaAgarre()); f != formaTactilAgarre {
		t.Fatalf("el agarre no se reconoce: forma %v, puntas %.2f, medio %.2f, ext %.2f", f, r.puntas, r.medio, r.ext)
	}
	otras := map[string]forma{
		"puño": formaPuno(), "dos dedos": formaDosDedos(), "pellizco cerrado del zoom": formaZoom(0.08),
		"pellizco abierto del zoom": formaZoom(1.10), "mano abierta": formaAbierta(), "relajada": formaRelajada(),
		"L": formaL(), "señalar": formaSenalar(),
	}
	for nombre, f := range otras {
		if g, r := formaTactilDeSintetica(f); g == formaTactilAgarre {
			t.Errorf("%s se confunde con el agarre (puntas %.2f, medio %.2f, ext %.2f)", nombre, r.puntas, r.medio, r.ext)
		}
	}
}

// palancas devuelve la palanca de cada cuadro y los eventos.
func palancas(tramos ...tramo) ([]float64, []EventoTactil) {
	r := luzBuena
	r.semilla = 91
	e := NuevoEstadoTactil(true, AjustesTactilesPorDefecto)
	var ps []float64
	var eventos []EventoTactil
	for _, c := range sintetizar(colocacionN(0), r, tramos) {
		var ev EventoTactil
		e, ev = ClasificarTactil(e, c)
		if ev.Accion != TactilNinguno {
			eventos = append(eventos, ev)
		}
		ps = append(ps, e.Palanca())
	}
	return ps, eventos
}

func TestTactilPalancaSigueLaManoMientrasSeSostiene(t *testing.T) {
	agarra := tramo{f: formaAgarre(), ms: 300}
	baja := tramo{f: formaAgarre(), ms: 300, mov: v3{0, 0.24, 0}}
	sostiene := tramo{f: formaAgarre(), ms: 2000} // más que VentanaMovimiento
	vuelve := tramo{f: formaAgarre(), ms: 400, mov: v3{0, -0.24, 0}}
	quieta := tramo{f: formaAgarre(), ms: 300}
	suelta := tramo{f: formaRelajada(), ms: 600, trans: 100}

	ps, ev := palancas(agarra, baja, sostiene, vuelve, quieta, suelta)
	quiereTactiles(t, "un solo evento al empezar", ev, TactilScroll)
	if len(ev) == 1 && ev[0].Direccion != TactilAbajo {
		t.Errorf("dirección %v", ev[0].Direccion)
	}
	// cuadros a ~30 fps: el final de «sostiene» está ~2,5 s después del inicio.
	finSostiene := (300 + 300 + 2000) * 30 / 1000
	if p := ps[finSostiene-3]; p <= 0.3 {
		t.Errorf("sosteniendo la mano abajo la palanca debe seguir firme hacia abajo: %.2f", p)
	}
	finQuieta := (300 + 300 + 2000 + 400 + 300) * 30 / 1000
	if p := ps[finQuieta-3]; p != 0 {
		t.Errorf("con la mano de vuelta donde agarró la palanca debe ser 0: %.2f", p)
	}
	if p := ps[len(ps)-1]; p != 0 {
		t.Errorf("al soltar la pose la palanca debe ser 0: %.2f", p)
	}

	ps, ev = palancas(agarra, tramo{f: formaAgarre(), ms: 600, mov: v3{0, -0.24, 0}})
	quiereTactiles(t, "hacia arriba", ev, TactilScroll)
	if p := ps[len(ps)-1]; p >= 0 {
		t.Errorf("mano arriba: palanca %.2f, quiere negativa", p)
	}

	ps, _ = palancas(agarra, tramo{f: formaAgarre(), ms: 600, mov: v3{0, 0.04, 0}})
	for _, p := range ps {
		if p != 0 {
			t.Fatalf("un temblor no puede mover la palanca: %.2f", p)
		}
	}
}

func accionesTactiles(eventos []EventoTactil) []AccionTactil {
	acciones := make([]AccionTactil, 0, len(eventos))
	for _, ev := range eventos {
		acciones = append(acciones, ev.Accion)
	}
	return acciones
}

func quiereTactiles(t *testing.T, nombre string, eventos []EventoTactil, acciones ...AccionTactil) {
	t.Helper()
	if got := accionesTactiles(eventos); !slices.Equal(got, acciones) {
		t.Errorf("%s: salió %v, quiere %v", nombre, got, acciones)
	}
}

func TestTactilNoExponeCursorNiClic(t *testing.T) {
	var nombres []string
	for a := TactilNinguno; a <= TactilBarrido; a++ {
		nombres = append(nombres, a.String())
	}
	quiere := []string{"ninguno", "modo", "scroll", "zoom", "barrido"}
	if len(nombresTactiles) != len(quiere) || !slices.Equal(nombres, quiere) {
		t.Fatalf("acciones táctiles %v, quiere %v", nombres, quiere)
	}
}

func TestTactilLAlternaModo(t *testing.T) {
	rel := tramo{f: formaRelajada(), ms: 1100, trans: 100}
	l := tramo{f: formaL(), ms: 700, trans: 120}
	eventos := correrTactil(false, rel, l, rel, l, rel)
	quiereTactiles(t, "encender y apagar", eventos, TactilModo, TactilModo)
	if len(eventos) == 2 && (!eventos[0].Activo || eventos[1].Activo) {
		t.Errorf("estados del modo: %+v", eventos)
	}
	quiereTactiles(t, "L breve", correrTactil(false, rel, tramo{f: formaL(), ms: 250, trans: 100}, rel))
	quiereTactiles(t, "L sostenida no repite", correrTactil(false, tramo{f: formaL(), ms: 3000}), TactilModo)
}

func TestTactilScrollAgarre(t *testing.T) {
	base := tramo{f: formaAgarre(), ms: 250}
	casos := []struct {
		nombre string
		mov    v3
		dir    DireccionTactil
	}{
		{"arriba", v3{0, -0.22, 0}, TactilArriba},
		{"abajo", v3{0, 0.22, 0}, TactilAbajo},
	}
	for _, caso := range casos {
		eventos := correrTactil(true, base, tramo{f: formaAgarre(), ms: 450, mov: caso.mov})
		quiereTactiles(t, caso.nombre, eventos, TactilScroll)
		if len(eventos) == 1 && eventos[0].Direccion != caso.dir {
			t.Errorf("%s: dirección %v", caso.nombre, eventos[0].Direccion)
		}
	}
	quiereTactiles(t, "un dedo", correrTactil(true,
		tramo{f: formaSenalar(), ms: 250}, tramo{f: formaSenalar(), ms: 450, mov: v3{0, -0.3, 0}}))
	quiereTactiles(t, "movimiento corto", correrTactil(true,
		base, tramo{f: formaAgarre(), ms: 450, mov: v3{0, -0.08, 0}}))
}

func TestTactilLaPinzaYaNoHaceZoom(t *testing.T) {
	for _, c := range [][2]float64{{0.08, 1.10}, {1.10, 0.08}} {
		quiereTactiles(t, "pinza que abre o cierra", correrTactil(true,
			tramo{f: formaZoom(c[0]), ms: 250}, tramo{f: formaZoom(c[1]), ms: 500, trans: 400}))
	}
}

func TestTactilBarridoHorizontal(t *testing.T) {
	base := tramo{f: formaDosDedos(), ms: 250}
	casos := []struct {
		nombre string
		mov    v3
		dir    DireccionTactil
	}{
		// Imagen sin espejar: mover a la izquierda es la derecha de la persona.
		{"derecha", v3{-0.32, 0, 0}, TactilDerecha},
		{"izquierda", v3{0.32, 0, 0}, TactilIzquierda},
	}
	for _, caso := range casos {
		eventos := correrTactil(true, base, tramo{f: formaDosDedos(), ms: 450, mov: caso.mov})
		quiereTactiles(t, caso.nombre, eventos, TactilBarrido)
		if len(eventos) == 1 && eventos[0].Direccion != caso.dir {
			t.Errorf("%s: dirección %v", caso.nombre, eventos[0].Direccion)
		}
	}
	quiereTactiles(t, "diagonal", correrTactil(true,
		base, tramo{f: formaDosDedos(), ms: 450, mov: v3{0.3, 0.24, 0}}))
	quiereTactiles(t, "palma quieta", correrTactil(true, tramo{f: formaAbierta(), ms: 2000}))
	quiereTactiles(t, "saludo con palma", correrTactil(true,
		tramo{f: formaAbierta(), ms: 250}, tramo{f: formaAbierta(), ms: 450, mov: v3{0.5, 0, 0}}))
}

func TestTactilSoloRespondeActivoYSeRearma(t *testing.T) {
	dos := tramo{f: formaAgarre(), ms: 250}
	mov := tramo{f: formaAgarre(), ms: 450, mov: v3{0, -0.24, 0}}
	quiereTactiles(t, "scroll apagado", correrTactil(false, dos, mov))
	quiereTactiles(t, "una pose no repite", correrTactil(true, dos, mov, tramo{f: formaAgarre(), ms: 2000}), TactilScroll)
	quiereTactiles(t, "rearma tras neutral", correrTactil(true,
		dos, mov, tramo{f: formaRelajada(), ms: 500, trans: 100}, dos, mov), TactilScroll, TactilScroll)
}

func TestTactilIgnoraElRegresoDeLaMano(t *testing.T) {
	dos := tramo{f: formaAgarre(), ms: 250}
	baja := tramo{f: formaAgarre(), ms: 450, mov: v3{0, 0.24, 0}}
	sube := tramo{f: formaAgarre(), ms: 450, mov: v3{0, -0.24, 0}}
	suelta := tramo{f: formaRelajada(), ms: 500, trans: 100}

	// Bajar, volver arriba y bajar otra vez: dos scrolls abajo, ningún arriba.
	ev := correrTactil(true, dos, baja, suelta, dos, sube, suelta, dos, baja)
	quiereTactiles(t, "bombeo hacia abajo", ev, TactilScroll, TactilScroll)
	for _, e := range ev {
		if e.Direccion != TactilAbajo {
			t.Fatalf("el regreso de la mano contó como scroll: %+v", ev)
		}
	}

	// Tras una pausa más larga que IgnorarRegreso, cambiar de dirección vale.
	pausa := tramo{f: formaRelajada(), ms: AjustesTactilesPorDefecto.IgnorarRegreso + 200, trans: 100}
	ev = correrTactil(true, dos, baja, pausa, dos, sube)
	quiereTactiles(t, "cambio de dirección con pausa", ev, TactilScroll, TactilScroll)
	if len(ev) == 2 && ev[1].Direccion != TactilArriba {
		t.Fatalf("después de la pausa: %+v", ev)
	}
}

func TestTactilEsPuro(t *testing.T) {
	cs := sintetizar(colocacionN(0), luzBuena, []tramo{{f: formaL(), ms: 700}})
	e := NuevoEstadoTactil(false, AjustesTactilesPorDefecto)
	for _, c := range cs[:len(cs)-1] {
		e, _ = ClasificarTactil(e, c)
	}
	copia := e
	e1, ev1 := ClasificarTactil(e, cs[len(cs)-1])
	e2, ev2 := ClasificarTactil(e, cs[len(cs)-1])
	if e != copia || e1 != e2 || ev1 != ev2 {
		t.Fatal("ClasificarTactil no es puro")
	}
}

func TestTactilGrabacionesReales(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "gestos"))
	if err != nil {
		t.Fatal(err)
	}
	medicion, resultados := MedirTactil(gs)
	for _, r := range resultados {
		if !r.Acierto() {
			t.Errorf("%s: esperaba %v, salió %+v", r.Grabacion, r.Esperado, r.Emitidos)
		}
	}
	if medicion.Positivas != 25 || medicion.AciertosPos != 25 {
		t.Errorf("L real: %d/%d", medicion.AciertosPos, medicion.Positivas)
	}
	if medicion.FalsosPositivos != 0 || medicion.Negativas != len(gs)-25 {
		t.Errorf("reales: %d falsos en %d negativas", medicion.FalsosPositivos, medicion.Negativas)
	}
}

func TestTactilNegativasRealesEnModoActivo(t *testing.T) {
	gs, err := LeerDirectorio(filepath.Join("..", "fixtures", "gestos"))
	if err != nil {
		t.Fatal(err)
	}
	negativas := 0
	for _, g := range gs {
		if g.Cabecera.Gesto != "NINGUNO" {
			continue
		}
		negativas++
		r := ReproducirTactilDesde(g, true)
		if len(r.Emitidos) != 0 {
			t.Errorf("%s en modo activo: %+v", g.Archivo, r.Emitidos)
		}
	}
	if negativas != 51 {
		t.Errorf("se probaron %d negativas reales, quiere 51", negativas)
	}
}
