package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rafael088/lazul/platform"
)

// bordeFalso imita a Hyprland: recuerda si el redimensionado en el borde está
// encendido y cuántas veces se leyó o cambió.
type bordeFalso struct {
	valor      bool
	lecturas   int
	cambios    int
	errLeer    error
	errCambiar error
}

var _ platform.BorderResize = (*bordeFalso)(nil)

func (f *bordeFalso) ResizeOnBorder(context.Context) (bool, error) {
	f.lecturas++
	return f.valor, f.errLeer
}

func (f *bordeFalso) SetResizeOnBorder(_ context.Context, on bool) error {
	f.cambios++
	if f.errCambiar != nil {
		return f.errCambiar
	}
	f.valor = on
	return nil
}

func rutaBordePrueba(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), NombreBorde)
}

func TestBordeApagaYRestaura(t *testing.T) {
	ruta := rutaBordePrueba(t)
	f := &bordeFalso{valor: true}
	avisos := 0
	b := NuevoBorde(f, ruta, nil)
	b.Aviso = func(texto string) {
		avisos++
		if texto != "borde: clic" {
			t.Errorf("aviso = %q", texto)
		}
	}
	if err := b.Encender(); err != nil {
		t.Fatal(err)
	}
	if f.valor || f.cambios != 1 {
		t.Fatalf("tras encender: valor %v, cambios %d", f.valor, f.cambios)
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no quedó el valor original: %v", err)
	}
	if !strings.Contains(string(datos), `"resize_on_border":true`) {
		t.Errorf("borde.json = %s", datos)
	}
	if avisos != 1 {
		t.Errorf("avisos = %d, quiere 1", avisos)
	}
	// Repetir el encendido no vuelve a consultar ni a cambiar.
	if err := b.Encender(); err != nil {
		t.Fatal(err)
	}
	if f.lecturas != 1 || f.cambios != 1 || avisos != 1 {
		t.Errorf("repetir encendido: lecturas %d, cambios %d, avisos %d", f.lecturas, f.cambios, avisos)
	}
	if err := b.Restaurar(); err != nil {
		t.Fatal(err)
	}
	if !f.valor || f.cambios != 2 {
		t.Fatalf("tras restaurar: valor %v, cambios %d", f.valor, f.cambios)
	}
	if _, err := os.Stat(ruta); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("borde.json no se borró: %v", err)
	}
	// Restaurar otra vez no enciende nada por su cuenta.
	if err := b.Restaurar(); err != nil {
		t.Fatal(err)
	}
	if f.cambios != 2 {
		t.Errorf("restaurar de más cambió el valor: %d cambios", f.cambios)
	}
}

func TestBordeDejaEnPazSiYaEstabaApagado(t *testing.T) {
	ruta := rutaBordePrueba(t)
	f := &bordeFalso{valor: false}
	b := NuevoBorde(f, ruta, nil)
	if err := b.Encender(); err != nil {
		t.Fatal(err)
	}
	if f.cambios != 0 {
		t.Errorf("cambió un valor que ya estaba apagado: %d cambios", f.cambios)
	}
	if _, err := os.Stat(ruta); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("guardó un original que no había que devolver: %v", err)
	}
	if err := b.Restaurar(); err != nil {
		t.Fatal(err)
	}
	if f.cambios != 0 || f.valor {
		t.Errorf("restaurar encendió lo que el dueño tenía apagado: valor %v, cambios %d", f.valor, f.cambios)
	}
}

func TestBordeRecuperaTrasUnaCaida(t *testing.T) {
	ruta := rutaBordePrueba(t)
	f := &bordeFalso{valor: true}
	if err := NuevoBorde(f, ruta, nil).Encender(); err != nil {
		t.Fatal(err)
	}
	// La app se cayó con el valor apagado y borde.json en disco. Otra
	// ejecución arranca y lo devuelve antes de tocar el panel.
	f2 := &bordeFalso{valor: false}
	b2 := NuevoBorde(f2, ruta, nil)
	restaurado, err := b2.Recuperar()
	if err != nil || !restaurado {
		t.Fatalf("Recuperar = %v, %v", restaurado, err)
	}
	if !f2.valor || f2.cambios != 1 {
		t.Errorf("no se devolvió el valor: %v, %d cambios", f2.valor, f2.cambios)
	}
	if _, err := os.Stat(ruta); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("borde.json sigue ahí: %v", err)
	}
	if restaurado, err := b2.Recuperar(); err != nil || restaurado {
		t.Errorf("la segunda recuperación = %v, %v", restaurado, err)
	}
}

func TestBordeNoApagaSiNoPuedeGuardarElOriginal(t *testing.T) {
	// La ruta tiene como padre un archivo: no se puede crear borde.json.
	archivo := filepath.Join(t.TempDir(), "no-es-carpeta")
	if err := os.WriteFile(archivo, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f := &bordeFalso{valor: true}
	b := NuevoBorde(f, filepath.Join(archivo, NombreBorde), nil)
	if err := b.Encender(); err == nil {
		t.Fatal("apagó el borde sin poder guardar el original")
	}
	if !f.valor || f.cambios != 0 {
		t.Errorf("tocó Hyprland sin poder recuperar: valor %v, cambios %d", f.valor, f.cambios)
	}
}

func TestBordeSiElCambioFallaConservaElOriginal(t *testing.T) {
	ruta := rutaBordePrueba(t)
	f := &bordeFalso{valor: true, errCambiar: errors.New("hyprctl colgado")}
	b := NuevoBorde(f, ruta, nil)
	if err := b.Encender(); err == nil {
		t.Fatal("aceptó el error del compositor")
	}
	// El comando pudo aplicarse antes de fallar: borde.json se conserva para
	// devolver el valor original al apagar el panel o al arrancar.
	if _, err := os.Stat(ruta); err != nil {
		t.Errorf("no conservó el original para recuperarlo: %v", err)
	}
	// Cuando el compositor vuelve a responder, se puede apagar de nuevo.
	f.errCambiar = nil
	if err := b.Encender(); err != nil {
		t.Fatal(err)
	}
	if f.valor {
		t.Error("no apagó el borde al segundo intento")
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Errorf("no hay original guardado tras el segundo intento: %v", err)
	}
}

func TestPanelEnciendeYApagaElBordeConElModo(t *testing.T) {
	f := &bordeFalso{valor: true}
	b := NuevoBorde(f, rutaBordePrueba(t), nil)
	p, err := NuevoPanel(filepath.Join(t.TempDir(), NombreCalibracionPanel), nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Borde = b
	p.Encender(true)
	if f.valor {
		t.Fatal("la L no apagó el redimensionado en el borde")
	}
	p.Encender(false)
	if !f.valor {
		t.Fatal("apagar la L no devolvió el redimensionado en el borde")
	}
}
