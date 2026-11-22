package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Rafael088/lazul/platform"
)

// NombreBorde es el archivo donde queda el valor original de
// general:resize_on_border mientras Lazul lo tiene apagado. Se escribe antes
// de apagarlo: si la app se cae, al arrancar se devuelve desde aquí.
const NombreBorde = "borde.json"

// TiempoBorde acota cada consulta o cambio a hyprctl. No va en el camino del
// cuadro: solo se toca al encender o apagar el panel.
const TiempoBorde = 2 * time.Second

// Borde apaga el redimensionado al tocar el borde de las ventanas mientras el
// panel está encendido. Omarchy trae general:resize_on_border = true y 8 px de
// agarre, así que un toque cerca del borde redimensionaba la ventana en vez de
// hacer clic. Al apagar el panel devuelve el valor original; si la app se cae,
// el original queda en borde.json y se restaura al arrancar.
type Borde struct {
	Control platform.BorderResize
	Ruta    string // borde.json en la carpeta de estado
	Reg     *Registro
	// Aviso, si no es nil, se llama una sola vez (la primera vez que Lazul
	// apaga el redimensionado) para decirlo en el indicador.
	Aviso func(string)

	mu      sync.Mutex
	apagado bool // Lazul lo apagó y todavía no lo devolvió
	avisado bool
}

// NuevoBorde prepara el control. Si Control es nil, todos los métodos no
// hacen nada (otro compositor u otro sistema).
func NuevoBorde(c platform.BorderResize, ruta string, reg *Registro) *Borde {
	return &Borde{Control: c, Ruta: ruta, Reg: reg}
}

// valorBorde es lo que queda en borde.json. Solo se escribe cuando había que
// apagarlo; el valor guardado es el que hay que devolver.
type valorBorde struct {
	ResizeOnBorder bool `json:"resize_on_border"`
}

func (b *Borde) guardar(original bool) error {
	if b.Ruta == "" {
		return errors.New("no hay dónde guardar general:resize_on_border")
	}
	if err := os.MkdirAll(filepath.Dir(b.Ruta), 0o700); err != nil {
		return err
	}
	datos, err := json.Marshal(valorBorde{ResizeOnBorder: original})
	if err != nil {
		return err
	}
	datos = append(datos, '\n')
	tmp := b.Ruta + ".tmp"
	if err := os.WriteFile(tmp, datos, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, b.Ruta); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (b *Borde) leer() (original bool, hay bool, err error) {
	if b.Ruta == "" {
		return false, false, nil
	}
	datos, err := os.ReadFile(b.Ruta)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	var v valorBorde
	if err := json.Unmarshal(datos, &v); err != nil {
		return false, false, fmt.Errorf("%s: %w", NombreBorde, err)
	}
	return v.ResizeOnBorder, true, nil
}

func (b *Borde) borrar() {
	if b.Ruta != "" {
		_ = os.Remove(b.Ruta)
	}
}

func (b *Borde) anotar(resultado string, err error) {
	l := Linea{Tipo: TipoPanel, Resultado: resultado}
	if err != nil {
		l.Error = ClaseError(err)
	}
	b.Reg.Anotar(l)
}

// Encender es lo que pasa al encender el panel: lee el valor actual, y si está
// encendido lo apaga y guarda el original. Si ya estaba apagado (el dueño lo
// apagó a mano) no toca nada, para no encendérselo después. Si hay un
// borde.json de una ejecución que se cayó, primero devuelve ese valor.
func (b *Borde) Encender() error {
	if b == nil || b.Control == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.apagado {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), TiempoBorde)
	defer cancel()
	if original, hay, err := b.leer(); err != nil {
		b.anotar("borde_error", err)
		return err
	} else if hay {
		// Quedó de una caída: se devuelve antes de volver a apagarlo.
		if err := b.Control.SetResizeOnBorder(ctx, original); err != nil {
			b.anotar("borde_error", err)
			return err
		}
		b.borrar()
	}
	actual, err := b.Control.ResizeOnBorder(ctx)
	if err != nil {
		b.anotar("borde_error", err)
		return err
	}
	if !actual {
		// Ya estaba apagado: nada que devolver al final.
		b.apagado = true
		return nil
	}
	// Primero el archivo: si la app se cae justo después, al arrancar se
	// sabe qué devolver. Si no se puede guardar, mejor no apagarlo.
	if err := b.guardar(true); err != nil {
		b.anotar("borde_error", err)
		return err
	}
	if err := b.Control.SetResizeOnBorder(ctx, false); err != nil {
		// El comando pudo aplicarse antes de fallar: se conserva borde.json
		// para devolver el original al apagar el panel o al arrancar.
		b.anotar("borde_error", err)
		return err
	}
	b.apagado = true
	b.anotar("borde_apagado", nil)
	if !b.avisado && b.Aviso != nil {
		b.avisado = true
		b.Aviso("borde: clic")
	}
	return nil
}

// Restaurar devuelve el valor original (al apagar el panel o cerrar la app).
// Si no había nada guardado, no enciende nada: el dueño ya lo tenía apagado.
func (b *Borde) Restaurar() error {
	if b == nil || b.Control == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	original, hay, err := b.leer()
	if err != nil {
		b.anotar("borde_error", err)
		return err
	}
	if !hay {
		b.apagado = false
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), TiempoBorde)
	defer cancel()
	if err := b.Control.SetResizeOnBorder(ctx, original); err != nil {
		// Se conserva el archivo para reintentarlo al arrancar.
		b.anotar("borde_error", err)
		return err
	}
	b.borrar()
	b.apagado = false
	b.anotar("borde_restaurado", nil)
	return nil
}

// Recuperar devuelve el valor que dejó una ejecución anterior que se cayó con
// el redimensionado apagado. Se llama al arrancar. Devuelve true si restauró.
func (b *Borde) Recuperar() (bool, error) {
	if b == nil || b.Control == nil {
		return false, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	original, hay, err := b.leer()
	if err != nil {
		b.anotar("borde_error", err)
		return false, err
	}
	if !hay {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), TiempoBorde)
	defer cancel()
	if err := b.Control.SetResizeOnBorder(ctx, original); err != nil {
		b.anotar("borde_error", err)
		return false, err
	}
	b.borrar()
	b.anotar("borde_recuperado", nil)
	return true, nil
}
