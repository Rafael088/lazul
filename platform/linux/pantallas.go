//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/Rafael088/lazul/platform"
)

// nombreTactilHyprland es el nombre con que Hyprland ve el táctil uinput
// («Lazul tactil virtual»): en minúsculas y con guiones.
const nombreTactilHyprland = "lazul-tactil-virtual"

// pantallasHyprland implementa platform.Displays con hyprctl. Target lleva el
// foco a ese monitor (hl.dsp.focus), le asigna la salida del táctil
// (hl.device) y recrea el táctil para que la tome; el puntero absoluto, que
// Hyprland reparte por todo el escritorio, recibe el rectángulo del monitor.
type pantallasHyprland struct {
	ejecutar func(ctx context.Context, programa string, args ...string) ([]byte, error)
	puntero  *punteroVirtual
	tactil   interface{ Recrear() error }
}

var _ platform.Displays = pantallasHyprland{}

// monitorHyprland son los campos de hyprctl monitors -j que hacen falta.
type monitorHyprland struct {
	Nombre    string  `json:"name"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Ancho     int     `json:"width"`
	Alto      int     `json:"height"`
	Escala    float64 `json:"scale"`
	Giro      int     `json:"transform"`
	EnFoco    bool    `json:"focused"`
	Apagado   bool    `json:"disabled"`
	Duplicado string  `json:"mirrorOf"`
}

// leerMonitoresHyprland convierte la salida de hyprctl monitors -j en
// monitores con el tamaño lógico (el modo dividido por la escala, y con ancho
// y alto cambiados si está girado 90° o 270°), que es el que usa el
// escritorio de Hyprland para x e y. Los apagados y los espejos no cuentan.
func leerMonitoresHyprland(datos []byte) ([]platform.Display, error) {
	var ms []monitorHyprland
	if err := json.Unmarshal(datos, &ms); err != nil {
		return nil, fmt.Errorf("leer los monitores de hyprctl: %w", err)
	}
	var ds []platform.Display
	for _, m := range ms {
		if m.Apagado || (m.Duplicado != "" && m.Duplicado != "none") || m.Ancho <= 0 || m.Alto <= 0 {
			continue
		}
		escala := m.Escala
		if escala <= 0 {
			escala = 1
		}
		ancho := int(math.Round(float64(m.Ancho) / escala))
		alto := int(math.Round(float64(m.Alto) / escala))
		if m.Giro%2 == 1 {
			ancho, alto = alto, ancho
		}
		ds = append(ds, platform.Display{Name: m.Nombre, X: m.X, Y: m.Y, Width: ancho, Height: alto, Focused: m.EnFoco})
	}
	return ds, nil
}

func (p pantallasHyprland) List(ctx context.Context) ([]platform.Display, error) {
	datos, err := p.ejecutar(ctx, "hyprctl", "monitors", "-j")
	if err != nil {
		return nil, fmt.Errorf("consultar los monitores con hyprctl: %w", err)
	}
	return leerMonitoresHyprland(datos)
}

// nombreSalida acepta solo nombres de conector (DP-1, eDP-1, HDMI-A-1): el
// nombre va dentro de un código Lua.
var nombreSalida = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// codigoDirigir es el Lua que pasa el foco y el táctil a la salida.
func codigoDirigir(salida string) (string, error) {
	if !nombreSalida.MatchString(salida) {
		return "", fmt.Errorf("nombre de monitor no válido: %w", platform.ErrAccionInvalida)
	}
	return fmt.Sprintf(`hl.device({ name = %q, output = %q }); hl.dispatch(hl.dsp.focus({ monitor = %q }))`,
		nombreTactilHyprland, salida, salida), nil
}

// respuestaEvalFallida: hyprctl eval contesta «ok» (o nada) si el Lua corrió;
// un error de Lua vuelve como texto aunque el proceso salga con 0.
func respuestaEvalFallida(salida []byte) bool {
	s := strings.ToLower(strings.TrimSpace(string(salida)))
	return s != "" && s != "ok"
}

func (p pantallasHyprland) Target(ctx context.Context, name string) error {
	codigo, err := codigoDirigir(name)
	if err != nil {
		return err
	}
	ds, err := p.List(ctx)
	if err != nil {
		return err
	}
	d, ok := platform.PorNombre(ds, name)
	if !ok {
		return fmt.Errorf("no hay monitor %q: %w", name, platform.ErrAccionInvalida)
	}
	salida, err := p.ejecutar(ctx, "hyprctl", "eval", codigo)
	if err != nil || respuestaEvalFallida(salida) {
		return fmt.Errorf("dirigir a %s con hyprctl eval: %v %s", name, err, strings.TrimSpace(string(salida)))
	}
	if p.puntero != nil {
		if len(ds) > 1 {
			p.puntero.dirigir(&d, platform.Abarque(ds))
		} else {
			p.puntero.dirigir(nil, platform.Display{})
		}
	}
	if p.tactil != nil {
		if err := p.tactil.Recrear(); err != nil {
			return err
		}
	}
	return nil
}
