//go:build linux

package linux

import (
	"context"
	"fmt"

	"github.com/Rafael088/lazul/platform"
	"github.com/godbus/dbus/v5"
)

// Accesibilidad de la sesión (AT-SPI). Las apps (Firefox, Zen, GTK, Qt) solo
// se registran en el bus de accesibilidad si org.a11y.Status.IsEnabled está
// en true cuando arrancan. Hyprland/Omarchy lo traen en false, así que
// Firefox y Zen no aparecen en el bus y el imán no ve ningún botón.
//
// Al arrancar la app con el imán encendido, Lazul lo enciende por D-Bus. Los
// navegadores que ya estaban abiertos no se registran hacia atrás (solo lo
// miran al arrancar), así que hay que reiniciarlos; la app lo avisa una vez.
//
// Nunca se apaga al cerrar: otras apps y la persona pueden estar usándola.
// La lectura de los objetivos sigue en atspi.go.

const (
	// El bus de accesibilidad se pide en el bus de la sesión (lo arranca su
	// launcher la primera vez) y el estado, en el mismo objeto.
	busA11y           = "org.a11y.Bus"
	rutaA11y          = dbus.ObjectPath("/org/a11y/bus")
	ifazEstadoA11y    = "org.a11y.Status"
	propA11yEncendida = "IsEnabled"
)

// accesibilidadSesion implementa platform.Accessibility con D-Bus.
type accesibilidadSesion struct {
	// conectar abre el bus de la sesión. Las pruebas meten un bus falso; en
	// la app es nil y se usa dbus.ConnectSessionBus.
	conectar func() (*dbus.Conn, error)
}

var _ platform.Accessibility = accesibilidadSesion{}

// Enable lee org.a11y.Status.IsEnabled y, si está en false, lo pone en true.
// No lo apaga nunca. Sin bus de accesibilidad devuelve ErrSinAccesibilidad.
func (a accesibilidadSesion) Enable(ctx context.Context) (platform.AccessibilityResult, error) {
	var res platform.AccessibilityResult
	if err := ctx.Err(); err != nil {
		return res, err
	}
	conectar := a.conectar
	if conectar == nil {
		conectar = func() (*dbus.Conn, error) { return dbus.ConnectSessionBus() }
	}
	c, err := conectar()
	if err != nil {
		return res, fmt.Errorf("%w: bus de la sesión: %v", platform.ErrSinAccesibilidad, err)
	}
	defer c.Close()
	obj := c.Object(busA11y, rutaA11y)
	// GetAddress arranca el launcher si hace falta; sin at-spi2-core no hay
	// servicio y el nombre no tiene dueño.
	var dir string
	if err := obj.CallWithContext(ctx, "org.a11y.Bus.GetAddress", 0).Store(&dir); err != nil {
		return res, fmt.Errorf("%w: %v", platform.ErrSinAccesibilidad, err)
	}
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, ifazEstadoA11y, propA11yEncendida).Store(&v); err != nil {
		return res, fmt.Errorf("leer %s.%s: %w", ifazEstadoA11y, propA11yEncendida, err)
	}
	if encendida, _ := v.Value().(bool); encendida {
		res.WasOn = true
		return res, nil
	}
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, ifazEstadoA11y, propA11yEncendida, dbus.MakeVariant(true)).Store(); err != nil {
		return res, fmt.Errorf("encender %s.%s: %w", ifazEstadoA11y, propA11yEncendida, err)
	}
	res.TurnedOn = true
	return res, nil
}
