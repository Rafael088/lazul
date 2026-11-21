package gestures

import (
	"math"
	"sort"
)

// Medición de los toques (tarea cotejo-vsddsi): qué sale del embrague y de
// ToquesDe con las tomas reales de fixtures/pellizco, y qué haría la app con
// cada toque. Los umbrales son de medida, no de producción: sirven para ver
// qué reparte la app, no para cambiarla. Si un toque no es tap, ni pulsación
// larga, ni arrastre, queda sin clase (una pulsación de entre 250 y 500 ms
// quieta: la app probablemente la trate como tap, pero la medida no lo da por
// hecho).

const (
	// ToqueTapMs es el máximo de un toque para ser tap.
	ToqueTapMs int64 = 250
	// ToqueLargaMs es el mínimo de una pulsación larga.
	ToqueLargaMs int64 = 500
	// ToqueQuieto es el recorrido máximo (en unidades del panel, 0..1) para
	// que un toque siga siendo tap o pulsación larga.
	ToqueQuieto = 0.05
	// toqueMsApartar es el tiempo que se simula que tarda la mano en
	// apartarse: las tomas se cortan al dejar de mover la mano y muchas
	// terminan con el pellizco cerrado. Se añaden cuadros sin manos (15 fps)
	// hasta que el embrague suelte, como pasa de verdad al irse la mano.
	toqueMsApartar      int64 = 67
	toqueCuadrosApartar       = 12
)

// TipoToque es lo que haría la app con un toque ya terminado.
type TipoToque string

const (
	ToqueTap      TipoToque = "tap"
	ToqueLarga    TipoToque = "larga"
	ToqueArrastre TipoToque = "arrastre"
	ToqueSinClase TipoToque = "sin clase"
)

// Toque es un dedo apoyado y levantado: lo que el táctil virtual habrá visto.
type Toque struct {
	Ranura    int        `json:"ranura"`
	InicioMs  int64      `json:"inicio_ms"`
	FinMs     int64      `json:"fin_ms"`
	Inicio    PuntoPanel `json:"inicio"`
	Fin       PuntoPanel `json:"fin"`
	Recorrido float64    `json:"recorrido"`
	Tipo      TipoToque  `json:"tipo"`
}

// DuracionMs es lo que el dedo estuvo apoyado, del apoyo al levantar.
func (t Toque) DuracionMs() int64 { return t.FinMs - t.InicioMs }

// clasificarToque reparte un toque según lo que duró y lo que se movió.
func clasificarToque(duracionMs int64, recorrido float64) TipoToque {
	if recorrido > ToqueQuieto {
		return ToqueArrastre
	}
	if duracionMs < ToqueTapMs {
		return ToqueTap
	}
	if duracionMs > ToqueLargaMs {
		return ToqueLarga
	}
	return ToqueSinClase
}

// TomaToques es lo medido en una toma de fixtures/pellizco.
type TomaToques struct {
	Archivo       string
	Toma          string
	Toques        []Toque
	FueraDelPanel int // pellizcos que empezaron fuera y el embrague descartó
	// Alcance y salto al apoyar (precision_toques.go).
	Bordes, BordesPosibles int
	Saltos                 []float64
	Temblor, Retraso       []float64
}

// ConteoToques resume una o varias tomas.
type ConteoToques struct {
	Tomas          int
	TomasConToques int
	Toques         int
	Tap            int
	Larga          int
	Arrastre       int
	SinClase       int
	FueraDelPanel  int
}

// ContarToques suma las medidas. Se puede llamar con una toma o con todas.
func ContarToques(tomas []TomaToques) ConteoToques {
	c := ConteoToques{Tomas: len(tomas)}
	for _, t := range tomas {
		if len(t.Toques) > 0 {
			c.TomasConToques++
		}
		c.Toques += len(t.Toques)
		c.FueraDelPanel += t.FueraDelPanel
		for _, toque := range t.Toques {
			switch toque.Tipo {
			case ToqueTap:
				c.Tap++
			case ToqueLarga:
				c.Larga++
			case ToqueArrastre:
				c.Arrastre++
			default:
				c.SinClase++
			}
		}
	}
	return c
}

// MedirToques pasa todas las tomas por el embrague y ToquesDe, con el tamaño
// de palma mediano de todas ellas como calibración, y devuelve una medida por
// toma.
func MedirToques(gs []Grabacion) ([]TomaToques, error) {
	tam := TamanoMedianoPellizco(gs)
	tomas := make([]TomaToques, 0, len(gs))
	for _, g := range gs {
		t, err := ToquesDeToma(g, tam)
		if err != nil {
			return nil, err
		}
		tomas = append(tomas, t)
	}
	return tomas, nil
}

// ToquesDeToma pasa una toma por el embrague y ToquesDe y empareja cada dedo
// apoyado con su levantar.
func ToquesDeToma(g Grabacion, tamano float64) (TomaToques, error) {
	cal, err := CalibracionDeToma(g, tamano)
	if err != nil {
		return TomaToques{}, err
	}
	med := NuevoMedidorEmbrague(cal)
	filtro := filtroPanelApp{cal: cal}
	puntero := punteroApp{filtro: NuevoFiltroOneEuroPanel()}
	seg := seguimientoPrecision{cal: cal}
	resultados := make([]ResultadoEmbrague, 0, len(g.Cuadros)+toqueCuadrosApartar)
	var tUltimo int64
	var aspecto float32
	for _, c := range g.Cuadros {
		tUltimo, aspecto = c.TMs, c.Aspecto
		r := med.Cuadro(filtro.cuadro(c))
		puntero.cuadro(r)
		seg.cuadro(c, r, &puntero)
		resultados = append(resultados, r)
	}
	// Se aparta la mano de la toma, como en la prueba de las tomas reales.
	for i := 0; i < toqueCuadrosApartar && len(resultados) > 0 &&
		resultados[len(resultados)-1].ManosArmadas > 0; i++ {
		tUltimo += toqueMsApartar
		resultados = append(resultados, med.Cuadro(Cuadro{TMs: tUltimo, Aspecto: aspecto}))
	}
	return TomaToques{
		Archivo:        g.Archivo,
		Toma:           g.Cabecera.Toma,
		Toques:         toquesDeCuadros(ToquesDe(resultados)),
		FueraDelPanel:  pellizcosFueraDelPanel(g, cal),
		Bordes:         seg.bordes,
		BordesPosibles: seg.posibles,
		Saltos:         seg.saltos,
		Temblor:        puntero.temblor,
		Retraso:        puntero.retraso,
	}, nil
}

// toquesDeCuadros empareja los cuadros del táctil en toques: de un apoyo al
// levantar de la misma ranura. Un dedo que siga apoyado al final se cierra en
// el último cuadro, para que ninguna toma quede con un toque a medias.
func toquesDeCuadros(cuadros []CuadroToques) []Toque {
	abiertos := map[int]*Toque{}
	var toques []Toque
	for _, c := range cuadros {
		for _, d := range c.Dedos {
			p := PuntoPanel{X: d.X, Y: d.Y}
			if d.Apoyado {
				t := abiertos[d.Ranura]
				if t == nil {
					t = &Toque{Ranura: d.Ranura, InicioMs: c.TMs, Inicio: p}
					abiertos[d.Ranura] = t
				} else {
					t.Recorrido += math.Hypot(p.X-t.Fin.X, p.Y-t.Fin.Y)
				}
				t.Fin = p
				continue
			}
			if t := abiertos[d.Ranura]; t != nil {
				t.Fin = p
				cerrarToque(t, c.TMs)
				toques = append(toques, *t)
				delete(abiertos, d.Ranura)
			}
		}
	}
	var ultimoMs int64
	if len(cuadros) > 0 {
		ultimoMs = cuadros[len(cuadros)-1].TMs
	}
	ranuras := make([]int, 0, len(abiertos))
	for n := range abiertos {
		ranuras = append(ranuras, n)
	}
	sort.Ints(ranuras)
	for _, n := range ranuras {
		cerrarToque(abiertos[n], ultimoMs)
		toques = append(toques, *abiertos[n])
	}
	sort.Slice(toques, func(a, b int) bool {
		if toques[a].InicioMs != toques[b].InicioMs {
			return toques[a].InicioMs < toques[b].InicioMs
		}
		return toques[a].Ranura < toques[b].Ranura
	})
	return toques
}

// cerrarToque fija el final y la clase de un toque.
func cerrarToque(t *Toque, finMs int64) {
	t.FinMs = finMs
	t.Tipo = clasificarToque(t.DuracionMs(), t.Recorrido)
}

// pellizcosFueraDelPanel cuenta los pellizcos que se cierran con la mano
// fuera del panel. El embrague no los arma (dec-fa72), así que no llegan a
// ser toque: se cuentan aparte para saber cuántos gestos involuntarios atrapa
// la regla. Se usa el mismo MedidorPellizco que el embrague, no el estado
// guardado en la toma, para que la cuenta hable del mismo umbral.
func pellizcosFueraDelPanel(g Grabacion, cal CalibracionPanel) int {
	med := NuevoMedidorPellizco(AjustesPellizcoPorDefecto)
	cerrada := map[ClaveMano]bool{}
	fuera := 0
	for _, c := range g.Cuadros {
		estados := med.Cuadro(c.Manos, c.Aspecto)
		vistos := map[bool]int{}
		for i, m := range c.Manos {
			clave := ClaveMano{Derecha: m.Derecha, N: vistos[m.Derecha]}
			vistos[m.Derecha]++
			antes := cerrada[clave]
			cerrada[clave] = estados[i].Cerrado
			if !estados[i].Cerrado || antes {
				continue
			}
			dentro := false
			if mu, ok := MuestraPanelDe(m, c.Aspecto); ok {
				_, dentro = cal.Mapear(mu)
			}
			if !dentro {
				fuera++
			}
		}
	}
	return fuera
}
