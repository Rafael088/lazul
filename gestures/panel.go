package gestures

import (
	"errors"
	"math"
)

// MuestraPanel resume la posición de la palma y su tamaño aparente. X e Y
// están centradas en el eje óptico y expresadas en unidades de alto de la
// imagen; Tamano usa esas mismas unidades. Dividir posición por tamaño quita
// el cambio de perspectiva cuando la mano se acerca o se aleja de la cámara.
type MuestraPanel struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Tamano float64 `json:"tamano"`
}

// PuntoPanel es una posición normalizada dentro del rectángulo calibrado.
// (0,0) es la esquina superior izquierda de la persona y (1,1), la inferior
// derecha, aunque la cámara entregue la imagen sin espejar.
type PuntoPanel struct {
	X float64
	Y float64
}

// CalibracionPanel guarda las dos esquinas y el tamaño aparente de la mano en
// cada una. Dos puntos bastan para el panel frontal de esta primera versión.
type CalibracionPanel struct {
	Version           int          `json:"version"`
	SuperiorIzquierda MuestraPanel `json:"superior_izquierda"`
	InferiorDerecha   MuestraPanel `json:"inferior_derecha"`
}

const (
	versionCalibracionPanel = 1
	// La mano puede variar un 35 % de tamaño respecto al plano calibrado. Es
	// bastante para el ruido y el gesto, pero rechaza una mano claramente por
	// delante o por detrás del rectángulo imaginario.
	toleranciaProfundidadPanel = 0.35
)

// MuestraPanelDe usa solo huesos de la palma (muñeca y nudillos), no las
// puntas de los dedos: abrir o cerrar la mano cambia poco la medición.
func MuestraPanelDe(m Mano, aspecto float32) (MuestraPanel, bool) {
	if aspecto <= 0 {
		return MuestraPanel{}, false
	}
	indices := [...]int{0, 5, 9, 13, 17}
	var x, y float64
	for _, i := range indices {
		x += float64(m.Puntos[i].X)
		y += float64(m.Puntos[i].Y)
	}
	x /= float64(len(indices))
	y /= float64(len(indices))
	a := float64(aspecto)
	distancia := func(i, j int) float64 {
		dx := float64(m.Puntos[i].X-m.Puntos[j].X) * a
		dy := float64(m.Puntos[i].Y - m.Puntos[j].Y)
		return math.Hypot(dx, dy)
	}
	// Ancho de la palma y largo hasta el nudillo central. La hipotenusa es
	// estable incluso si una de las dos medidas queda casi de frente.
	tamano := math.Hypot(distancia(5, 17), distancia(0, 9))
	muestra := MuestraPanel{X: (x - 0.5) * a, Y: y - 0.5, Tamano: tamano}
	return muestra, muestraPanelValida(muestra)
}

func muestraPanelValida(m MuestraPanel) bool {
	return m.Tamano > 1e-4 && !math.IsNaN(m.X) && !math.IsNaN(m.Y) && !math.IsNaN(m.Tamano) &&
		!math.IsInf(m.X, 0) && !math.IsInf(m.Y, 0) && !math.IsInf(m.Tamano, 0)
}

func proyectarPanel(m MuestraPanel) PuntoPanel {
	return PuntoPanel{X: m.X / m.Tamano, Y: m.Y / m.Tamano}
}

// NuevaCalibracionPanel valida que las dos muestras sí formen la diagonal de
// un rectángulo útil. X puede crecer o decrecer porque MediaPipe recibe la
// imagen sin espejar; Y sí tiene que ir de arriba hacia abajo.
func NuevaCalibracionPanel(superiorIzquierda, inferiorDerecha MuestraPanel) (CalibracionPanel, error) {
	if !muestraPanelValida(superiorIzquierda) || !muestraPanelValida(inferiorDerecha) {
		return CalibracionPanel{}, errors.New("la mano no se pudo medir")
	}
	a, b := proyectarPanel(superiorIzquierda), proyectarPanel(inferiorDerecha)
	if math.Abs(b.X-a.X) < 0.35 {
		return CalibracionPanel{}, errors.New("las esquinas están demasiado cerca en horizontal")
	}
	if b.Y-a.Y < 0.25 {
		return CalibracionPanel{}, errors.New("la esquina inferior tiene que quedar claramente debajo de la superior")
	}
	return CalibracionPanel{Version: versionCalibracionPanel, SuperiorIzquierda: superiorIzquierda, InferiorDerecha: inferiorDerecha}, nil
}

// Valida comprueba una calibración leída del disco.
func (c CalibracionPanel) Valida() error {
	if c.Version != versionCalibracionPanel {
		return errors.New("versión de calibración desconocida")
	}
	_, err := NuevaCalibracionPanel(c.SuperiorIzquierda, c.InferiorDerecha)
	return err
}

// Mapear transforma una muestra en coordenadas de pantalla. dentro solo es
// true si la mano cae en el rectángulo y su tamaño aparente coincide con la
// profundidad del plano calibrado.
func (c CalibracionPanel) Mapear(m MuestraPanel) (p PuntoPanel, dentro bool) {
	if c.Valida() != nil || !muestraPanelValida(m) {
		return PuntoPanel{}, false
	}
	a, b, q := proyectarPanel(c.SuperiorIzquierda), proyectarPanel(c.InferiorDerecha), proyectarPanel(m)
	p.X = (q.X - a.X) / (b.X - a.X)
	p.Y = (q.Y - a.Y) / (b.Y - a.Y)
	if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
		return p, false
	}
	avance := (p.X + p.Y) / 2
	esperado := c.SuperiorIzquierda.Tamano + avance*(c.InferiorDerecha.Tamano-c.SuperiorIzquierda.Tamano)
	if esperado <= 0 || math.Abs(m.Tamano/esperado-1) > toleranciaProfundidadPanel {
		return p, false
	}
	return p, true
}

// AjustesOneEuro equilibran quietud y respuesta: a poca velocidad domina
// MinCutoff; al mover de verdad, Beta eleva el corte y reduce el retraso.
type AjustesOneEuro struct {
	MinCutoff        float64
	Beta             float64
	DerivativeCutoff float64
}

var AjustesOneEuroPorDefecto = AjustesOneEuro{MinCutoff: 1.2, Beta: 0.08, DerivativeCutoff: 1.0}

type filtroOneEuro1D struct {
	iniciado           bool
	tiempo, crudo      float64
	filtrado, derivada float64
}

func alfaOneEuro(corte, dt float64) float64 {
	if corte <= 0 || dt <= 0 {
		return 1
	}
	tau := 1 / (2 * math.Pi * corte)
	return 1 / (1 + tau/dt)
}

func (f *filtroOneEuro1D) filtrar(tiempo, valor float64, a AjustesOneEuro) float64 {
	if !f.iniciado {
		*f = filtroOneEuro1D{iniciado: true, tiempo: tiempo, crudo: valor, filtrado: valor}
		return valor
	}
	dt := tiempo - f.tiempo
	if dt <= 0 {
		return f.filtrado
	}
	derivadaCruda := (valor - f.crudo) / dt
	ad := alfaOneEuro(a.DerivativeCutoff, dt)
	f.derivada += ad * (derivadaCruda - f.derivada)
	corte := a.MinCutoff + a.Beta*math.Abs(f.derivada)
	ax := alfaOneEuro(corte, dt)
	f.filtrado += ax * (valor - f.filtrado)
	f.tiempo, f.crudo = tiempo, valor
	return f.filtrado
}

// FiltroOneEuroPanel suaviza los dos ejes de un punto. Es estado pequeño y
// determinista para poder probarlo con recorridos sintéticos.
type FiltroOneEuroPanel struct {
	Ajustes AjustesOneEuro
	x, y    filtroOneEuro1D
}

func NuevoFiltroOneEuroPanel() FiltroOneEuroPanel {
	return FiltroOneEuroPanel{Ajustes: AjustesOneEuroPorDefecto}
}

// Filtrar recibe el reloj monotónico del cuadro en milisegundos.
func (f *FiltroOneEuroPanel) Filtrar(tMs int64, p PuntoPanel) PuntoPanel {
	a := f.Ajustes
	if a == (AjustesOneEuro{}) {
		a = AjustesOneEuroPorDefecto
	}
	t := float64(tMs) / 1000
	return PuntoPanel{X: f.x.filtrar(t, p.X, a), Y: f.y.filtrar(t, p.Y, a)}
}

func (f *FiltroOneEuroPanel) Reiniciar() { *f = NuevoFiltroOneEuroPanel() }
