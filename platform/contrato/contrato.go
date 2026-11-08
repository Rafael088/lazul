// Package contrato tiene las pruebas que toda implementación de platform
// debe pasar. Cada sistema las corre desde su propio _test.go:
//
//	func TestContrato(t *testing.T) {
//		contrato.Exigir(t)                  // se salta sin LAZUL_CONTRATO=1
//		contrato.Correr(t, linux.New(), contrato.Opciones{AppInofensiva: "..."})
//	}
//
// Se corren con `./dev contrato` (Linux) o `./dev win contrato` (Windows, en
// la sesión del escritorio). Necesitan un escritorio real con alguna ventana
// enfocada; por eso no van en CI. Los meta-tests de este paquete verifican
// que una implementación vacía (platform/vacio) falla todas las pruebas y
// que la falsa (platform/falso) las pasa todas.
package contrato

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Rafael088/lazul/platform"
)

// Opciones ajusta el contrato a cada sistema.
type Opciones struct {
	// AppInofensiva es un ID que la implementación sabe abrir y que no hace
	// daño (por ejemplo, la calculadora). Vacío: se salta esa prueba.
	AppInofensiva string
	// Abrir habilita las pruebas que abren ventanas de verdad. Correr lo
	// prende con LAZUL_CONTRATO_ABRIR=1.
	Abrir bool
}

// ErrOmitida marca una prueba que no aplica con estas Opciones.
var ErrOmitida = errors.New("contrato: omitida")

// Prueba es una regla del contrato: devuelve nil si la implementación cumple.
type Prueba struct {
	Nombre string
	Correr func(ctx context.Context) error
}

// MaxCaptura es lo más que puede tardar una captura (el comando completo
// debe quedar por debajo de 100 ms; la captura es la parte cara).
const MaxCaptura = time.Second

// Exigir salta la prueba si no se pidió el contrato con LAZUL_CONTRATO=1.
func Exigir(t *testing.T) {
	t.Helper()
	if os.Getenv("LAZUL_CONTRATO") != "1" {
		t.Skip("contrato de platform: correr con ./dev contrato o ./dev win contrato")
	}
}

// Correr corre todas las pruebas del contrato como subtests.
func Correr(t *testing.T, s platform.Sistema, op Opciones) {
	t.Helper()
	if os.Getenv("LAZUL_CONTRATO_ABRIR") == "1" {
		op.Abrir = true
	}
	for _, p := range Pruebas(s, op) {
		t.Run(p.Nombre, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.Correr(ctx)
			switch {
			case errors.Is(err, ErrOmitida):
				t.Skip(err)
			case err != nil:
				t.Fatal(err)
			}
		})
	}
}

// Pruebas arma la lista completa del contrato para s.
func Pruebas(s platform.Sistema, op Opciones) []Prueba {
	var ps []Prueba
	ps = append(ps, pruebasScreenshot(s.Screenshot)...)
	ps = append(ps, pruebasClipboard(s.Clipboard)...)
	ps = append(ps, pruebasActiveApp(s.ActiveApp)...)
	ps = append(ps, pruebasLaunchApp(s.LaunchApp, op)...)
	return ps
}

func cancelado() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// exigirCancelado: con el contexto ya cancelado hay que devolver ese error.
func exigirCancelado(err error) error {
	if !errors.Is(err, context.Canceled) {
		return fmt.Errorf("con el contexto cancelado debe devolver un error con context.Canceled; devolvió %v", err)
	}
	return nil
}

func falta(nombre string) []Prueba {
	return []Prueba{{Nombre: nombre + "/existe", Correr: func(context.Context) error {
		return fmt.Errorf("%s es nil", nombre)
	}}}
}

func pruebasScreenshot(sc platform.Screenshot) []Prueba {
	if sc == nil {
		return falta("Screenshot")
	}
	return []Prueba{
		{"Screenshot/png_valido", func(ctx context.Context) error {
			inicio := time.Now()
			img, err := sc.Capture(ctx)
			dur := time.Since(inicio)
			if err != nil {
				return fmt.Errorf("Capture: %w", err)
			}
			if img.Ancho <= 0 || img.Alto <= 0 {
				return fmt.Errorf("tamaño %dx%d", img.Ancho, img.Alto)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(img.PNG))
			if err != nil {
				return fmt.Errorf("el PNG no se puede leer: %w", err)
			}
			if cfg.Width != img.Ancho || cfg.Height != img.Alto {
				return fmt.Errorf("el PNG mide %dx%d pero Imagen dice %dx%d", cfg.Width, cfg.Height, img.Ancho, img.Alto)
			}
			if dur > MaxCaptura {
				return fmt.Errorf("tardó %v (máximo %v)", dur, MaxCaptura)
			}
			return nil
		}},
		{"Screenshot/contexto_cancelado", func(context.Context) error {
			_, err := sc.Capture(cancelado())
			return exigirCancelado(err)
		}},
	}
}

func pruebasClipboard(cb platform.Clipboard) []Prueba {
	if cb == nil {
		return falta("Clipboard")
	}
	idaYVuelta := func(ctx context.Context, texto string) error {
		antes, errAntes := cb.ReadText(ctx)
		if err := cb.WriteText(ctx, texto); err != nil {
			return fmt.Errorf("WriteText: %w", err)
		}
		leido, err := cb.ReadText(ctx)
		if errAntes == nil {
			_ = cb.WriteText(ctx, antes) // devolverle al usuario lo que tenía
		}
		if err != nil {
			return fmt.Errorf("ReadText: %w", err)
		}
		if leido != texto {
			return fmt.Errorf("escribí %q y leí %q", texto, leido)
		}
		return nil
	}
	return []Prueba{
		{"Clipboard/ida_y_vuelta", func(ctx context.Context) error {
			return idaYVuelta(ctx, fmt.Sprintf("lazul-contrato-%d", time.Now().UnixNano()))
		}},
		{"Clipboard/unicode", func(ctx context.Context) error {
			return idaYVuelta(ctx, "año, canción, 日本語, emoji 🖐")
		}},
		{"Clipboard/saltos_de_linea", func(ctx context.Context) error {
			return idaYVuelta(ctx, "uno\ndos\n\ntres\n")
		}},
		{"Clipboard/texto_largo", func(ctx context.Context) error {
			return idaYVuelta(ctx, strings.Repeat("lazul ", 50_000)) // ~300 KB
		}},
		{"Clipboard/contexto_cancelado", func(context.Context) error {
			_, err := cb.ReadText(cancelado())
			if err := exigirCancelado(err); err != nil {
				return fmt.Errorf("ReadText: %w", err)
			}
			if err := exigirCancelado(cb.WriteText(cancelado(), "x")); err != nil {
				return fmt.Errorf("WriteText: %w", err)
			}
			return nil
		}},
	}
}

func pruebasActiveApp(aa platform.ActiveApp) []Prueba {
	if aa == nil {
		return falta("ActiveApp")
	}
	return []Prueba{
		{"ActiveApp/ventana_con_foco", func(ctx context.Context) error {
			app, err := aa.Active(ctx)
			if err != nil {
				return fmt.Errorf("Active (¿hay una ventana enfocada?): %w", err)
			}
			if app.ID == "" || app.Nombre == "" {
				return fmt.Errorf("ID y Nombre son obligatorios: %+v", app)
			}
			if app.PID < 0 {
				return fmt.Errorf("PID negativo: %d", app.PID)
			}
			return nil
		}},
		{"ActiveApp/contexto_cancelado", func(context.Context) error {
			_, err := aa.Active(cancelado())
			return exigirCancelado(err)
		}},
	}
}

func pruebasLaunchApp(la platform.LaunchApp, op Opciones) []Prueba {
	if la == nil {
		return falta("LaunchApp")
	}
	return []Prueba{
		{"LaunchApp/ref_vacia", func(ctx context.Context) error {
			if err := la.Launch(ctx, platform.AppRef{}); !errors.Is(err, platform.ErrRefInvalida) {
				return fmt.Errorf("con AppRef vacía debe devolver ErrRefInvalida; devolvió %v", err)
			}
			return nil
		}},
		{"LaunchApp/app_inexistente", func(ctx context.Context) error {
			ref := platform.AppRef{ID: "lazul-contrato-app-que-no-existe"}
			if err := la.Launch(ctx, ref); !errors.Is(err, platform.ErrNoEncontrada) {
				return fmt.Errorf("con una app que no existe debe devolver ErrNoEncontrada; devolvió %v", err)
			}
			return nil
		}},
		{"LaunchApp/archivo_inexistente", func(ctx context.Context) error {
			ref := platform.AppRef{Archivo: "lazul-contrato-no-existe/nada.txt"}
			if err := la.Launch(ctx, ref); !errors.Is(err, platform.ErrNoEncontrada) {
				return fmt.Errorf("con un archivo que no existe debe devolver ErrNoEncontrada; devolvió %v", err)
			}
			return nil
		}},
		{"LaunchApp/app_inofensiva", func(ctx context.Context) error {
			if op.AppInofensiva == "" || !op.Abrir {
				return fmt.Errorf("%w: abre una ventana; LAZUL_CONTRATO_ABRIR=1 y Opciones.AppInofensiva", ErrOmitida)
			}
			if err := la.Launch(ctx, platform.AppRef{ID: op.AppInofensiva}); err != nil {
				return fmt.Errorf("Launch(%q): %w", op.AppInofensiva, err)
			}
			return nil
		}},
		{"LaunchApp/contexto_cancelado", func(context.Context) error {
			return exigirCancelado(la.Launch(cancelado(), platform.AppRef{ID: "x"}))
		}},
	}
}
