package core

import (
	"context"
	"testing"

	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

func TestURLSegura(t *testing.T) {
	bien := []string{"https://lazul.dev", "http://192.168.1.5:8080/x?y=1"}
	mal := []string{"", "file:///etc/passwd", "ms-msdt:/id", "search-ms:query=x", "javascript:alert(1)",
		"calc.exe", "//lazul.dev", "https://", "https://usuario:clave@lazul.dev", "ftp://lazul.dev"}
	for _, s := range bien {
		if !urlSegura(s) {
			t.Errorf("rechazó %q", s)
		}
	}
	for _, s := range mal {
		if urlSegura(s) {
			t.Errorf("aceptó %q", s)
		}
	}
}

func TestArchivoAbrible(t *testing.T) {
	for _, n := range []string{"a.png", "B.PDF", "x.docx", "c.mp4"} {
		if !archivoAbrible(n) {
			t.Errorf("rechazó %q", n)
		}
	}
	for _, n := range []string{"a.exe", "a.bat", "a.lnk", "a.ps1", "a.sh", "a.desktop", "a.msi", "a.js", "a.docm", "a", "a.png.exe", ""} {
		if archivoAbrible(n) {
			t.Errorf("aceptó %q", n)
		}
	}
}

func entregarObjeto(r *Receptor, o *lazulv1.Objeto) *lazulv1.Respuesta {
	return r.Atender(context.Background(), orden(&lazulv1.Orden{Accion: &lazulv1.Orden_Entregar{Entregar: &lazulv1.Entregar{Objeto: o}}})).GetRespuesta()
}

func TestReceptorNoAbreURLQueNoEsWeb(t *testing.T) {
	f := falso.New("a")
	r := &Receptor{Sistema: f.Sistema()}
	resp := entregarObjeto(r, &lazulv1.Objeto{Tipo: &lazulv1.Objeto_Url{Url: "file:///etc/passwd"}})
	if resp.GetOk() || resp.GetFalla().GetCodigo() != lazulv1.Falla_CODIGO_INVALIDO {
		t.Fatalf("%v", resp)
	}
	if ab := f.Abiertas(); len(ab) != 0 {
		t.Fatalf("abrió %v", ab)
	}
}

func TestReceptorNoLanzaUnIdQueElCatalogoNoConoce(t *testing.T) {
	f := falso.New("a")
	r := &Receptor{Sistema: f.Sistema()}
	entregarObjeto(r, &lazulv1.Objeto{Tipo: &lazulv1.Objeto_App{App: &lazulv1.AppRef{
		NativoId: `C:\Windows\System32\cmd.exe`, Web: "https://lazul.dev"}}})
	ab := f.Abiertas()
	if len(ab) != 1 || ab[0].URL != "https://lazul.dev" || ab[0].ID != "" {
		t.Fatalf("abrió %v", ab)
	}
}
