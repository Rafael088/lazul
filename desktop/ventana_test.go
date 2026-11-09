package desktop

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLeerEstadoVentanaInexistenteUsaInicial(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "no-existe", NombreEstadoVentana)
	e, err := LeerEstadoVentana(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if e != VentanaInicial() {
		t.Fatalf("estado = %+v; quiero %+v", e, VentanaInicial())
	}
}

func TestEstadoVentanaSeGuardaYSeLee(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "estado", NombreEstadoVentana)
	quiero := EstadoVentana{Ancho: 712, Alto: 503, X: -820, Y: 44, TienePosicion: true}
	if err := GuardarEstadoVentana(ruta, quiero); err != nil {
		t.Fatal(err)
	}
	tengo, err := LeerEstadoVentana(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tengo, quiero) {
		t.Fatalf("estado = %+v; quiero %+v", tengo, quiero)
	}
	info, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos = %o; quiero 600", info.Mode().Perm())
	}
}

func TestLeerEstadoVentanaInvalidoNoLoAplica(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreEstadoVentana)
	if err := os.WriteFile(ruta, []byte(`{"ancho":100,"alto":80}`), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := LeerEstadoVentana(ruta)
	if err == nil {
		t.Fatal("faltó el error del tamaño inválido")
	}
	if e != VentanaInicial() {
		t.Fatalf("estado = %+v; quiero el inicial %+v", e, VentanaInicial())
	}
}

func TestGuardarEstadoVentanaRechazaTamanoInvalido(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), NombreEstadoVentana)
	if err := GuardarEstadoVentana(ruta, EstadoVentana{Ancho: 1, Alto: 1}); err == nil {
		t.Fatal("faltó el error del tamaño inválido")
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("el archivo no debía existir: %v", err)
	}
}
