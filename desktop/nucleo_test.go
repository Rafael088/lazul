package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

type equipo struct {
	n   *Nucleo
	sis *falso.Falso
	reg string
}

// nuevoEquipo arranca un Nucleo con un sistema en memoria en 127.0.0.1.
func nuevoEquipo(t *testing.T, ctx context.Context, nombre string, permiso func(context.Context, core.Solicitud) bool) equipo {
	t.Helper()
	dir := t.TempDir()
	ruta := filepath.Join(dir, "registro.jsonl")
	reg, err := AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Cerrar() })
	sis := falso.New(nombre)
	n, err := NuevoNucleo(Opciones{Dir: dir, Nombre: nombre, Escucha: "127.0.0.1:0", Sistema: sis.Sistema(), Reg: reg, Permiso: permiso, Adelantar: true})
	if err != nil {
		t.Fatal(err)
	}
	n.Arrancar(ctx)
	if n.Dir() == "" {
		t.Fatal(n.Estado().Error)
	}
	return equipo{n, sis, ruta}
}

func esperar(t *testing.T, que string, f func() bool) {
	t.Helper()
	for range 200 {
		if f() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("no pasó: %s", que)
}

func gesto(g lazulv1.Gesto, d lazulv1.Direccion) Disparo {
	return Disparo{Evento: gestures.Evento{Gesto: g, Direccion: d, Confianza: 1}, Activacion: 1, Sesion: "s"}
}

func TestEmparejarConPermisoYSoltarCaptura(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var vistas []core.Solicitud
	win := nuevoEquipo(t, ctx, "PC Windows", func(_ context.Context, s core.Solicitud) bool { vistas = append(vistas, s); return true })
	lin := nuevoEquipo(t, ctx, "PC Linux", nil)

	// Sin emparejar no hay a quién mandar.
	lin.n.Disparar(gesto(lazulv1.Gesto_GESTO_SOLTAR, lazulv1.Direccion_DIRECCION_DERECHA))
	esperar(t, "aviso sin destino", func() bool { return lin.n.Estado().Accion != "" })

	codigo, err := win.n.AbrirCodigo()
	if err != nil {
		t.Fatal(err)
	}
	if win.n.Estado().Codigo != codigo {
		t.Error("el receptor no muestra el código")
	}
	if err := lin.n.Emparejar(ctx, win.n.Dir(), codigo, ""); err != nil {
		t.Fatal(err)
	}
	if len(vistas) != 1 || vistas[0].Nombre != "PC Linux" || vistas[0].Huella.Corta() != lin.n.Estado().Huella {
		t.Fatalf("la persona tenía que ver nombre y huella del controlador: %+v", vistas)
	}
	if win.n.Estado().Codigo != "" {
		t.Error("el código es de un solo uso")
	}
	e := lin.n.Estado()
	if e.Destino != "PC Windows" || len(e.Equipos) != 1 || e.Equipos[0].Rol != "le mando" {
		t.Fatalf("estado del controlador: %+v", e)
	}
	if eq := win.n.Estado().Equipos; len(eq) != 1 || eq[0].Rol != "me manda" {
		t.Fatalf("equipos del receptor: %+v", eq)
	}

	lin.n.Disparar(gesto(lazulv1.Gesto_GESTO_AGARRAR, 0))
	lin.n.Disparar(gesto(lazulv1.Gesto_GESTO_SOLTAR, lazulv1.Direccion_DIRECCION_DERECHA))
	esperar(t, "la captura se abre en el otro", func() bool { return len(win.sis.Abiertas()) == 1 })
	if a := win.sis.Abiertas()[0]; !strings.HasSuffix(a.Archivo, ".png") {
		t.Errorf("abrió %+v, quería la captura", a)
	}
	esperar(t, "el controlador lo cuenta", func() bool { return strings.Contains(lin.n.Estado().Accion, "soltada") })
	if !lin.n.Estado().Conectado {
		t.Error("debería verse conectado")
	}
	esperar(t, "el receptor lo cuenta", func() bool { return win.n.Estado().Recibido != "" })

	// Lo anotado: acciones ok con destino y tipo; en el receptor, lo recibido.
	ls := lineas(t, lin.reg)
	acc := deTipo(ls, TipoAccion)
	if len(acc) != 3 || acc[0].Error != "sin_destino" || acc[1].Resultado != "ok" || acc[2].Resultado != "ok" || acc[2].Que != "captura" || acc[2].Destino != "PC Windows" {
		t.Errorf("acciones %+v", acc)
	}
	if r := deTipo(lineas(t, win.reg), TipoRecibido); len(r) != 1 || r[0].Que != "archivo" || r[0].Resultado != "ok" {
		t.Errorf("recibidos %+v", r)
	}

	// Con la app vuelta a abrir, el destino sigue emparejado.
	n2, err := NuevoNucleo(Opciones{Dir: lin.n.op.Dir, Nombre: "PC Linux", Escucha: "127.0.0.1:0", Sistema: lin.sis.Sistema()})
	if err != nil {
		t.Fatal(err)
	}
	if n2.Estado().Destino != "PC Windows" || n2.Huella() != lin.n.Huella() {
		t.Error("no recordó el destino o la identidad")
	}
}

func TestPersonaRechazaNoEmpareja(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	win := nuevoEquipo(t, ctx, "PC Windows", func(context.Context, core.Solicitud) bool { return false })
	lin := nuevoEquipo(t, ctx, "PC Linux", nil)
	codigo, _ := win.n.AbrirCodigo()
	if err := lin.n.Emparejar(ctx, win.n.Dir(), codigo, ""); !errors.Is(err, core.ErrRechazado) {
		t.Fatalf("err %v, quería rechazado", err)
	}
	if len(lin.n.Estado().Equipos) != 0 || len(win.n.Estado().Equipos) != 0 {
		t.Error("nadie debería quedar emparejado")
	}
}

func TestCodigoErroneoNoEmpareja(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	win := nuevoEquipo(t, ctx, "PC Windows", func(context.Context, core.Solicitud) bool { return true })
	lin := nuevoEquipo(t, ctx, "PC Linux", nil)
	codigo, _ := win.n.AbrirCodigo()
	malo := "000000"
	if codigo == malo {
		malo = "111111"
	}
	if err := lin.n.Emparejar(ctx, win.n.Dir(), malo, ""); !errors.Is(err, core.ErrCodigoErroneo) {
		t.Fatalf("err %v, quería código erróneo", err)
	}
	// La huella anunciada (mDNS) tiene que ser la del receptor.
	otra := strings.Repeat("ab", 32)
	if err := lin.n.Emparejar(ctx, win.n.Dir(), codigo, otra); err == nil {
		t.Fatal("emparejó con una huella anunciada que no es la del receptor")
	}
}

func TestEnPruebaNoMandaNada(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	win := nuevoEquipo(t, ctx, "PC Windows", func(context.Context, core.Solicitud) bool { return true })
	lin := nuevoEquipo(t, ctx, "PC Linux", nil)
	codigo, _ := win.n.AbrirCodigo()
	if err := lin.n.Emparejar(ctx, win.n.Dir(), codigo, ""); err != nil {
		t.Fatal(err)
	}
	for _, g := range []lazulv1.Gesto{lazulv1.Gesto_GESTO_AGARRAR, lazulv1.Gesto_GESTO_SOLTAR} {
		d := gesto(g, lazulv1.Direccion_DIRECCION_IZQUIERDA)
		d.Prueba = true
		lin.n.Disparar(d)
	}
	esperar(t, "las dos acciones anotadas", func() bool { return len(deTipo(lineas(t, lin.reg), TipoAccion)) == 2 })
	for _, l := range deTipo(lineas(t, lin.reg), TipoAccion) {
		if l.Resultado != "no_mandada_prueba" {
			t.Errorf("acción %+v", l)
		}
	}
	if len(win.sis.Abiertas()) != 0 {
		t.Error("en la prueba no se manda nada")
	}
}

func TestClaseErrorNoLlevaTexto(t *testing.T) {
	if c := ClaseError(errors.New("archivo abierto: open /home/alguien/secreto.pdf: permiso")); c != "otro" {
		t.Errorf("clase %q", c)
	}
	if c := ClaseError(core.ErrNadaAgarrado); c != "nada_agarrado" {
		t.Errorf("clase %q", c)
	}
}
