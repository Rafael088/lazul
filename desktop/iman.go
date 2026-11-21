package desktop

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// Imán sobre los botones (asistencia de puntería, nivel 2). Con la lista de
// elementos que se pueden tocar en la ventana con el foco (la da la
// accesibilidad del sistema, platform.TargetFinder):
//
//   - el puntero va más lento encima de un objetivo chico (FrenoIman), para
//     que la mano que tiembla no se salga de él;
//   - al apoyar el dedo, si cae cerca de un objetivo chico pero fuera de
//     todos, el toque se lleva a ese objetivo (AjustarToque); si hay duda
//     (dos objetivos casi igual de cerca) no se corrige.
//
// Todas las medidas van en píxeles lógicos (los CSS a escala 1).
const (
	// RadioIman: hasta dónde va a buscar el toque un objetivo. Es la mitad
	// del dedo de platform.DiametroContacto: lo que un dedo de verdad
	// habría rozado.
	RadioIman = platform.DiametroContacto / 2
	// MargenDudaIman: si el segundo objetivo más cercano está a menos de
	// esto más lejos que el primero, no se sabe cuál se quería. Con 8 px
	// la prueba sintética corrige más pero agrega 4 veces más toques en un
	// objetivo que no se quería (TestImanNoTocaObjetivosQueNoSeQuerian).
	MargenDudaIman = 12.0
	// LadoChicoIman: un objetivo es «chico» si su lado corto no pasa de
	// esto (un botón o un enlace de una línea). Los grandes se aciertan
	// solos y moverse a su centro sería un salto largo.
	LadoChicoIman = 48.0
	// LadoLargoMaxIman: y si su lado largo no pasa de esto (descarta barras
	// y contenedores que la accesibilidad marca como botón).
	LadoLargoMaxIman = 480.0
	// AdentroIman: en un objetivo largo (un enlace de varias palabras) el
	// toque no va al centro, que puede estar lejos, sino a esto del borde.
	AdentroIman = 12.0
	// GananciaFrenoIman: cuánto avanza el puntero encima de un objetivo
	// chico por cada píxel que avanza la mano.
	GananciaFrenoIman = 0.5
	// RecuperarFrenoIman: fuera de los objetivos, qué parte de la distancia
	// a la mano recupera el puntero en cada cuadro (a 25 cuadros/s, en
	// ~150 ms ya va con la mano).
	RecuperarFrenoIman = 0.35
)

// Tiempos de la caché por ventana.
const (
	// ConsultarVentanaCada: cada cuánto se mira qué ventana tiene el foco
	// (GetForegroundWindow: microsegundos).
	ConsultarVentanaCada = 250 * time.Millisecond
	// LeerImanCada: cada cuánto se vuelve a leer la misma ventana mientras
	// la mano está en el panel (la página pudo cambiar o desplazarse).
	LeerImanCada = 1500 * time.Millisecond
	// VigenciaIman: una lista más vieja que esto no corrige nada.
	VigenciaIman = 2500 * time.Millisecond
	// EsperaTrasToqueIman: después de levantar un dedo (un tap abre otra
	// página, un arrastre desplaza con inercia) lo leído ya no vale y se
	// espera esto antes de leer otra vez.
	EsperaTrasToqueIman = 400 * time.Millisecond
	// RepetirToqueIman: un segundo toque así de pronto y cerca del primero
	// (doble toque) repite el mismo ajuste, aunque la lista ya no valga.
	RepetirToqueIman = 500 * time.Millisecond
)

// rectIman es un objetivo en píxeles lógicos.
type rectIman struct{ x0, y0, x1, y1 float64 }

func (r rectIman) contiene(x, y float64) bool {
	return x >= r.x0 && x <= r.x1 && y >= r.y0 && y <= r.y1
}

func (r rectIman) distancia(x, y float64) float64 {
	dx := math.Max(math.Max(r.x0-x, 0), x-r.x1)
	dy := math.Max(math.Max(r.y0-y, 0), y-r.y1)
	return math.Hypot(dx, dy)
}

func (r rectIman) chico() bool {
	w, h := r.x1-r.x0, r.y1-r.y0
	return math.Min(w, h) <= LadoChicoIman && math.Max(w, h) <= LadoLargoMaxIman
}

// destino es el punto del objetivo al que va el toque: el centro en un
// objetivo chico y, en uno largo, el punto del objetivo más cercano a
// AdentroIman del borde.
func (r rectIman) destino(x, y float64) (float64, float64) {
	eje := func(v, a, b float64) float64 {
		m := math.Min((b-a)/2, AdentroIman)
		return math.Min(math.Max(v, a+m), b-m)
	}
	return eje(x, r.x0, r.x1), eje(y, r.y0, r.y1)
}

// ObjetivosIman son los objetivos de una ventana listos para el imán.
type ObjetivosIman struct {
	todos        []rectIman // todos: si el toque cae en uno, ya acierta
	chicos       []rectIman // los que atraen y frenan
	ancho, alto  float64    // tamaño lógico de (0,0)-(1,1)
	Ventana      uint64
	Leido        time.Time
	CantChicos   int
	CantObjetivo int
}

// NuevosObjetivosIman pasa el conjunto del sistema a píxeles lógicos y
// junta los repetidos (un enlace dentro de un botón del mismo tamaño).
func NuevosObjetivosIman(s platform.TargetSet) *ObjetivosIman {
	o := &ObjetivosIman{ancho: s.LogicalWidth, alto: s.LogicalHeight, Ventana: s.Window}
	if o.ancho <= 0 || o.alto <= 0 {
		return o
	}
	for _, t := range s.Targets {
		if !(t.Width > 0 && t.Height > 0) || math.IsNaN(t.X) || math.IsNaN(t.Y) {
			continue
		}
		r := rectIman{t.X * o.ancho, t.Y * o.alto, (t.X + t.Width) * o.ancho, (t.Y + t.Height) * o.alto}
		if r.x1 < 0 || r.y1 < 0 || r.x0 > o.ancho || r.y0 > o.alto {
			continue // fuera de este monitor
		}
		if repetido(o.todos, r) {
			continue
		}
		o.todos = append(o.todos, r)
		if r.chico() {
			o.chicos = append(o.chicos, r)
		}
	}
	o.CantObjetivo, o.CantChicos = len(o.todos), len(o.chicos)
	return o
}

// repetido: casi el mismo rectángulo que uno ya visto (bordes a ≤ 2 px).
func repetido(rs []rectIman, r rectIman) bool {
	for _, v := range rs {
		if math.Abs(v.x0-r.x0) <= 2 && math.Abs(v.y0-r.y0) <= 2 && math.Abs(v.x1-r.x1) <= 2 && math.Abs(v.y1-r.y1) <= 2 {
			return true
		}
	}
	return false
}

// ResultadoAjuste dice qué hizo AjustarToque. Nunca lleva posiciones al
// registro: solo el resultado.
type ResultadoAjuste string

const (
	AjusteNada    ResultadoAjuste = ""         // sin objetivos cerca o ya cae en uno
	AjusteMovido  ResultadoAjuste = "ajusta"   // el toque se llevó al objetivo
	AjusteDuda    ResultadoAjuste = "duda"     // dos objetivos casi igual de cerca: no se corrige
	AjusteDentro  ResultadoAjuste = "acierta"  // ya caía dentro de un objetivo
	AjusteSinWeb  ResultadoAjuste = "sin_web"  // sin lista vigente
	AjusteRepetir ResultadoAjuste = "repetido" // doble toque: repite el ajuste anterior
)

// AjustarToque lleva el punto p (normalizado) al objetivo chico más
// cercano si p no cae en ningún objetivo y ese está a RadioIman o menos.
// Con duda no lo mueve. Devuelve el punto que hay que tocar.
func (o *ObjetivosIman) AjustarToque(p gestures.PuntoPanel) (gestures.PuntoPanel, ResultadoAjuste) {
	if o == nil || o.ancho <= 0 || o.alto <= 0 {
		return p, AjusteSinWeb
	}
	x, y := p.X*o.ancho, p.Y*o.alto
	for _, r := range o.todos {
		if r.contiene(x, y) {
			return p, AjusteDentro
		}
	}
	mejor, segunda := -1, math.Inf(1)
	d1 := math.Inf(1)
	for i, r := range o.chicos {
		d := r.distancia(x, y)
		if d > RadioIman+MargenDudaIman {
			continue
		}
		if d < d1 {
			segunda, d1, mejor = d1, d, i
		} else if d < segunda {
			segunda = d
		}
	}
	if mejor < 0 || d1 > RadioIman {
		return p, AjusteNada
	}
	if segunda-d1 < MargenDudaIman {
		return p, AjusteDuda
	}
	dx, dy := o.chicos[mejor].destino(x, y)
	return gestures.PuntoPanel{X: dx / o.ancho, Y: dy / o.alto}, AjusteMovido
}

// sobreChico dice si p (normalizado) está encima de un objetivo chico.
func (o *ObjetivosIman) sobreChico(p gestures.PuntoPanel) bool {
	if o == nil || o.ancho <= 0 {
		return false
	}
	x, y := p.X*o.ancho, p.Y*o.alto
	for _, r := range o.chicos {
		if r.contiene(x, y) {
			return true
		}
	}
	return false
}

// FrenoIman hace que el puntero avance más lento encima de un objetivo
// chico. Encima, cada paso de la mano mueve el puntero GananciaFrenoIman de
// ese paso; fuera, el puntero se mueve con la mano y además recupera la
// distancia que le quedó. Nunca se separa de la mano más de RadioIman.
type FrenoIman struct {
	salida, anterior gestures.PuntoPanel
	hay              bool
}

// Aplicar recibe la posición de la mano (ya filtrada) y devuelve la del
// puntero. Con o nil (sin lista vigente) el puntero va con la mano.
func (f *FrenoIman) Aplicar(o *ObjetivosIman, mano gestures.PuntoPanel) gestures.PuntoPanel {
	if o == nil || o.ancho <= 0 || o.alto <= 0 {
		f.Reiniciar()
		return mano
	}
	if !f.hay {
		f.salida, f.anterior, f.hay = mano, mano, true
		return mano
	}
	dx, dy := mano.X-f.anterior.X, mano.Y-f.anterior.Y
	f.anterior = mano
	if o.sobreChico(f.salida) {
		f.salida.X += dx * GananciaFrenoIman
		f.salida.Y += dy * GananciaFrenoIman
	} else {
		f.salida.X += dx + (mano.X-f.salida.X-dx)*RecuperarFrenoIman
		f.salida.Y += dy + (mano.Y-f.salida.Y-dy)*RecuperarFrenoIman
	}
	// Tope: a RadioIman lógicos de la mano.
	ex, ey := (f.salida.X-mano.X)*o.ancho, (f.salida.Y-mano.Y)*o.alto
	if d := math.Hypot(ex, ey); d > RadioIman {
		k := RadioIman / d
		f.salida.X = mano.X + ex*k/o.ancho
		f.salida.Y = mano.Y + ey*k/o.alto
	}
	return f.salida
}

// Reiniciar olvida el desfase: el puntero vuelve a ir con la mano.
func (f *FrenoIman) Reiniciar() { f.hay = false }

// EstadoIman es lo que la app muestra y mide del imán.
type EstadoIman struct {
	Encendido  bool    `json:"encendido"`
	Disponible bool    `json:"disponible"` // hay TargetFinder en este sistema
	Vigente    bool    `json:"vigente"`
	Objetivos  int     `json:"objetivos"`
	Chicos     int     `json:"chicos"`
	Lecturas   int     `json:"lecturas"`
	MsLectura  float64 `json:"ms_lectura"` // media de las lecturas
	Fallos     int     `json:"fallos"`
	Ajustes    int     `json:"ajustes"`
	Dudas      int     `json:"dudas"`
	Error      string  `json:"error,omitempty"`
}

// Iman guarda la lista de objetivos de la ventana con el foco. La lee
// cuando cambia la ventana y cada LeerImanCada mientras el panel la pide,
// siempre fuera del camino del cuadro (Correr, en su gorutina). El panel
// solo consulta lo ya leído.
type Iman struct {
	mu       sync.Mutex
	Buscador platform.TargetFinder

	apagado  bool
	pedido   bool   // el panel está encendido con la mano dentro
	ventana  uint64 // última ventana con el foco que se vio
	actual   *ObjetivosIman
	cambio   time.Time // último dedo levantado: lo leído antes no vale
	consulta time.Time

	lecturas, fallos, ajustes, dudas int
	msTotal                          float64
	err                              string
}

// NuevoIman prepara el imán sobre b (que puede ser nil: sin imán).
// Arranca encendido; Encender(false) es el interruptor.
func NuevoIman(b platform.TargetFinder) *Iman { return &Iman{Buscador: b} }

// Encender prende o apaga el imán. Apagado no lee nada ni corrige nada.
func (m *Iman) Encender(on bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apagado = !on
	if !on {
		m.actual = nil
	}
}

// Encendido dice si el imán está prendido y tiene con qué leer.
func (m *Iman) Encendido() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.apagado && m.Buscador != nil
}

// Pedir dice si hace falta la lista: con el panel apagado o la mano fuera
// no se lee nada (no gasta CPU).
func (m *Iman) Pedir(si bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.pedido = si
	m.mu.Unlock()
}

// Cambio avisa que se levantó un dedo: la pantalla pudo cambiar.
func (m *Iman) Cambio(ahora time.Time) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.cambio = ahora
	m.mu.Unlock()
}

// Objetivos devuelve la lista vigente o nil: la de la ventana que tiene el
// foco, leída después del último toque y hace menos de VigenciaIman.
func (m *Iman) Objetivos(ahora time.Time) *ObjetivosIman {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vigente(ahora)
}

func (m *Iman) vigente(ahora time.Time) *ObjetivosIman {
	o := m.actual
	if m.apagado || o == nil || o.Ventana != m.ventana || !o.Leido.After(m.cambio) || ahora.Sub(o.Leido) > VigenciaIman {
		return nil
	}
	return o
}

// Anotar cuenta el resultado de un ajuste para el estado.
func (m *Iman) Anotar(r ResultadoAjuste) {
	if m == nil {
		return
	}
	m.mu.Lock()
	switch r {
	case AjusteMovido, AjusteRepetir:
		m.ajustes++
	case AjusteDuda:
		m.dudas++
	}
	m.mu.Unlock()
}

// Paso hace una vuelta de la caché: mira la ventana con el foco y, si hace
// falta, vuelve a leer sus objetivos. Correr lo llama cada
// ConsultarVentanaCada; las pruebas lo llaman con el reloj que quieran.
func (m *Iman) Paso(ctx context.Context, ahora time.Time) {
	m.mu.Lock()
	b := m.Buscador
	if b == nil || m.apagado || !m.pedido {
		m.mu.Unlock()
		return
	}
	cambio := m.cambio
	m.mu.Unlock()

	v, err := b.FocusedWindow(ctx)
	if err != nil {
		m.mu.Lock()
		m.ventana, m.err = 0, err.Error()
		m.mu.Unlock()
		return
	}
	m.mu.Lock()
	m.ventana = v
	a := m.actual
	hace := a == nil || a.Ventana != v || ahora.Sub(a.Leido) >= LeerImanCada || !a.Leido.After(cambio)
	if hace && ahora.Sub(cambio) < EsperaTrasToqueIman {
		hace = false // la página todavía se mueve
	}
	m.mu.Unlock()
	if !hace {
		return
	}

	inicio := time.Now()
	s, err := b.Targets(ctx)
	ms := float64(time.Since(inicio).Microseconds()) / 1000
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lecturas++
	m.msTotal += ms
	if err != nil {
		m.fallos++
		m.err = err.Error()
		m.actual = nil
		return
	}
	m.err = ""
	if m.apagado || s.Window != m.ventana {
		return // se apagó o cambió el foco mientras leía
	}
	o := NuevosObjetivosIman(s)
	o.Leido = ahora
	m.actual = o
}

// Correr mantiene la caché hasta que ctx termine.
func (m *Iman) Correr(ctx context.Context) {
	if m == nil || m.Buscador == nil {
		return
	}
	t := time.NewTicker(ConsultarVentanaCada)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ahora := <-t.C:
			c, cancel := context.WithTimeout(ctx, time.Second)
			m.Paso(c, ahora)
			cancel()
		}
	}
}

// Estado resume el imán para la ventana y la línea de medición.
func (m *Iman) Estado(ahora time.Time) EstadoIman {
	if m == nil {
		return EstadoIman{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e := EstadoIman{Encendido: !m.apagado, Disponible: m.Buscador != nil, Lecturas: m.lecturas,
		Fallos: m.fallos, Ajustes: m.ajustes, Dudas: m.dudas, Error: m.err}
	if m.lecturas > 0 {
		e.MsLectura = m.msTotal / float64(m.lecturas)
	}
	if o := m.vigente(ahora); o != nil {
		e.Vigente, e.Objetivos, e.Chicos = true, o.CantObjetivo, o.CantChicos
	}
	return e
}
