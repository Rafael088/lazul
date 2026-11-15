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

func TestTactilScrollDosDedos(t *testing.T) {
	base := tramo{f: formaDosDedos(), ms: 250}
	casos := []struct {
		nombre string
		mov    v3
		dir    DireccionTactil
	}{
		{"arriba", v3{0, -0.22, 0}, TactilArriba},
		{"abajo", v3{0, 0.22, 0}, TactilAbajo},
	}
	for _, caso := range casos {
		eventos := correrTactil(true, base, tramo{f: formaDosDedos(), ms: 450, mov: caso.mov})
		quiereTactiles(t, caso.nombre, eventos, TactilScroll)
		if len(eventos) == 1 && eventos[0].Direccion != caso.dir {
			t.Errorf("%s: dirección %v", caso.nombre, eventos[0].Direccion)
		}
	}
	quiereTactiles(t, "un dedo", correrTactil(true,
		tramo{f: formaSenalar(), ms: 250}, tramo{f: formaSenalar(), ms: 450, mov: v3{0, -0.3, 0}}))
	quiereTactiles(t, "movimiento corto", correrTactil(true,
		base, tramo{f: formaDosDedos(), ms: 450, mov: v3{0, -0.08, 0}}))
}

func TestTactilZoomAbreYCierra(t *testing.T) {
	casos := []struct {
		nombre        string
		inicio, final float64
		dir           DireccionTactil
	}{
		{"abre", 0.08, 1.10, TactilAbrir},
		{"cierra", 1.10, 0.08, TactilCerrar},
	}
	for _, caso := range casos {
		eventos := correrTactil(true,
			tramo{f: formaZoom(caso.inicio), ms: 250},
			tramo{f: formaZoom(caso.final), ms: 500, trans: 400})
		quiereTactiles(t, caso.nombre, eventos, TactilZoom)
		if len(eventos) == 1 && eventos[0].Direccion != caso.dir {
			t.Errorf("%s: dirección %v", caso.nombre, eventos[0].Direccion)
		}
	}
	quiereTactiles(t, "pellizco solo trasladado", correrTactil(true,
		tramo{f: formaZoom(0.12), ms: 250}, tramo{f: formaZoom(0.12), ms: 500, mov: v3{0.25, 0, 0}}))
	quiereTactiles(t, "cambio pequeño", correrTactil(true,
		tramo{f: formaZoom(0.35), ms: 250}, tramo{f: formaZoom(0.58), ms: 500, trans: 400}))
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
	dos := tramo{f: formaDosDedos(), ms: 250}
	mov := tramo{f: formaDosDedos(), ms: 450, mov: v3{0, -0.24, 0}}
	quiereTactiles(t, "scroll apagado", correrTactil(false, dos, mov))
	quiereTactiles(t, "una pose no repite", correrTactil(true, dos, mov, tramo{f: formaDosDedos(), ms: 2000}), TactilScroll)
	quiereTactiles(t, "rearma tras neutral", correrTactil(true,
		dos, mov, tramo{f: formaRelajada(), ms: 500, trans: 100}, dos, mov), TactilScroll, TactilScroll)
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
