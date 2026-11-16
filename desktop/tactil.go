package desktop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

// TactilApagarSinMano evita que el modo quede armado cuando la persona se
// aparta. La cámara sigue dando cuadros vacíos, así que no hace falta un
// temporizador ni una gorrutina aparte.
const TactilApagarSinMano = 3 * time.Second

// Tactil conecta el clasificador táctil con la ventana que tiene el foco.
// También conserva una línea por gesto para poder revisar activaciones falsas.
type Tactil struct {
	Reg        *Registro
	Acciones   platform.TactileActions
	Ajustes    gestures.AjustesTactiles
	SinMano    time.Duration
	Navegacion platform.NavigationMode

	mu sync.Mutex

	sesion        string
	enUso         bool
	est           gestures.EstadoTactil
	prueba        bool
	act           int
	abierta       bool
	huboAccion    bool
	actPrueba     bool
	actDesde      time.Time
	ultimaManoMs  int64
	hayUltimaMano bool
	latido        time.Time
	cuadros       int
	conMano       int
	ultimo        string
	ultimoT       time.Time
	ultimoError   string
}

func (t *Tactil) ajustes() gestures.AjustesTactiles {
	if t.Ajustes == (gestures.AjustesTactiles{}) {
		return gestures.AjustesTactilesPorDefecto
	}
	return t.Ajustes
}

func (t *Tactil) sinMano() time.Duration {
	if t.SinMano <= 0 {
		return TactilApagarSinMano
	}
	return t.SinMano
}

func (t *Tactil) navegacion() platform.NavigationMode {
	if t.Navegacion == 0 {
		return platform.NavigationSlides
	}
	return t.Navegacion
}

func (t *Tactil) empezar(ahora time.Time) {
	t.sesion = ahora.Format("20060102-150405.000")
	t.enUso = true
	t.est = gestures.NuevoEstadoTactil(false, t.ajustes())
	t.act, t.abierta, t.huboAccion = 0, false, false
	t.hayUltimaMano = false
	t.latido, t.cuadros, t.conMano = ahora, 0, 0
	t.Reg.Anotar(Linea{T: ahora, Tipo: TipoInicio, Sesion: t.sesion, Prueba: t.prueba})
}

func nombreAccionTactil(a gestures.AccionTactil) string {
	return strings.ToUpper(a.String())
}

func nombreDireccionTactil(d gestures.DireccionTactil) string {
	if d == gestures.TactilSinDireccion {
		return ""
	}
	return d.String()
}

func (t *Tactil) cerrar(ahora time.Time, resultado string) {
	if !t.abierta {
		return
	}
	if resultado == "" {
		resultado = ActSinGesto
		if t.huboAccion {
			resultado = ActTactil
		}
	}
	t.Reg.Anotar(Linea{T: ahora, Tipo: TipoActivacion, Sesion: t.sesion, Activacion: t.act,
		Resultado: resultado, Ms: ahora.Sub(t.actDesde).Milliseconds(), Prueba: t.actPrueba})
	t.abierta, t.huboAccion = false, false
}

func (t *Tactil) ejecutar(ev gestures.EventoTactil) error {
	if t.Acciones == nil {
		return platform.ErrNoImplementado
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	switch ev.Accion {
	case gestures.TactilScroll:
		switch ev.Direccion {
		case gestures.TactilArriba:
			return t.Acciones.Scroll(ctx, platform.ScrollUp)
		case gestures.TactilAbajo:
			return t.Acciones.Scroll(ctx, platform.ScrollDown)
		}
	case gestures.TactilZoom:
		switch ev.Direccion {
		case gestures.TactilAbrir:
			return t.Acciones.Zoom(ctx, platform.ZoomIn)
		case gestures.TactilCerrar:
			return t.Acciones.Zoom(ctx, platform.ZoomOut)
		}
	case gestures.TactilBarrido:
		switch ev.Direccion {
		case gestures.TactilIzquierda:
			return t.Acciones.Navigate(ctx, platform.NavigateLeft, t.navegacion())
		case gestures.TactilDerecha:
			return t.Acciones.Navigate(ctx, platform.NavigateRight, t.navegacion())
		}
	}
	return platform.ErrAccionInvalida
}

func (t *Tactil) registrar(ev gestures.EventoTactil, pose gestures.Pose, ahora time.Time, resultado string, err error) {
	g, d := nombreAccionTactil(ev.Accion), nombreDireccionTactil(ev.Direccion)
	modo := "inactivo"
	if ev.Activo {
		modo = "activo"
	}
	l := Linea{T: ahora, Tipo: TipoGesto, Sesion: t.sesion, Gesto: g, Direccion: d,
		Confianza: ev.Confianza, Pose: pose.String(), Modo: modo, Activacion: t.act, Resultado: resultado, Prueba: t.actPrueba}
	if err != nil {
		l.Error = ClaseError(err)
		t.ultimoError = err.Error()
	} else {
		t.ultimoError = ""
	}
	t.Reg.Anotar(l)
	t.ultimo, t.ultimoT = g, ahora
	if d != "" {
		t.ultimo += " → " + d
	}
}

func (t *Tactil) latir(ahora time.Time) {
	if ahora.Sub(t.latido) < LatidoCada {
		return
	}
	t.Reg.Anotar(Linea{T: ahora, Tipo: TipoLatido, Sesion: t.sesion, Cuadros: t.cuadros,
		ConMano: t.conMano, Prueba: t.prueba})
	t.latido, t.cuadros, t.conMano = ahora, 0, 0
}

func (t *Tactil) vista(c gestures.Cuadro, pose gestures.Pose) Vista {
	activo := t.est.Activo()
	modo := "inactivo"
	if activo {
		modo = "activo"
	}
	v := Vista{Pose: pose.String(), Modo: modo, Ultimo: t.ultimo, Activacion: t.act,
		Prueba: t.prueba, Error: t.ultimoError}
	if !t.ultimoT.IsZero() {
		v.UltimoHora = t.ultimoT.Format("15:04:05")
	}
	if activo && t.hayUltimaMano {
		v.SinMano = len(c.Manos) == 0
		pasado := time.Duration(c.TMs-t.ultimaManoMs) * time.Millisecond
		v.ApagaEnMs = max(0, (t.sinMano() - pasado).Milliseconds())
	}
	return v
}

// Cuadro clasifica un cuadro, ejecuta la acción local y devuelve el estado
// visible. ahora es la hora de pared para el registro; c.TMs es el reloj
// monotónico de la cámara usado por el clasificador y el apagado sin mano.
func (t *Tactil) Cuadro(c gestures.Cuadro, ahora time.Time) Vista {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.enUso {
		t.empezar(ahora)
	}
	t.cuadros++
	pose := gestures.PoseNinguna
	if len(c.Manos) > 0 {
		t.conMano++
		pose = gestures.PoseDe(c.Manos[0], c.Aspecto)
		if t.est.Activo() {
			t.ultimaManoMs, t.hayUltimaMano = c.TMs, true
		}
	}

	// Si desaparece la mano, el modo se desarma aunque nunca vuelva a verse
	// la L. Se registra como un cambio de modo para que quede auditable.
	if t.est.Activo() && t.hayUltimaMano && len(c.Manos) == 0 &&
		time.Duration(c.TMs-t.ultimaManoMs)*time.Millisecond >= t.sinMano() {
		t.est = gestures.NuevoEstadoTactil(false, t.ajustes())
		ev := gestures.EventoTactil{Accion: gestures.TactilModo, Activo: false, CuadroMs: c.TMs}
		t.registrar(ev, pose, ahora, "sin_mano", nil)
		t.cerrar(ahora, "")
		t.hayUltimaMano = false
		t.latir(ahora)
		v := t.vista(c, pose)
		v.Gesto = nombreAccionTactil(ev.Accion)
		return v
	}

	var ev gestures.EventoTactil
	t.est, ev = gestures.ClasificarTactil(t.est, c)
	if ev.Accion != gestures.TactilNinguno {
		if ev.Accion == gestures.TactilModo && ev.Activo {
			t.act++
			t.abierta, t.huboAccion, t.actPrueba, t.actDesde = true, false, t.prueba, ahora
			t.ultimaManoMs, t.hayUltimaMano = c.TMs, len(c.Manos) > 0
		}
		resultado := "ok"
		var err error
		if ev.Accion != gestures.TactilModo {
			t.huboAccion = true
			if t.prueba {
				resultado = "no_ejecutada_prueba"
			} else if err = t.ejecutar(ev); err != nil {
				resultado = "falla"
			}
		}
		t.registrar(ev, pose, ahora, resultado, err)
		if ev.Accion == gestures.TactilModo && !ev.Activo {
			t.cerrar(ahora, "")
			t.hayUltimaMano = false
		}
	}
	t.latir(ahora)
	v := t.vista(c, pose)
	if ev.Accion != gestures.TactilNinguno {
		v.Gesto, v.Direccion = nombreAccionTactil(ev.Accion), nombreDireccionTactil(ev.Direccion)
	}
	return v
}

// Parar apaga el modo y cierra la sesión de cámara.
func (t *Tactil) Parar(ahora time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.enUso {
		return
	}
	if t.abierta {
		resultado := ActInterrumpe
		if t.huboAccion {
			resultado = ActTactil
		}
		t.cerrar(ahora, resultado)
	}
	t.Reg.Anotar(Linea{T: ahora, Tipo: TipoFin, Sesion: t.sesion, Cuadros: t.cuadros, ConMano: t.conMano})
	t.enUso = false
	t.est = gestures.NuevoEstadoTactil(false, t.ajustes())
	t.hayUltimaMano = false
}

// Prueba activa el trabajo normal: se reconocen y registran gestos, pero no
// se mandan teclas a la ventana con foco.
func (t *Tactil) Prueba(on bool, ahora time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.prueba == on {
		return
	}
	t.prueba = on
	resultado := "termina"
	if on {
		resultado = "empieza"
	}
	t.Reg.Anotar(Linea{T: ahora, Tipo: TipoPrueba, Sesion: t.sesion, Resultado: resultado, Prueba: on})
}

func (t *Tactil) EnPrueba() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.prueba
}

func (t *Tactil) Marcar(falsa bool, ahora time.Time) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.act == 0 {
		return 0, errors.New("todavía no hubo ninguna activación con la cámara encendida")
	}
	t.Reg.Anotar(Linea{T: ahora, Tipo: TipoMarca, Sesion: t.sesion, Activacion: t.act, Falsa: &falsa})
	return t.act, nil
}
