// Package core es el núcleo de Lazul: recibe Sobres, decide y responde
// usando solo las interfaces de platform. Nunca llama al sistema.
//
// Hoy tiene el receptor (atiende las Órdenes de un controlador), el
// transporte por WebSocket sobre TLS 1.3 con huella fijada y el
// emparejamiento con código de 6 dígitos y permiso de la persona (pareja.go);
// el descubrimiento mDNS está en core/descubre. Falta el enrutador del
// controlador y LocalSend (cotejo-sb2kdm).
package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Receptor atiende las Órdenes que llegan a este equipo.
type Receptor struct {
	Sistema platform.Sistema
	Hola    *lazulv1.Hola
	// Reloj se puede cambiar en pruebas.
	Reloj func() time.Time
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
		resp.Contenido = &lazulv1.Sobre_Respuesta{Respuesta: r.orden(ctx, c.Orden)}
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
		// Gesto 4 por niveles (dec-r2ii): URL, archivo o app sin estado.
		switch {
		case app.URL != "":
			return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Url{Url: app.URL}}, nil
		case app.Archivo != "":
			return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Archivo{Archivo: &lazulv1.Archivo{Nombre: app.Archivo}}}, nil
		}
		return &lazulv1.Objeto{Tipo: &lazulv1.Objeto_App{App: &lazulv1.AppRef{NativoId: app.ID, Nombre: app.Nombre}}}, nil
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
		// El catálogo (cotejo-qxb94p) traducirá catalogo_id a nativo_id.
		err := s.LaunchApp.Launch(ctx, platform.AppRef{ID: t.App.GetNativoId()})
		if errors.Is(err, platform.ErrNoEncontrada) && t.App.GetWeb() != "" {
			return s.LaunchApp.Launch(ctx, platform.AppRef{URL: t.App.GetWeb()})
		}
		return err
	case *lazulv1.Objeto_Archivo:
		return fmt.Errorf("%w: recibir archivos llega con LocalSend", platform.ErrNoImplementado)
	}
	return fmt.Errorf("%w: objeto vacío", errInvalido)
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
