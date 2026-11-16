package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
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
	// Las grabaciones van pegadas, así que se apaga IgnorarRegreso: si no,
	// el scroll abajo justo después del arriba contaría como regreso.
	aj := gestures.AjustesTactilesPorDefecto
	aj.IgnorarRegreso = -1
	v := &Tactil{Reg: reg, Acciones: acciones, Ajustes: aj}
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

func TestTactilAlimentaElIndicador(t *testing.T) {
	reg, _ := AbrirRegistro(filepath.Join(t.TempDir(), "r.jsonl"))
	ind := &Indicador{}
	v := &Tactil{Reg: reg, Acciones: &accionesTactilesPrueba{}, AlEvento: ind.Aplicar}
	cs := recorridoTactil(t, "modo-activar-buena-001,scroll-07-arriba-buena-001")
	pasarTactil(t, v, cs, time.Now())
	if vi := ind.Vista(time.Now()); !vi.Activo || vi.Gesto == "" || !vi.Destello {
		t.Fatalf("indicador tras activar y hacer scroll: %+v", vi)
	}

	// Sin mano 3 s: el modo se apaga y el indicador también.
	ult := cs[len(cs)-1].TMs
	var vacios []gestures.Cuadro
	for ms := ult + 33; ms <= ult+3500; ms += 33 {
		vacios = append(vacios, gestures.Cuadro{TMs: ms, Aspecto: 4.0 / 3.0})
	}
	pasarTactil(t, v, vacios, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()
	if ind.Vista(time.Now()).Activo {
		t.Fatal("el indicador sigue activo después de apagarse el modo sin mano")
	}
}

// ruedaPrueba suma lo que la app manda por la rueda virtual.
type ruedaPrueba struct {
	accionesTactilesPrueba
	unidades  int
	llamadas  int
	errRueda  error
	preparada bool
}

func (r *ruedaPrueba) ScrollContinuo(ctx context.Context, n int) error {
	r.llamadas++
	if r.errRueda != nil {
		return r.errRueda
	}
	r.unidades += n
	return nil
}

func (r *ruedaPrueba) Preparar() error { r.preparada = true; return nil }

// sosteniendo repite el último cuadro durante ms, como una mano quieta.
func sosteniendo(cs []gestures.Cuadro, ms int64) []gestures.Cuadro {
	ult := cs[len(cs)-1]
	for d := int64(33); d <= ms; d += 33 {
		c := ult
		c.TMs = ult.TMs + d
		cs = append(cs, c)
	}
	return cs
}

func TestTactilScrollContinuoPorLaRueda(t *testing.T) {
	reg, _ := AbrirRegistro(filepath.Join(t.TempDir(), "r.jsonl"))
	r := &ruedaPrueba{}
	v := &Tactil{Reg: reg, Acciones: r}
	abajo := sosteniendo(recorridoTactil(t, "modo-activar-buena-001,scroll-02-abajo-mala-001"), 1000)
	pasarTactil(t, v, abajo, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()
	if !r.preparada {
		t.Error("la rueda no se preparó al empezar")
	}
	if r.unidades <= 120 {
		t.Fatalf("sosteniendo la mano abajo 1 s la rueda debía bajar más de un paso: %d unidades", r.unidades)
	}
	if len(r.hechas) != 0 {
		t.Fatalf("con rueda no se mandan flechas: %v", r.hechas)
	}
}

func TestTactilSinRuedaVuelveAlScrollPorPasos(t *testing.T) {
	reg, _ := AbrirRegistro(filepath.Join(t.TempDir(), "r.jsonl"))
	r := &ruedaPrueba{errRueda: errors.New("sin /dev/uinput")}
	v := &Tactil{Reg: reg, Acciones: r}
	cs := sosteniendo(recorridoTactil(t, "modo-activar-buena-001,scroll-02-abajo-mala-001"), 500)
	vista := pasarTactil(t, v, cs, time.Now())
	if r.llamadas != 1 || !strings.Contains(vista.Error, "rueda virtual") {
		t.Fatalf("la falla de la rueda debe verse una vez: llamadas %d, error %q", r.llamadas, vista.Error)
	}
	// Soltar, esperar más que IgnorarRegreso y volver a bajar: ya va por flechas.
	mas := sosteniendo(recorridoTactil(t, "scroll-04-abajo-mala-001"), 300)
	base := cs[len(cs)-1].TMs + 3000
	for i := range mas {
		mas[i].TMs += base
	}
	pasarTactil(t, v, mas, time.Now())
	v.Parar(time.Now())
	reg.Cerrar()
	if len(r.hechas) == 0 || r.hechas[len(r.hechas)-1] != "scroll-abajo" {
		t.Fatalf("sin rueda debía volver a las flechas: %v", r.hechas)
	}
}
