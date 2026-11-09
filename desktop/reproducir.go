package desktop

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Rafael088/lazul/gestures"
)

// Encadenar pone grabaciones una detrás de otra, con el tiempo seguido y
// un hueco sin mano entre ellas, como si fueran una sola toma de la cámara.
// Sirve para pasar por la app un recorrido entero (activar, agarrar,
// soltar) sin cámara: así se prueba la app de punta a punta en los 2 PCs.
func Encadenar(gs []gestures.Grabacion, huecoMs int64) []gestures.Cuadro {
	var cs []gestures.Cuadro
	var base int64
	for _, g := range gs {
		if len(g.Cuadros) == 0 {
			continue
		}
		t0 := g.Cuadros[0].TMs
		aspecto := float32(0)
		if g.Cabecera.Ancho > 0 && g.Cabecera.Alto > 0 {
			aspecto = float32(g.Cabecera.Ancho) / float32(g.Cabecera.Alto)
		}
		var ult int64
		for _, c := range g.Cuadros {
			c.TMs = base + c.TMs - t0
			if c.Aspecto == 0 {
				c.Aspecto = aspecto
			}
			cs = append(cs, c)
			ult = c.TMs
		}
		// El hueco: cuadros vacíos cada 33 ms.
		for t := ult + 33; t < ult+huecoMs; t += 33 {
			cs = append(cs, gestures.Cuadro{TMs: t, Aspecto: aspecto})
		}
		base = ult + huecoMs
	}
	return cs
}

// LeerRecorrido lee las grabaciones nombradas (rutas separadas por comas,
// relativas a dir si no son absolutas) en ese orden.
func LeerRecorrido(dir, lista string) ([]gestures.Grabacion, error) {
	var gs []gestures.Grabacion
	todas, err := gestures.LeerDirectorio(dir)
	if err != nil {
		return nil, err
	}
	por := map[string]gestures.Grabacion{}
	for _, g := range todas {
		por[filepath.Base(g.Archivo)] = g
	}
	for _, n := range strings.Split(lista, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if !strings.HasSuffix(n, ".jsonl") {
			n += ".jsonl"
		}
		g, ok := por[filepath.Base(n)]
		if !ok {
			return nil, fmt.Errorf("no está la grabación %s en %s", n, dir)
		}
		gs = append(gs, g)
	}
	return gs, nil
}
