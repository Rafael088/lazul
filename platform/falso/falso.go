// Package falso es un sistema en memoria que cumple el contrato. Lo usan
// ./dev sim (receptores falsos) y las pruebas del núcleo.
package falso

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sync"

	"github.com/Rafael088/lazul/platform"
)

// Falso guarda el estado del equipo simulado.
type Falso struct {
	mu          sync.Mutex
	texto       string
	app         platform.App
	instaladas  map[string]bool
	archivos    map[string]bool
	abiertas    []platform.AppRef
	ancho, alto int
}

// New crea un equipo falso con nombre (sale en la app activa y en la
// captura) y una lista de apps instaladas.
func New(nombre string, instaladas ...string) *Falso {
	f := &Falso{
		app:        platform.App{ID: "falso." + nombre, Nombre: "Falso " + nombre, Titulo: nombre, PID: 1},
		instaladas: map[string]bool{},
		archivos:   map[string]bool{},
		ancho:      64, alto: 36,
	}
	for _, id := range instaladas {
		f.instaladas[id] = true
	}
	return f
}

// Sistema devuelve f como platform.Sistema.
func (f *Falso) Sistema() platform.Sistema {
	return platform.Sistema{Screenshot: f, Clipboard: f, ActiveApp: f, LaunchApp: f}
}

// PonerApp cambia la ventana con el foco.
func (f *Falso) PonerApp(app platform.App) { f.mu.Lock(); f.app = app; f.mu.Unlock() }

// PonerArchivo hace que exista un archivo (para Launch con Archivo); los
// que están en el disco también existen.
func (f *Falso) PonerArchivo(ruta string) { f.mu.Lock(); f.archivos[ruta] = true; f.mu.Unlock() }

// Abiertas devuelve lo que se pidió abrir, en orden.
func (f *Falso) Abiertas() []platform.AppRef {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]platform.AppRef(nil), f.abiertas...)
}

func (f *Falso) Capture(ctx context.Context) (platform.Imagen, error) {
	if err := ctx.Err(); err != nil {
		return platform.Imagen{}, err
	}
	img := image.NewRGBA(image.Rect(0, 0, f.ancho, f.alto))
	for y := range f.alto {
		for x := range f.ancho {
			img.Set(x, y, color.RGBA{0x1f, 0x4e, uint8(x * 4), 0xff})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return platform.Imagen{}, err
	}
	return platform.Imagen{PNG: b.Bytes(), Ancho: f.ancho, Alto: f.alto}, nil
}

func (f *Falso) ReadText(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.texto, nil
}

func (f *Falso) WriteText(ctx context.Context, texto string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	f.texto = texto
	f.mu.Unlock()
	return nil
}

func (f *Falso) Active(ctx context.Context) (platform.App, error) {
	if err := ctx.Err(); err != nil {
		return platform.App{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.app.ID == "" {
		return platform.App{}, platform.ErrSinVentana
	}
	return f.app, nil
}

func (f *Falso) Launch(ctx context.Context, ref platform.AppRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case ref.URL != "":
	case ref.Archivo != "":
		// Existe si se puso con PonerArchivo o si está en el disco (lo que
		// llegó por LocalSend en ./dev sim).
		if _, err := os.Stat(ref.Archivo); !f.archivos[ref.Archivo] && err != nil {
			return fmt.Errorf("archivo %q: %w", ref.Archivo, platform.ErrNoEncontrada)
		}
	case ref.ID != "":
		if !f.instaladas[ref.ID] {
			return fmt.Errorf("app %q: %w", ref.ID, platform.ErrNoEncontrada)
		}
	default:
		return platform.ErrRefInvalida
	}
	f.abiertas = append(f.abiertas, ref)
	return nil
}
