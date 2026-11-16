package desktop

import (
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
)

func TestIndicadorArrancaInactivoYSinDestello(t *testing.T) {
	var i Indicador
	v := i.Vista(time.Now())
	if v.Activo || v.Destello || v.Gesto != "" || v.Direccion != "" || v.Secuencia != 0 {
		t.Fatalf("indicador inicial = %+v; quería inactivo y sin destello", v)
	}
}

func TestIndicadorSigueLaLYNoDestellaAlApagar(t *testing.T) {
	var i Indicador
	t0 := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilModo, Activo: true}, t0)
	if v := i.Vista(t0); !v.Activo || v.Destello {
		t.Fatalf("tras la L = %+v; quería activo sin destello", v)
	}
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilScroll, Direccion: gestures.TactilArriba, Activo: true}, t0.Add(time.Second))
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilModo, Activo: false}, t0.Add(2*time.Second))
	v := i.Vista(t0.Add(2 * time.Second))
	if v.Activo || v.Destello || v.Gesto != "" {
		t.Fatalf("tras apagar con la L = %+v; quería inactivo y sin destello", v)
	}
}

func TestIndicadorDestellaScrollZoomYBarrido(t *testing.T) {
	casos := []struct {
		nombre string
		ev     gestures.EventoTactil
		gesto  string
		dir    string
	}{
		{"scroll arriba", gestures.EventoTactil{Accion: gestures.TactilScroll, Direccion: gestures.TactilArriba, Activo: true}, "scroll", "arriba"},
		{"scroll abajo", gestures.EventoTactil{Accion: gestures.TactilScroll, Direccion: gestures.TactilAbajo, Activo: true}, "scroll", "abajo"},
		{"zoom abrir", gestures.EventoTactil{Accion: gestures.TactilZoom, Direccion: gestures.TactilAbrir, Activo: true}, "zoom", "abrir"},
		{"zoom cerrar", gestures.EventoTactil{Accion: gestures.TactilZoom, Direccion: gestures.TactilCerrar, Activo: true}, "zoom", "cerrar"},
		{"barrido izquierda", gestures.EventoTactil{Accion: gestures.TactilBarrido, Direccion: gestures.TactilIzquierda, Activo: true}, "barrido", "izquierda"},
		{"barrido derecha", gestures.EventoTactil{Accion: gestures.TactilBarrido, Direccion: gestures.TactilDerecha, Activo: true}, "barrido", "derecha"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			var i Indicador
			t0 := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
			i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilModo, Activo: true}, t0)
			i.Aplicar(c.ev, t0.Add(100*time.Millisecond))
			v := i.Vista(t0.Add(200 * time.Millisecond))
			if !v.Activo || !v.Destello || v.Gesto != c.gesto || v.Direccion != c.dir {
				t.Fatalf("vista = %+v; quería activo, destello de %s %s", v, c.gesto, c.dir)
			}
			if v.Secuencia != 1 {
				t.Fatalf("secuencia = %d; quería 1", v.Secuencia)
			}
		})
	}
}

func TestIndicadorElDestelloDuraPocoYLaSecuenciaAvanza(t *testing.T) {
	var i Indicador
	t0 := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilModo, Activo: true}, t0)
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilScroll, Direccion: gestures.TactilArriba, Activo: true}, t0)
	if v := i.Vista(t0.Add(DuracionDestello - time.Millisecond)); !v.Destello {
		t.Fatalf("a %.0f ms todavía debía destellar: %+v", DuracionDestello.Seconds()*1000-1, v)
	}
	if v := i.Vista(t0.Add(DuracionDestello)); v.Destello {
		t.Fatalf("a los %.0f ms ya no debía destellar: %+v", DuracionDestello.Seconds()*1000, v)
	}
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilZoom, Direccion: gestures.TactilAbrir, Activo: true}, t0.Add(2*time.Second))
	if v := i.Vista(t0.Add(2 * time.Second)); !v.Destello || v.Secuencia != 2 || v.Gesto != "zoom" {
		t.Fatalf("segundo gesto = %+v; quería zoom con secuencia 2", v)
	}
}

func TestIndicadorIgnoraMovimientoQueNoEstaActivo(t *testing.T) {
	var i Indicador
	t0 := time.Now()
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilScroll, Direccion: gestures.TactilArriba, Activo: false}, t0)
	if v := i.Vista(t0); v.Destello || v.Secuencia != 0 {
		t.Fatalf("vista = %+v; un evento inactivo no debía destellar", v)
	}
}

func TestIndicadorApagarLimpiaTodo(t *testing.T) {
	var i Indicador
	t0 := time.Now()
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilModo, Activo: true}, t0)
	i.Aplicar(gestures.EventoTactil{Accion: gestures.TactilBarrido, Direccion: gestures.TactilDerecha, Activo: true}, t0)
	i.Apagar()
	if v := i.Vista(t0); v.Activo || v.Destello || v.Gesto != "" || v.Direccion != "" {
		t.Fatalf("tras Apagar = %+v", v)
	}
}
