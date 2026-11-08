package core

import (
	"context"
	"testing"

	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/platform/vacio"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

func orden(o *lazulv1.Orden) *lazulv1.Sobre {
	return &lazulv1.Sobre{Version: 1, Id: "x", Contenido: &lazulv1.Sobre_Orden{Orden: o}}
}

func tomar(q lazulv1.Tomar_Que) *lazulv1.Sobre {
	return orden(&lazulv1.Orden{Accion: &lazulv1.Orden_Tomar{Tomar: &lazulv1.Tomar{Que: q}}})
}

func TestVersionDesconocida(t *testing.T) {
	r := &Receptor{Sistema: falso.New("a").Sistema()}
	resp := r.Atender(context.Background(), &lazulv1.Sobre{Version: 99, Id: "x"})
	if got := resp.GetRespuesta().GetFalla().GetCodigo(); got != lazulv1.Falla_CODIGO_INVALIDO {
		t.Fatalf("código %v", got)
	}
}

func TestSinCapacidadEsNoSoportado(t *testing.T) {
	r := &Receptor{Sistema: vacio.New()}
	for _, q := range []lazulv1.Tomar_Que{lazulv1.Tomar_QUE_CAPTURA, lazulv1.Tomar_QUE_PORTAPAPELES, lazulv1.Tomar_QUE_APP_ACTIVA} {
		resp := r.Atender(context.Background(), tomar(q)).GetRespuesta()
		if resp.GetOk() || resp.GetFalla().GetCodigo() != lazulv1.Falla_CODIGO_NO_SOPORTADO {
			t.Errorf("%v: %v", q, resp)
		}
	}
}

func TestAppAusenteSinWebEsNoEncontrado(t *testing.T) {
	r := &Receptor{Sistema: falso.New("a").Sistema()}
	o := &lazulv1.Objeto{Tipo: &lazulv1.Objeto_App{App: &lazulv1.AppRef{NativoId: "nada"}}}
	resp := r.Atender(context.Background(), orden(&lazulv1.Orden{Accion: &lazulv1.Orden_Entregar{Entregar: &lazulv1.Entregar{Objeto: o}}})).GetRespuesta()
	if resp.GetFalla().GetCodigo() != lazulv1.Falla_CODIGO_NO_ENCONTRADO {
		t.Fatalf("%v", resp)
	}
}

func TestOrdenVaciaEsInvalida(t *testing.T) {
	r := &Receptor{Sistema: falso.New("a").Sistema()}
	resp := r.Atender(context.Background(), orden(&lazulv1.Orden{})).GetRespuesta()
	if resp.GetFalla().GetCodigo() != lazulv1.Falla_CODIGO_INVALIDO {
		t.Fatalf("%v", resp)
	}
}
