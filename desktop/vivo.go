package desktop

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// cuadrosEstable: la pose tiene que repetirse esto para anotarla (sin esto,
// el titileo de un cuadro a otro llena el registro).
const cuadrosEstable = 3

// Vista es lo que la ventana muestra después de cada cuadro.
type Vista struct {
	Pose       string `json:"pose"`
	Modo       string `json:"modo"`
	Gesto      string `json:"gesto,omitempty"` // el de este cuadro, si hubo
	Direccion  string `json:"direccion,omitempty"`
	Ultimo     string `json:"ultimo"`      // último gesto reconocido, con su dirección
	UltimoHora string `json:"ultimo_hora"` // hh:mm:ss
	Activacion int    `json:"activacion"`  // número de la última activación
	Prueba     bool   `json:"prueba"`
}

// Disparo es un gesto que el núcleo tiene que convertir en Órdenes.
type Disparo struct {
	Evento     gestures.Evento
	Activacion int
	Prueba     bool // en la prueba de trabajo normal: no se manda nada
	Sesion     string
}

// Vivo sigue la cámara cuadro a cuadro: pose, modo, gestos y cada
// activación hasta que termina. Anota en Reg y le pasa a AlGesto los gestos
// que mueven algo (agarrar, soltar, mandar app).
type Vivo struct {
	Reg     *Registro
	AlGesto func(Disparo) // se llama sin el candado tomado; no debe bloquear
	Ajustes gestures.Ajustes

	mu        sync.Mutex
	sesion    string
	enUso     bool
	est       gestures.Estado
	prueba    bool
	cand      gestures.Pose
	nCand     int
	estable   gestures.Pose
	hayPose   bool
	act       int
	abierta   bool
	agarro    bool
	actPrueba bool
	actDesde  time.Time
	latido    time.Time
	cuadros   int
	conMano   int
	ultimo    string
	ultimoT   time.Time
}

func nombreGesto(g lazulv1.Gesto) string { return strings.TrimPrefix(g.String(), "GESTO_") }

func nombreDir(d lazulv1.Direccion) string {
	if d == lazulv1.Direccion_DIRECCION_NINGUNA {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(d.String(), "DIRECCION_"))
}

func (v *Vivo) ajustes() gestures.Ajustes {
	if v.Ajustes == (gestures.Ajustes{}) {
		return gestures.AjustesPorDefecto
	}
	return v.Ajustes
}

// empezar abre una sesión de uso (la cámara empezó a dar cuadros).
func (v *Vivo) empezar(ahora time.Time) {
	v.sesion = ahora.Format("20060102-150405.000")
	v.enUso = true
	v.est = gestures.NuevoEstado(gestures.ModoReposo, v.ajustes())
	v.hayPose, v.nCand, v.act, v.abierta = false, 0, 0, false
	v.latido, v.cuadros, v.conMano = ahora, 0, 0
	v.Reg.Anotar(Linea{T: ahora, Tipo: TipoInicio, Sesion: v.sesion, Prueba: v.prueba})
}

// Parar cierra la sesión de uso (cámara apagada o app cerrada).
func (v *Vivo) Parar(ahora time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.enUso {
		return
	}
	if v.abierta {
		v.cerrar(ahora, ActInterrumpe)
	}
	v.Reg.Anotar(Linea{T: ahora, Tipo: TipoFin, Sesion: v.sesion, Cuadros: v.cuadros, ConMano: v.conMano})
	v.enUso = false
}

func (v *Vivo) cerrar(ahora time.Time, res string) {
	v.Reg.Anotar(Linea{T: ahora, Tipo: TipoActivacion, Sesion: v.sesion, Activacion: v.act, Resultado: res,
		Ms: ahora.Sub(v.actDesde).Milliseconds(), Prueba: v.actPrueba})
	v.abierta, v.agarro = false, false
}

// Cuadro pasa un cuadro por el clasificador. ahora es la hora de pared (para
// el registro); c.TMs, el reloj de la cámara (para el clasificador).
func (v *Vivo) Cuadro(c gestures.Cuadro, ahora time.Time) Vista {
	v.mu.Lock()
	if !v.enUso {
		v.empezar(ahora)
	}
	v.cuadros++
	pose := gestures.PoseNinguna
	if len(c.Manos) > 0 {
		v.conMano++
		pose = gestures.PoseDe(c.Manos[0], c.Aspecto)
	}
	antes := v.est.Modo()
	var ev gestures.Evento
	v.est, ev = gestures.Clasificar(v.est, c)
	modo := v.est.Modo()

	// La pose estable, solo cuando cambia.
	if pose == v.cand {
		v.nCand++
	} else {
		v.cand, v.nCand = pose, 1
	}
	if v.nCand == cuadrosEstable && (!v.hayPose || v.estable != pose) {
		v.estable, v.hayPose = pose, true
		v.Reg.Anotar(Linea{T: ahora, Tipo: TipoPose, Sesion: v.sesion, Pose: pose.String(), Modo: modo.String()})
	}

	var disparo *Disparo
	if ev.Gesto != lazulv1.Gesto_GESTO_NINGUNO {
		if ev.Gesto == lazulv1.Gesto_GESTO_ACTIVAR {
			v.act++
			v.abierta, v.agarro, v.actPrueba, v.actDesde = true, false, v.prueba, ahora
		}
		v.Reg.Anotar(Linea{T: ahora, Tipo: TipoGesto, Sesion: v.sesion, Gesto: nombreGesto(ev.Gesto), Direccion: nombreDir(ev.Direccion),
			Confianza: ev.Confianza, Pose: pose.String(), Modo: modo.String(), Activacion: v.act, Prueba: v.actPrueba})
		v.ultimo, v.ultimoT = nombreGesto(ev.Gesto), ahora
		if d := nombreDir(ev.Direccion); d != "" {
			v.ultimo += " → " + d
		}
		switch ev.Gesto {
		case lazulv1.Gesto_GESTO_AGARRAR:
			v.agarro = true
		case lazulv1.Gesto_GESTO_SOLTAR:
			if v.abierta {
				v.cerrar(ahora, ActCaptura)
			}
		case lazulv1.Gesto_GESTO_MANDAR_APP:
			if v.abierta {
				v.cerrar(ahora, ActGesto4)
			}
		}
		if ev.Gesto != lazulv1.Gesto_GESTO_ACTIVAR {
			disparo = &Disparo{Evento: ev, Activacion: v.act, Prueba: v.actPrueba, Sesion: v.sesion}
		}
	}
	// Volvió a reposo sin terminar en un gesto: venció o se canceló.
	if v.abierta && modo == gestures.ModoReposo && antes != gestures.ModoReposo {
		if v.agarro {
			v.cerrar(ahora, ActCancelada)
		} else {
			v.cerrar(ahora, ActSinGesto)
		}
	}
	if ahora.Sub(v.latido) >= LatidoCada {
		v.Reg.Anotar(Linea{T: ahora, Tipo: TipoLatido, Sesion: v.sesion, Cuadros: v.cuadros, ConMano: v.conMano, Prueba: v.prueba})
		v.latido, v.cuadros, v.conMano = ahora, 0, 0
	}
	vista := Vista{Pose: pose.String(), Modo: modo.String(), Ultimo: v.ultimo, Activacion: v.act, Prueba: v.prueba}
	if !v.ultimoT.IsZero() {
		vista.UltimoHora = v.ultimoT.Format("15:04:05")
	}
	if ev.Gesto != lazulv1.Gesto_GESTO_NINGUNO {
		vista.Gesto, vista.Direccion = nombreGesto(ev.Gesto), nombreDir(ev.Direccion)
	}
	al := v.AlGesto
	v.mu.Unlock()
	if disparo != nil && al != nil {
		al(*disparo)
	}
	return vista
}

// Prueba empieza o termina la prueba de trabajo normal: mientras dure, toda
// activación cuenta como falsa y no se manda nada a ningún equipo.
func (v *Vivo) Prueba(on bool, ahora time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.prueba == on {
		return
	}
	v.prueba = on
	res := "termina"
	if on {
		res = "empieza"
	}
	v.Reg.Anotar(Linea{T: ahora, Tipo: TipoPrueba, Sesion: v.sesion, Resultado: res, Prueba: on})
}

// EnPrueba dice si está en la prueba de trabajo normal.
func (v *Vivo) EnPrueba() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.prueba
}

// Marcar anota que la última activación fue sin querer (falsa) o a
// propósito. Devuelve su número (0: no hubo ninguna en esta sesión).
func (v *Vivo) Marcar(falsa bool, ahora time.Time) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.act == 0 {
		return 0, fmt.Errorf("todavía no hubo ninguna activación con la cámara encendida")
	}
	v.Reg.Anotar(Linea{T: ahora, Tipo: TipoMarca, Sesion: v.sesion, Activacion: v.act, Falsa: &falsa})
	return v.act, nil
}

// Sesion de uso actual ("" si la cámara no da cuadros).
func (v *Vivo) Sesion() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.enUso {
		return ""
	}
	return v.sesion
}
