//go:build linux

package linux

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
	"github.com/godbus/dbus/v5"
)

// estadoFalso es el org.a11y.Bus de un bus de prueba: GetAddress y las
// propiedades org.a11y.Status.
type estadoFalso struct {
	mu        sync.Mutex
	encendida bool
	vecesSet  int
	errGet    *dbus.Error
	errSet    *dbus.Error
}

func (e *estadoFalso) GetAddress() (string, *dbus.Error) { return "", nil }

func (e *estadoFalso) Get(_, prop string) (dbus.Variant, *dbus.Error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.errGet != nil {
		return dbus.Variant{}, e.errGet
	}
	if prop != propA11yEncendida {
		return dbus.Variant{}, dbus.MakeFailedError(errors.New("propiedad desconocida"))
	}
	return dbus.MakeVariant(e.encendida), nil
}

func (e *estadoFalso) Set(_, prop string, v dbus.Variant) *dbus.Error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.errSet != nil {
		return e.errSet
	}
	b, ok := v.Value().(bool)
	if prop != propA11yEncendida || !ok {
		return dbus.MakeFailedError(errors.New("propiedad desconocida"))
	}
	e.encendida, e.vecesSet = b, e.vecesSet+1
	return nil
}

func (e *estadoFalso) estado() (bool, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.encendida, e.vecesSet
}

// busA11yFalso publica estadoFalso en un bus propio y devuelve la capacidad
// apuntando a él.
func busA11yFalso(t *testing.T, e *estadoFalso) accesibilidadSesion {
	t.Helper()
	dir := busPrueba(t)
	c := conectarPrueba(t, dir)
	for _, ifaz := range []string{busA11y, ifazEstadoA11y, "org.freedesktop.DBus.Properties"} {
		if err := c.Export(e, rutaA11y, ifaz); err != nil {
			t.Fatal(err)
		}
	}
	if r, err := c.RequestName(busA11y, dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("RequestName: %v %v", r, err)
	}
	return accesibilidadSesion{conectar: func() (*dbus.Conn, error) {
		return dbus.Connect(dir, dbus.WithAuth(dbus.AuthAnonymous()))
	}}
}

func TestAccesibilidadEnciendeIsEnabled(t *testing.T) {
	e := &estadoFalso{encendida: false}
	a := busA11yFalso(t, e)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := a.Enable(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TurnedOn || res.WasOn {
		t.Errorf("resultado = %+v, quiero TurnedOn", res)
	}
	if encendida, sets := e.estado(); !encendida || sets != 1 {
		t.Errorf("el bus quedó encendida=%v con %d Set, quiero true y 1", encendida, sets)
	}
}

func TestAccesibilidadNoTocaSiYaEstaba(t *testing.T) {
	e := &estadoFalso{encendida: true}
	a := busA11yFalso(t, e)
	res, err := a.Enable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.WasOn || res.TurnedOn {
		t.Errorf("resultado = %+v, quiero WasOn", res)
	}
	if encendida, sets := e.estado(); !encendida || sets != 0 {
		t.Errorf("el bus quedó encendida=%v con %d Set, quiero true sin Set", encendida, sets)
	}
}

func TestAccesibilidadSinBus(t *testing.T) {
	dir := busPrueba(t) // sin nadie que se llame org.a11y.Bus
	a := accesibilidadSesion{conectar: func() (*dbus.Conn, error) {
		return dbus.Connect(dir, dbus.WithAuth(dbus.AuthAnonymous()))
	}}
	res, err := a.Enable(context.Background())
	if !errors.Is(err, platform.ErrSinAccesibilidad) {
		t.Fatalf("err = %v, quiero ErrSinAccesibilidad", err)
	}
	if res.TurnedOn || res.WasOn {
		t.Errorf("resultado = %+v, sin bus no se toca nada", res)
	}
}

func TestAccesibilidadErroresDelBus(t *testing.T) {
	t.Run("leer falla", func(t *testing.T) {
		e := &estadoFalso{encendida: false, errGet: dbus.MakeFailedError(errors.New("sin permiso"))}
		a := busA11yFalso(t, e)
		if _, err := a.Enable(context.Background()); err == nil || errors.Is(err, platform.ErrSinAccesibilidad) {
			t.Fatalf("err = %v, quiero un error que no sea ErrSinAccesibilidad", err)
		}
	})
	t.Run("escribir falla", func(t *testing.T) {
		e := &estadoFalso{encendida: false, errSet: dbus.MakeFailedError(errors.New("solo lectura"))}
		a := busA11yFalso(t, e)
		if _, err := a.Enable(context.Background()); err == nil || errors.Is(err, platform.ErrSinAccesibilidad) {
			t.Fatalf("err = %v, quiero un error que no sea ErrSinAccesibilidad", err)
		}
		if encendida, sets := e.estado(); encendida || sets != 0 {
			t.Errorf("el bus quedó encendida=%v con %d Set", encendida, sets)
		}
	})
}

// Con el contexto cancelado no se abre el bus siquiera (contrato de platform).
func TestAccesibilidadContextoCancelado(t *testing.T) {
	a := accesibilidadSesion{conectar: func() (*dbus.Conn, error) {
		t.Error("no debía conectar con el contexto cancelado")
		return nil, errors.New("no")
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Enable(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, quiero context.Canceled", err)
	}
}
