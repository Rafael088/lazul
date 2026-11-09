// Package core es el núcleo de Lazul: recibe Sobres, decide y responde
// usando solo las interfaces de platform. Nunca llama al sistema.
//
// Hoy tiene el receptor (atiende las Órdenes de un controlador), el
// transporte por WebSocket sobre TLS 1.3 con huella fijada y el
// emparejamiento con código de 6 dígitos y permiso de la persona (pareja.go);
// el descubrimiento mDNS está en core/descubre. El enrutador del controlador
// (enrutador.go) convierte gestos en Órdenes, y los archivos viajan por el
// protocolo de LocalSend (core/localsend) en el mismo puerto TLS.
package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/Rafael088/lazul/core/apps"
	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Receptor atiende las Órdenes que llegan a este equipo.
type Receptor struct {
	Sistema platform.Sistema
	Hola    *lazulv1.Hola
	// Catalogo traduce las apps del gesto 4 entre sistemas (apps.json).
	// nil: solo se prueba el nativo_id que llegó y después la web.
	Catalogo *apps.Catalogo
	// SO es el sistema de este equipo en el catálogo ("linux", "windows");
	// vacío es runtime.GOOS.
	SO string
	// Archivos recibe por LocalSend; Servir lo monta en el mismo puerto y
	// solo deja subir a equipos emparejados. nil: no recibe archivos.
	Archivos *localsend.Servidor
	// Registro, si no es nil, se llama tras atender cada Orden con lo que
	// se pidió, lo que se contestó y cuánto tardó aquí.
	Registro func(o *lazulv1.Orden, resp *lazulv1.Respuesta, d time.Duration)
	// Reloj se puede cambiar en pruebas.
	Reloj func() time.Time
}

func (r *Receptor) so() string {
	if r.SO != "" {
		return r.SO
	}
	return runtime.GOOS
}

func (r *Receptor) ahora() int64 {
	if r.Reloj != nil {
		return r.Reloj().UnixMilli()
	}
	return time.Now().UnixMilli()
}

// Atender responde un Sobre. Devuelve nil si no hay nada que contestar.
func (r *Receptor) Atender(ctx context.Context, s *lazulv1.Sobre) *lazulv1.Sobre {
	resp := &lazulv1.Sobre{
		Version:      uint32(lazulv1.Version_VERSION_ACTUAL),
		Id:           "r-" + s.GetId(),
		EnRespuestaA: s.GetId(),
	}
	defer func() { resp.EnviadoMs = r.ahora() }()
	if s.GetVersion() != uint32(lazulv1.Version_VERSION_ACTUAL) {
		resp.Contenido = respuestaFalla(lazulv1.Falla_CODIGO_INVALIDO, fmt.Sprintf("versión %d no soportada", s.GetVersion()))
		return resp
	}
	switch c := s.GetContenido().(type) {
	case *lazulv1.Sobre_Hola:
		resp.Contenido = &lazulv1.Sobre_Hola{Hola: r.Hola}
	case *lazulv1.Sobre_Ping:
		resp.Contenido = &lazulv1.Sobre_Pong{Pong: &lazulv1.Pong{}}
	case *lazulv1.Sobre_Orden:
		ini := time.Now()
		rr := r.orden(ctx, c.Orden)
		if r.Registro != nil {
			r.Registro(c.Orden, rr, time.Since(ini))
		}
		resp.Contenido = &lazulv1.Sobre_Respuesta{Respuesta: rr}
	default:
		return nil
	}
	return resp
}

func (r *Receptor) orden(ctx context.Context, o *lazulv1.Orden) *lazulv1.Respuesta {
	switch a := o.GetAccion().(type) {
	case *lazulv1.Orden_Tomar:
		obj, err := r.tomar(ctx, a.Tomar.GetQue())
		if err != nil {
			return falla(err)
		}
		return &lazulv1.Respuesta{Ok: true, Objeto: obj}
	case *lazulv1.Orden_Entregar:
		if err := r.entregar(ctx, a.Entregar.GetObjeto()); err != nil {
			return falla(err)
		}
		return &lazulv1.Respuesta{Ok: true}
	}
	return falla(fmt.Errorf("%w: orden vacía", errInvalido))
}

var errInvalido = errors.New("mensaje inválido")

func (r *Receptor) tomar(ctx context.Context, que lazulv1.Tomar_Que) (*lazulv1.Objeto, error) {
	s := r.Sistema
	switch que {
	case lazulv1.Tomar_QUE_PORTAPAPELES:
		if s.Clipboard == nil {
			return nil, platform.ErrNoImplementado
		}
		t, err := s.Clipboard.ReadText(ctx)
		if err != nil {
			return nil, err
		}
		return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Texto{Texto: t}}, nil
	case lazulv1.Tomar_QUE_CAPTURA:
		if s.Screenshot == nil {
			return nil, platform.ErrNoImplementado
		}
		img, err := s.Screenshot.Capture(ctx)
		if err != nil {
			return nil, err
		}
		suma := sha256.Sum256(img.PNG)
		// Los bytes viajan aparte por LocalSend (cotejo-sb2kdm).
		return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Archivo{Archivo: &lazulv1.Archivo{
			Nombre: fmt.Sprintf("captura-%d.png", r.ahora()),
			Bytes:  uint64(len(img.PNG)),
			Sha256: suma[:],
			Mime:   "image/png",
		}}}, nil
	case lazulv1.Tomar_QUE_APP_ACTIVA:
		if s.ActiveApp == nil {
			return nil, platform.ErrNoImplementado
		}
		app, err := s.ActiveApp.Active(ctx)
		if err != nil {
			return nil, err
		}
		return ObjetoDeApp(app, r.Catalogo, r.so()), nil
	}
	return nil, fmt.Errorf("%w: Tomar %v", errInvalido, que)
}

func (r *Receptor) entregar(ctx context.Context, o *lazulv1.Objeto) error {
	s := r.Sistema
	switch t := o.GetTipo().(type) {
	case *lazulv1.Objeto_Texto:
		if s.Clipboard == nil {
			return platform.ErrNoImplementado
		}
		return s.Clipboard.WriteText(ctx, t.Texto)
	case *lazulv1.Objeto_Url:
		if s.LaunchApp == nil {
			return platform.ErrNoImplementado
		}
		return s.LaunchApp.Launch(ctx, platform.AppRef{URL: t.Url})
	case *lazulv1.Objeto_App:
		if s.LaunchApp == nil {
			return platform.ErrNoImplementado
		}
		return r.abrirApp(ctx, t.App)
	case *lazulv1.Objeto_Archivo:
		if r.Archivos == nil {
			return fmt.Errorf("%w: este equipo no recibe archivos", platform.ErrNoImplementado)
		}
		rec, err := r.Archivos.Buscar(t.Archivo.GetTransferenciaId(), t.Archivo.GetSha256())
		if err != nil {
			return fmt.Errorf("%w: %v", platform.ErrNoEncontrada, err)
		}
		if s.LaunchApp == nil {
			return nil // llegó; no hay cómo abrirlo
		}
		return s.LaunchApp.Launch(ctx, platform.AppRef{Archivo: rec.Ruta})
	}
	return fmt.Errorf("%w: objeto vacío", errInvalido)
}

// abrirApp abre la app del gesto 4 en este equipo: la del catálogo para
// este sistema si la conoce; si no, el nativo_id que llegó (solo cuando el
// catálogo no sabe de esa app); y si no está instalada, su web.
func (r *Receptor) abrirApp(ctx context.Context, ref *lazulv1.AppRef) error {
	nativo, web := ref.GetNativoId(), ref.GetWeb()
	if r.Catalogo != nil && ref.GetCatalogoId() != "" {
		if a, ok := r.Catalogo.Buscar(ref.GetCatalogoId()); ok {
			nativo = a.Nativo(r.so())
			if a.Web != "" {
				web = a.Web
			}
		}
	}
	err := fmt.Errorf("app %q: %w", ref.GetNombre(), platform.ErrNoEncontrada)
	if nativo != "" {
		err = r.Sistema.LaunchApp.Launch(ctx, platform.AppRef{ID: nativo})
	}
	if errors.Is(err, platform.ErrNoEncontrada) && web != "" {
		return r.Sistema.LaunchApp.Launch(ctx, platform.AppRef{URL: web})
	}
	return err
}

// ObjetoDeApp es el gesto 4 por niveles (dec-r2ii): la URL si es el
// navegador y se pudo leer; si no, el archivo abierto; si no, la app sin
// estado, con su id de catálogo y su web de respaldo si el catálogo la
// conoce en este sistema (so).
func ObjetoDeApp(app platform.App, cat *apps.Catalogo, so string) *lazulv1.Objeto {
	switch {
	case app.URL != "":
		return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Url{Url: app.URL}}
	case app.Archivo != "":
		return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Archivo{Archivo: &lazulv1.Archivo{Nombre: app.Archivo}}}
	}
	ref := &lazulv1.AppRef{NativoId: app.ID, Nombre: app.Nombre}
	if cat != nil {
		if a, ok := cat.PorNativo(so, app.ID); ok {
			ref.CatalogoId, ref.Nombre, ref.Web = a.ID, a.Nombre, a.Web
		}
	}
	return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_App{App: ref}}
}

func respuestaFalla(c lazulv1.Falla_Codigo, detalle string) *lazulv1.Sobre_Respuesta {
	return &lazulv1.Sobre_Respuesta{Respuesta: &lazulv1.Respuesta{Falla: &lazulv1.Falla{Codigo: c, Detalle: detalle}}}
}

// falla traduce los errores de platform a códigos del protocolo.
func falla(err error) *lazulv1.Respuesta {
	c := lazulv1.Falla_CODIGO_INTERNO
	switch {
	case errors.Is(err, platform.ErrNoImplementado):
		c = lazulv1.Falla_CODIGO_NO_SOPORTADO
	case errors.Is(err, platform.ErrNoEncontrada):
		c = lazulv1.Falla_CODIGO_NO_ENCONTRADO
	case errors.Is(err, platform.ErrRefInvalida), errors.Is(err, errInvalido):
		c = lazulv1.Falla_CODIGO_INVALIDO
	}
	return &lazulv1.Respuesta{Falla: &lazulv1.Falla{Codigo: c, Detalle: err.Error()}}
}
