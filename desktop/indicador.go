package desktop

import (
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
)

// DuracionDestello es lo que el indicador mantiene a la vista el último gesto
// táctil. Es corto a propósito: en clase solo tiene que verse de reojo.
const DuracionDestello = 700 * time.Millisecond

// VistaIndicador es lo que pinta el indicador pequeño de la esquina: si el
// modo táctil está activo y, durante un destello breve, el gesto reconocido.
type VistaIndicador struct {
	Activo    bool   `json:"activo"`
	Gesto     string `json:"gesto,omitempty"`     // scroll | zoom | barrido
	Direccion string `json:"direccion,omitempty"` // arriba, abajo, abrir, cerrar, izquierda, derecha
	Destello  bool   `json:"destello"`
	Secuencia uint64 `json:"secuencia"` // cambia con cada gesto: la página reinicia la animación
}

// Indicador lleva el estado del modo táctil que se ve en la esquina. No
// guarda imágenes ni cuadros: solo el último gesto y cuándo ocurrió.
type Indicador struct {
	mu        sync.Mutex
	activo    bool
	gesto     string
	direccion string
	cuando    time.Time
	secuencia uint64
}

// Aplicar incorpora un evento del clasificador táctil. La L cambia el modo;
// scroll, zoom y barrido solo hacen destellar el gesto (el modo lo enciende
// y lo apaga la L, no cada movimiento).
func (i *Indicador) Aplicar(ev gestures.EventoTactil, ahora time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	switch ev.Accion {
	case gestures.TactilModo:
		i.activo = ev.Activo
		i.gesto, i.direccion, i.cuando = "", "", time.Time{}
	case gestures.TactilScroll, gestures.TactilZoom, gestures.TactilBarrido:
		if !ev.Activo {
			return
		}
		i.activo = true
		i.gesto = ev.Accion.String()
		i.direccion = ""
		if ev.Direccion != gestures.TactilSinDireccion {
			i.direccion = ev.Direccion.String()
		}
		i.cuando = ahora
		i.secuencia++
	}
}

// Vista devuelve el estado que la página puede pintar ahora mismo.
func (i *Indicador) Vista(ahora time.Time) VistaIndicador {
	i.mu.Lock()
	defer i.mu.Unlock()
	v := VistaIndicador{Activo: i.activo, Gesto: i.gesto, Direccion: i.direccion, Secuencia: i.secuencia}
	if !i.cuando.IsZero() && ahora.Before(i.cuando.Add(DuracionDestello)) {
		v.Destello = true
	}
	return v
}

// Apagar deja el indicador inactivo y sin destello (cámara apagada).
func (i *Indicador) Apagar() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.activo = false
	i.gesto, i.direccion, i.cuando = "", "", time.Time{}
}
