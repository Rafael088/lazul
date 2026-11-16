package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
)

type accionesTactilesPrueba struct {
	hechas []string
	err    error
}

func (a *accionesTactilesPrueba) Scroll(ctx context.Context, d platform.ScrollDirection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.hechas = append(a.hechas, map[platform.ScrollDirection]string{platform.ScrollUp: "scroll-arriba", platform.ScrollDown: "scroll-abajo"}[d])
	return a.err
}

func (a *accionesTactilesPrueba) Zoom(ctx context.Context, d platform.ZoomDirection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.hechas = append(a.hechas, map[platform.ZoomDirection]string{platform.ZoomIn: "zoom-acercar", platform.ZoomOut: "zoom-alejar"}[d])
	return a.err
}

func (a *accionesTactilesPrueba) Navigate(ctx context.Context, d platform.NavigationDirection, m platform.NavigationMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := map[platform.NavigationDirection]string{platform.NavigateLeft: "izquierda", platform.NavigateRight: "derecha"}[d]
	modo := map[platform.NavigationMode]string{platform.NavigationNormal: "normal", platform.NavigationSlides: "diapositivas"}[m]
	a.hechas = append(a.hechas, "navegar-"+dir+"-"+modo)
	return a.err
}

func recorridoTactil(t *testing.T, nombres string) []gestures.Cuadro {
	t.Helper()
	dir := filepath.Join("..", "gestures", "testdata", "tactiles")
	gs, err := LeerRecorrido(dir, nombres)
	if err != nil {
		t.Fatal(err)
	}
	return Encadenar(gs, 400)
}

func pasarTactil(t *testing.T, v *Tactil, cs []gestures.Cuadro, t0 time.Time) Vista {
	t.Helper()
	var vista Vista
	for _, c := range cs {
		vista = v.Cuadro(c, t0.Add(time.Duration(c.TMs)*time.Millisecond))
	}
	return vista
}

func TestTactilConectaGestosConAccionesDeLinux(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, err := AbrirRegistro(ruta)
	if err != nil {
		t.Fatal(err)
	}
	acciones := &accionesTactilesPrueba{}
	v := &Tactil{Reg: reg, Acciones: acciones}
	cs := recorridoTactil(t, "modo-activar-buena-001,scroll-07-arriba-buena-001,scroll-08-abajo-mala-001,zoom-07-abrir-buena-001,zoom-08-cerrar-mala-001,barrido-07-derecha-buena-001,barrido-08-izquierda-mala-001")
	vista := pasarTactil(t, v, cs, time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC))
	v.Parar(time.Now())
	if err := reg.Cerrar(); err != nil {
		t.Fatal(err)
	}

	quiere := []string{"scroll-arriba", "scroll-abajo", "zoom-acercar", "zoom-alejar", "navegar-derecha-diapositivas", "navegar-izquierda-diapositivas"}
	if !slices.Equal(acciones.hechas, quiere) {
		t.Fatalf("acciones %v, quiere %v", acciones.hechas, quiere)
	}
	if vista.Modo != "activo" || vista.Ultimo != "BARRIDO → izquierda" || vista.ApagaEnMs <= 0 {
		t.Errorf("vista final %+v", vista)
	}
	gestos := deTipo(lineas(t, ruta), TipoGesto)
	if len(gestos) != 7 {
		t.Fatalf("registro de gestos %+v", gestos)
	}
	for _, l := range gestos {
		if l.Resultado != "ok" || l.Gesto == "" {
			t.Errorf("gesto sin resultado completo: %+v", l)
		}
	}
}

func TestTactilRegistraFallaDeLaAccion(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, _ := AbrirRegistro(ruta)
	acciones := &accionesTactilesPrueba{err: errors.New("hyprctl no respondió")}
	v := &Tactil{Reg: reg, Acciones: acciones}
	cs := recorridoTactil(t, "modo-activar-buena-001,scroll-07-arriba-buena-001")
	vista := pasarTactil(t, v, cs, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()

	gestos := deTipo(lineas(t, ruta), TipoGesto)
	if len(gestos) != 2 || gestos[1].Resultado != "falla" || gestos[1].Error != "otro" {
		t.Fatalf("falla no registrada: %+v", gestos)
	}
	if vista.Error == "" {
		t.Fatal("la ventana no recibió el error de la acción")
	}
}

func TestTactilSeApagaTrasTresSegundosSinMano(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, _ := AbrirRegistro(ruta)
	acciones := &accionesTactilesPrueba{}
	v := &Tactil{Reg: reg, Acciones: acciones}
	cs := recorridoTactil(t, "modo-activar-buena-001")
	ult := cs[len(cs)-1].TMs
	for ms := ult + 33; ms <= ult+3500; ms += 33 {
		cs = append(cs, gestures.Cuadro{TMs: ms, Aspecto: 4.0 / 3.0})
	}
	vista := pasarTactil(t, v, cs, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()

	if vista.Modo != "inactivo" || vista.ApagaEnMs != 0 {
		t.Fatalf("no se apagó: %+v", vista)
	}
	gestos := deTipo(lineas(t, ruta), TipoGesto)
	if len(gestos) != 2 || gestos[1].Gesto != "MODO" || gestos[1].Resultado != "sin_mano" {
		t.Fatalf("registro del apagado %+v", gestos)
	}
	acts := deTipo(lineas(t, ruta), TipoActivacion)
	if len(acts) != 1 || acts[0].Resultado != ActSinGesto {
		t.Fatalf("activación %+v", acts)
	}
}

func TestTactilPruebaRegistraPeroNoActua(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "r.jsonl")
	reg, _ := AbrirRegistro(ruta)
	acciones := &accionesTactilesPrueba{}
	v := &Tactil{Reg: reg, Acciones: acciones}
	v.Prueba(true, time.Now())
	cs := recorridoTactil(t, "modo-activar-buena-001,scroll-07-arriba-buena-001")
	pasarTactil(t, v, cs, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()

	if len(acciones.hechas) != 0 {
		t.Fatalf("la prueba ejecutó %v", acciones.hechas)
	}
	gestos := deTipo(lineas(t, ruta), TipoGesto)
	if len(gestos) != 2 || gestos[1].Resultado != "no_ejecutada_prueba" || !gestos[1].Prueba {
		t.Fatalf("registro de prueba %+v", gestos)
	}
}
