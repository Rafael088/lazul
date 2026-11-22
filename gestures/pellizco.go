package gestures

import (
	"fmt"
	"math"
	"strings"
)

// Pellizco (modelo de manipulación directa, dec-fa72): el pulgar contra el
// índice es el embrague. La apertura se mide en largos de palma (muñeca a
// nudillo del medio) para que no dependa de la distancia a la cámara, y el
// estado se sostiene con histéresis para que no parpadee en el umbral.
//
// Las tomas del modelo nuevo (./dev grabar) guardan la apertura y el estado
// de cada mano en cada cuadro; los puntos crudos siguen ahí, así que el
// estado se puede recalcular si cambian los umbrales.

// AjustesPellizco son los umbrales de apertura, en largos de palma. Se
// cierra por debajo de UmbralCerrar y se abre por encima de UmbralAbrir:
// entre los dos el estado no cambia (histéresis). UmbralFirme es el segundo
// nivel del embrague: un pellizco que baja de él está claramente cerrado y
// apoya desde el primer cuadro; uno tibio (entre UmbralFirme y UmbralCerrar)
// sigue esperando CuadrosParaApoyar (cotejo-lgbapm). 0 lo apaga.
//
// En las tomas reales (fixtures/pellizco) el pellizco de verdad baja hasta
// 0,09 de mediana (p90 0,15) y el involuntario se queda en 0,18: con 0,30 se
// armaba sin que los dedos se tocaran. La calibración del panel puede fijar
// los tres a la mano del dueño (AjustesPellizcoPara).
type AjustesPellizco struct {
	UmbralCerrar float64
	UmbralAbrir  float64
	UmbralFirme  float64
}

var AjustesPellizcoPorDefecto = AjustesPellizco{
	UmbralCerrar: 0.20,
	UmbralAbrir:  0.45,
	UmbralFirme:  0.12,
}

// MaxPellizcoCerrado es la apertura más grande que se acepta como «dedos
// juntos» al calibrar: en las tomas, el 90 % de los pellizcos de verdad baja
// de 0,15 y el involuntario se queda en 0,18; por encima de 0,22 las puntas
// no se están tocando.
const MaxPellizcoCerrado = 0.22

// Márgenes de la calibración del pellizco, en largos de palma, sobre la
// apertura medida con los dedos juntos. Con 0,11 de margen para cerrar y 0,03
// para firme, el dueño de las tomas (≈0,09 con las puntas juntas) da los
// mismos 0,20 y 0,12 de por defecto. El mínimo de cerrar es 0,18: quieta y
// apretando, la mano mide menos que pellizcando en movimiento (mediana 0,13
// en los arrastres de las tomas) y con 0,16 las tomas pierden 7 aciertos.
const (
	margenCerrarCalibrado = 0.11
	margenFirmeCalibrado  = 0.03
	minCerrarCalibrado    = 0.18
	maxCerrarCalibrado    = 0.30 // el umbral de antes: nunca más generoso que eso
	minFirmeCalibrado     = 0.08
	huecoFirmeCerrar      = 0.06 // el firme queda al menos esto por debajo de cerrar
)

// AjustesPellizcoPara fija los umbrales a la mano del dueño a partir de la
// apertura de su pellizco con las puntas juntas: cerrar y firme un margen por
// encima, acotados para que una medida rara no deje el pellizco imposible ni
// vuelva al umbral generoso. Abrir no cambia: es el que corta el arrastre y
// lo mide el parpadeo, no la mano.
func AjustesPellizcoPara(cerrado float64) AjustesPellizco {
	aj := AjustesPellizcoPorDefecto
	if !(cerrado > 0 && cerrado <= MaxPellizcoCerrado) { // también NaN
		return aj
	}
	aj.UmbralCerrar = acotar(cerrado+margenCerrarCalibrado, minCerrarCalibrado, maxCerrarCalibrado)
	aj.UmbralFirme = acotar(cerrado+margenFirmeCalibrado, minFirmeCalibrado, aj.UmbralCerrar-huecoFirmeCerrar)
	return aj
}

func acotar(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// validarPellizcoCerrado acepta 0 (sin medir) o una apertura de dedos juntos.
func validarPellizcoCerrado(a float64) error {
	if a == 0 {
		return nil
	}
	if !(a > 0 && a <= MaxPellizcoCerrado) {
		return fmt.Errorf("pellizco calibrado fuera de rango: %v", a)
	}
	return nil
}

// EstadoPellizco es el pellizco de una mano en un cuadro.
type EstadoPellizco struct {
	Cerrado  bool    `json:"cerrado"`
	Apertura float32 `json:"apertura"` // distancia pulgar-índice / palma
}

// AperturaPellizco mide la distancia entre las puntas del pulgar y el índice
// en largos de palma (muñeca a nudillo del medio). Devuelve false si la mano
// es degenerada (palma de longitud cero).
func AperturaPellizco(m Mano, aspecto float32) (float64, bool) {
	asp := aspectoDe(aspecto)
	pulgar := vec{float64(m.Puntos[PulgarPunta].X) * asp, float64(m.Puntos[PulgarPunta].Y)}
	indice := vec{float64(m.Puntos[IndicePunta].X) * asp, float64(m.Puntos[IndicePunta].Y)}
	muneca := vec{float64(m.Puntos[Muneca].X) * asp, float64(m.Puntos[Muneca].Y)}
	medio := vec{float64(m.Puntos[MedioMCP].X) * asp, float64(m.Puntos[MedioMCP].Y)}
	palma := dist(muneca, medio)
	if palma < 1e-6 {
		return 0, false
	}
	return dist(pulgar, indice) / palma, true
}

// ClaveMano identifica una mano entre cuadros: el lado que da MediaPipe y,
// si aparecen dos del mismo lado, el orden. Así la histéresis sigue a la
// misma mano y una mano nueva empieza abierta. Es también la clave que
// publica el embrague, para que quien lea sus resultados pueda atar la
// posición de una mano a la misma mano del cuadro anterior.
type ClaveMano struct {
	Derecha bool `json:"derecha"`
	N       int  `json:"n"`
}

// String da una forma legible de la clave ("izquierda-0", "derecha-1").
func (c ClaveMano) String() string {
	lado := "izquierda"
	if c.Derecha {
		lado = "derecha"
	}
	return fmt.Sprintf("%s-%d", lado, c.N)
}

// MedidorPellizco calcula el estado de cada mano cuadro a cuadro, con
// histéresis. Es la misma cuenta que hace ./dev grabar al guardar una toma y
// la que muestra en vivo la página de grabar.
type MedidorPellizco struct {
	Ajustes AjustesPellizco
	estado  map[ClaveMano]bool
}

// NuevoMedidorPellizco arranca con todas las manos abiertas.
func NuevoMedidorPellizco(aj AjustesPellizco) *MedidorPellizco {
	return &MedidorPellizco{Ajustes: aj, estado: map[ClaveMano]bool{}}
}

// Cuadro avanza el estado con las manos de un cuadro. Devuelve un estado por
// mano, en el mismo orden en que vienen. Si una mano desaparece, su estado se
// descarta: al volver empieza abierta.
func (m *MedidorPellizco) Cuadro(manos []Mano, aspecto float32) []EstadoPellizco {
	if m.estado == nil {
		m.estado = map[ClaveMano]bool{}
	}
	aj := m.Ajustes
	if aj == (AjustesPellizco{}) {
		aj = AjustesPellizcoPorDefecto
	}
	estados := make([]EstadoPellizco, len(manos))
	vistos := map[bool]int{}
	presentes := make(map[ClaveMano]bool, len(manos))
	for i, mano := range manos {
		clave := ClaveMano{Derecha: mano.Derecha, N: vistos[mano.Derecha]}
		vistos[mano.Derecha]++
		presentes[clave] = true
		apertura, ok := AperturaPellizco(mano, aspecto)
		cerrado := m.estado[clave]
		if ok {
			cerrado = aj.siguiente(apertura, cerrado)
			m.estado[clave] = cerrado
		} else {
			cerrado = false
		}
		estados[i] = EstadoPellizco{Cerrado: cerrado, Apertura: float32(apertura)}
	}
	for clave := range m.estado {
		if !presentes[clave] {
			delete(m.estado, clave)
		}
	}
	return estados
}

// siguiente aplica la histéresis: cerrado se queda cerrado hasta pasar el
// umbral de abrir; abierto solo se cierra por debajo del umbral de cerrar.
func (a AjustesPellizco) siguiente(apertura float64, cerrado bool) bool {
	if cerrado {
		return apertura <= a.UmbralAbrir
	}
	return apertura < a.UmbralCerrar
}

// AnotarPellizco calcula la apertura y el estado de cada mano, cuadro a
// cuadro, y los deja en una copia de la grabación. Los cuadros originales no
// se tocan.
func AnotarPellizco(g Grabacion) Grabacion {
	return AnotarPellizcoCon(g, AjustesPellizcoPorDefecto)
}

// AnotarPellizcoCon es AnotarPellizco con otros ajustes.
func AnotarPellizcoCon(g Grabacion, aj AjustesPellizco) Grabacion {
	medidor := NuevoMedidorPellizco(aj)
	cuadros := make([]Cuadro, len(g.Cuadros))
	for i, c := range g.Cuadros {
		manos := append([]Mano(nil), c.Manos...)
		estados := medidor.Cuadro(manos, c.Aspecto)
		for j := range manos {
			e := estados[j]
			manos[j].Pellizco = &e
		}
		c.Manos = manos
		cuadros[i] = c
	}
	g.Cuadros = cuadros
	return g
}

// ResumenPellizco cuenta lo que ve una toma, para decirlo al guardarla.
type ResumenPellizco struct {
	Cuadros            int // cuadros de la toma
	CuadrosConPellizco int // con al menos una mano cerrada
	Cierres            int // veces que una mano pasó de abierta a cerrada
	Manos              int // cuadros-mano medidos
}

// Fraccion de cuadros con pellizco, de 0 a 1.
func (r ResumenPellizco) Fraccion() float64 {
	if r.Cuadros == 0 {
		return 0
	}
	return float64(r.CuadrosConPellizco) / float64(r.Cuadros)
}

// ResumirPellizco cuenta los estados ya anotados en los cuadros.
func ResumirPellizco(cuadros []Cuadro) ResumenPellizco {
	r := ResumenPellizco{Cuadros: len(cuadros)}
	previo := map[int]bool{}
	for _, c := range cuadros {
		presentes := make(map[int]bool, len(c.Manos))
		con := false
		for i, mano := range c.Manos {
			if mano.Pellizco == nil {
				continue
			}
			presentes[i] = true
			r.Manos++
			if mano.Pellizco.Cerrado {
				con = true
				if !previo[i] {
					r.Cierres++
				}
			}
			previo[i] = mano.Pellizco.Cerrado
		}
		for i := range previo {
			if !presentes[i] {
				delete(previo, i)
			}
		}
		if con {
			r.CuadrosConPellizco++
		}
	}
	return r
}

// TomasPellizco son las tomas del modelo nuevo que ofrece ./dev grabar.
var TomasPellizco = []string{
	"arrastrar-arriba",
	"arrastrar-abajo",
	"arrastrar-izquierda",
	"arrastrar-derecha",
	"entrar-panel",
	"salir-panel",
	"zoom-dos-manos",
	"normal",
	// Cambiar de pantalla con tres dedos (tres.go): deslizar a ese lado.
	"tres-derecha",
	"tres-izquierda",
}

// TomaPellizcoValida dice si la etiqueta es una de las tomas del modelo nuevo.
func TomaPellizcoValida(s string) bool {
	for _, t := range TomasPellizco {
		if s == t {
			return true
		}
	}
	return false
}

// validarCabeceraPellizco exige que "toma" sea una etiqueta conocida, que la
// toma sea NINGUNO (no cuelga de los cuatro gestos viejos) y que el panel,
// si viene, sea un rectángulo válido en coordenadas normalizadas.
func validarCabeceraPellizco(c Cabecera) error {
	if c.Toma == "" {
		if c.Panel != nil {
			return fmt.Errorf("panel sin toma")
		}
		return nil
	}
	if !TomaPellizcoValida(c.Toma) {
		return fmt.Errorf("toma desconocida %q", c.Toma)
	}
	if !strings.EqualFold(c.Gesto, "NINGUNO") {
		return fmt.Errorf("la toma %q tiene que ir con gesto NINGUNO", c.Toma)
	}
	return c.Panel.validar()
}

// validar comprueba el rectángulo del panel en coordenadas normalizadas.
func (p *Panel) validar() error {
	if p == nil {
		return nil
	}
	for _, v := range []float64{p.X0, p.Y0, p.X1, p.Y1} {
		if v < 0 || v > 1 {
			return fmt.Errorf("panel fuera de la imagen: %v", v)
		}
	}
	if p.X1 <= p.X0 || p.Y1 <= p.Y0 {
		return fmt.Errorf("panel vacío: %+v", *p)
	}
	return nil
}
