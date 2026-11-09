package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Rafael088/lazul/core/apps"
	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Remoto es un receptor emparejado y conectado, visto desde el controlador.
type Remoto struct {
	Nombre string
	Con    *Conexion
	// HTTP y Base ("https://host:puerto") son para subirle archivos por
	// LocalSend; HTTP debe fijar su huella (ClienteHTTP).
	HTTP *http.Client
	Base string

	mu  sync.Mutex
	sig int
}

// Pedir manda un Sobre y espera la respuesta que lo cita. Una a la vez por
// Remoto: la conexión no mezcla respuestas.
func (r *Remoto) Pedir(ctx context.Context, s *lazulv1.Sobre) (*lazulv1.Sobre, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sig++
	s.Version = uint32(lazulv1.Version_VERSION_ACTUAL)
	s.Id = fmt.Sprintf("c-%d", r.sig)
	s.EnviadoMs = time.Now().UnixMilli()
	if err := r.Con.Enviar(ctx, s); err != nil {
		return nil, err
	}
	resp, err := r.Con.Recibir(ctx)
	if err != nil {
		return nil, err
	}
	if resp.GetEnRespuestaA() != s.Id {
		return nil, fmt.Errorf("respuesta a %q, esperaba %q", resp.GetEnRespuestaA(), s.Id)
	}
	return resp, nil
}

// Entregar manda la Orden Entregar y falla si el receptor no pudo.
func (r *Remoto) Entregar(ctx context.Context, o *lazulv1.Objeto) error {
	resp, err := r.Pedir(ctx, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Orden{Orden: &lazulv1.Orden{
		Accion: &lazulv1.Orden_Entregar{Entregar: &lazulv1.Entregar{Objeto: o}}}}})
	if err != nil {
		return err
	}
	if rr := resp.GetRespuesta(); !rr.GetOk() {
		return &ErrRemoto{Nombre: r.Nombre, Codigo: rr.GetFalla().GetCodigo(), Detalle: rr.GetFalla().GetDetalle()}
	}
	return nil
}

// ErrRemoto es la falla con que el otro equipo contestó una Orden. El Detalle
// puede llevar nombres o rutas de ese equipo: para el registro vale solo el
// Codigo.
type ErrRemoto struct {
	Nombre  string
	Codigo  lazulv1.Falla_Codigo
	Detalle string
}

func (e *ErrRemoto) Error() string {
	return fmt.Sprintf("%s no pudo: %v %s", e.Nombre, e.Codigo, e.Detalle)
}

// ErrVentana: no se pudo leer la ventana activa de este equipo.
var ErrVentana = errors.New("no se pudo leer la ventana activa")

// ErrArchivoAbierto: el documento abierto no se pudo leer o es demasiado grande.
var ErrArchivoAbierto = errors.New("documento abierto no enviable")

// Enrutador convierte los gestos de la cámara de este equipo en Órdenes:
// agarrar toma la captura de esta pantalla, soltar hacia un lado la manda al
// equipo de ese lado (los bytes por LocalSend, el aviso por Entregar) y el
// gesto 4 manda la ventana activa por niveles (URL, archivo o app).
//
// Lo agarrado sale siempre de este equipo (el que tiene la cámara); agarrar
// en un receptor y soltar en otro queda para cuando haya más de 2 equipos.
type Enrutador struct {
	Local    platform.Sistema
	Catalogo *apps.Catalogo
	SO       string // de este equipo en el catálogo; vacío es runtime.GOOS
	Yo       localsend.Dispositivo
	// Lados dice qué equipo está hacia cada dirección; PorDefecto recibe lo
	// que se suelta hacia un lado sin equipo (con 2 PCs, el otro).
	Lados      map[lazulv1.Direccion]*Remoto
	PorDefecto *Remoto
	// Adelantar: si hay un solo destino posible (sin Lados), la captura se
	// empieza a subir al agarrar, mientras la mano hace el gesto de soltar;
	// al soltar solo se espera lo que falte y se manda la Orden.
	Adelantar bool
	// Reloj se puede cambiar en pruebas.
	Reloj func() time.Time

	mu       sync.Mutex
	agarrado *localsend.Archivo
	subida   *subida
}

// subida es una captura que se adelantó a subir al agarrar.
type subida struct {
	a     *Remoto
	listo chan struct{}
	ses   string
	err   error
}

// Hecho cuenta lo que hizo un gesto y cuánto tardó cada tramo.
type Hecho struct {
	Gesto   lazulv1.Gesto
	Que     string // "captura", "url", "archivo", "app", "" si no hizo nada
	Destino string
	Tomar   time.Duration // leer la pantalla o la ventana activa, aquí
	Subir   time.Duration // bytes por LocalSend
	Orden   time.Duration // Entregar hasta la Respuesta ok
	Total   time.Duration // desde que llegó el gesto hasta la Respuesta
	// App es el id del catálogo de la app mandada en el gesto 4, o
	// "fuera_del_catalogo" si el catálogo no la conoce en este sistema.
	// Nunca el nombre ni el ejecutable (eso puede decir qué usa la persona).
	App string
	// Adelantada: la captura se subió desde el agarrar; Subir es solo lo
	// que hubo que esperarla después de soltar.
	Adelantada bool
}

// ErrNadaAgarrado: soltar sin haber agarrado.
var ErrNadaAgarrado = errors.New("soltar sin nada agarrado")

func (e *Enrutador) ahora() time.Time {
	if e.Reloj != nil {
		return e.Reloj()
	}
	return time.Now()
}

func (e *Enrutador) destino(d lazulv1.Direccion) (*Remoto, error) {
	if r := e.Lados[d]; r != nil {
		return r, nil
	}
	if e.PorDefecto != nil {
		return e.PorDefecto, nil
	}
	return nil, fmt.Errorf("no hay ningún equipo hacia %v", d)
}

// Gesto hace lo que pide ev. El tiempo cuenta desde que se llama, que es
// cuando el clasificador reconoció el gesto.
func (e *Enrutador) Gesto(ctx context.Context, ev *lazulv1.EventoGesto) (Hecho, error) {
	ini := e.ahora()
	h := Hecho{Gesto: ev.GetGesto()}
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error
	switch ev.GetGesto() {
	case lazulv1.Gesto_GESTO_AGARRAR:
		h.Que = "captura"
		err = e.agarrar(ctx, &h)
	case lazulv1.Gesto_GESTO_SOLTAR:
		err = e.soltar(ctx, ev.GetDireccion(), &h)
	case lazulv1.Gesto_GESTO_MANDAR_APP:
		err = e.mandarApp(ctx, ev.GetDireccion(), &h)
	}
	h.Total = e.ahora().Sub(ini)
	return h, err
}

func (e *Enrutador) agarrar(ctx context.Context, h *Hecho) error {
	if e.Local.Screenshot == nil {
		return platform.ErrNoImplementado
	}
	t := e.ahora()
	img, err := e.Local.Screenshot.Capture(ctx)
	h.Tomar = e.ahora().Sub(t)
	if err != nil {
		return err
	}
	a := &localsend.Archivo{Nombre: fmt.Sprintf("captura-%s.png", e.ahora().Format("20060102-150405.000")), Mime: "image/png", Datos: img.PNG}
	e.agarrado, e.subida = a, nil
	if e.Adelantar && len(e.Lados) == 0 && e.PorDefecto != nil {
		r := e.PorDefecto
		sb := &subida{a: r, listo: make(chan struct{})}
		e.subida = sb
		go func() {
			// Con su propio plazo: el ctx del gesto de agarrar ya terminó.
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			sb.ses, sb.err = localsend.Enviar(ctx, r.HTTP, r.Base, e.Yo, *a)
			close(sb.listo)
		}()
	}
	return nil
}

func (e *Enrutador) soltar(ctx context.Context, d lazulv1.Direccion, h *Hecho) error {
	if e.agarrado == nil {
		return ErrNadaAgarrado
	}
	a := *e.agarrado
	h.Que = "captura"
	if sb := e.subida; sb != nil {
		e.subida = nil
		if r, err := e.destino(d); err == nil && r == sb.a {
			t := e.ahora()
			select {
			case <-sb.listo:
			case <-ctx.Done():
				return ctx.Err()
			}
			h.Subir, h.Adelantada = e.ahora().Sub(t), true
			if sb.err == nil {
				h.Destino = r.Nombre
				t = e.ahora()
				err := r.Entregar(ctx, objetoArchivo(a, sb.ses))
				h.Orden = e.ahora().Sub(t)
				if err == nil {
					e.agarrado = nil
				}
				return err
			}
			// Si la subida adelantada falló, se intenta de nuevo ahora.
		}
	}
	if err := e.mandarArchivo(ctx, d, a, h); err != nil {
		return err
	}
	e.agarrado = nil
	return nil
}

func objetoArchivo(a localsend.Archivo, ses string) *lazulv1.Objeto {
	suma := sha256.Sum256(a.Datos)
	return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Archivo{Archivo: &lazulv1.Archivo{
		Nombre: a.Nombre, Bytes: uint64(len(a.Datos)), Sha256: suma[:], Mime: a.Mime, TransferenciaId: ses}}}
}

// mandarArchivo sube a por LocalSend y después avisa con Entregar, que el
// receptor solo acepta si el archivo llegó entero y con ese sha256.
func (e *Enrutador) mandarArchivo(ctx context.Context, d lazulv1.Direccion, a localsend.Archivo, h *Hecho) error {
	r, err := e.destino(d)
	if err != nil {
		return err
	}
	h.Destino = r.Nombre
	t := e.ahora()
	ses, err := localsend.Enviar(ctx, r.HTTP, r.Base, e.Yo, a)
	h.Subir = e.ahora().Sub(t)
	if err != nil {
		return fmt.Errorf("LocalSend a %s: %w", r.Nombre, err)
	}
	t = e.ahora()
	err = r.Entregar(ctx, objetoArchivo(a, ses))
	h.Orden = e.ahora().Sub(t)
	return err
}

// MaxArchivoGesto4: el documento abierto que se manda con el gesto 4.
const MaxArchivoGesto4 = 64 << 20

func (e *Enrutador) mandarApp(ctx context.Context, d lazulv1.Direccion, h *Hecho) error {
	if e.Local.ActiveApp == nil {
		return platform.ErrNoImplementado
	}
	t := e.ahora()
	app, err := e.Local.ActiveApp.Active(ctx)
	h.Tomar = e.ahora().Sub(t)
	if err != nil {
		if errors.Is(err, platform.ErrNoImplementado) || errors.Is(err, platform.ErrSinVentana) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrVentana, err)
	}
	so := e.SO
	if so == "" {
		so = (&Receptor{}).so()
	}
	obj := ObjetoDeApp(app, e.Catalogo, so)
	if a := obj.GetApp(); a != nil {
		if h.App = a.GetCatalogoId(); h.App == "" {
			h.App = "fuera_del_catalogo"
		}
	}
	if ar := obj.GetArchivo(); ar != nil {
		// Nivel 2: el documento viaja por LocalSend y el otro lo abre.
		h.Que = "archivo"
		info, err := os.Stat(ar.GetNombre())
		if err != nil {
			return fmt.Errorf("%w: %v", ErrArchivoAbierto, err)
		}
		if info.Size() > MaxArchivoGesto4 {
			return fmt.Errorf("%w: %d bytes, más de %d", ErrArchivoAbierto, info.Size(), MaxArchivoGesto4)
		}
		datos, err := os.ReadFile(ar.GetNombre())
		if err != nil {
			return fmt.Errorf("%w: %v", ErrArchivoAbierto, err)
		}
		tipo := mime.TypeByExtension(filepath.Ext(ar.GetNombre()))
		if tipo == "" {
			tipo = "application/octet-stream"
		}
		return e.mandarArchivo(ctx, d, localsend.Archivo{Nombre: filepath.Base(ar.GetNombre()), Mime: tipo, Datos: datos}, h)
	}
	h.Que = "app"
	if obj.GetUrl() != "" {
		h.Que = "url"
	}
	r, err := e.destino(d)
	if err != nil {
		return err
	}
	h.Destino = r.Nombre
	t = e.ahora()
	err = r.Entregar(ctx, obj)
	h.Orden = e.ahora().Sub(t)
	return err
}
