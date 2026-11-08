// Package apps contiene el catalogo comun de aplicaciones de Lazul.
package apps

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

//go:embed apps.json
var datos []byte

// Catalogo es la raiz del formato apps.json v1.
type Catalogo struct {
	Version int   `json:"version"`
	Apps    []App `json:"apps"`
}

// App relaciona una aplicacion comun con sus identificadores por plataforma.
type App struct {
	ID      string                `json:"id"`
	Nombre  string                `json:"nombre"`
	Windows *IdentificadorWindows `json:"windows,omitempty"`
	Linux   string                `json:"linux,omitempty"`
	Android string                `json:"android,omitempty"`
	Web     string                `json:"web"`
}

// IdentificadorWindows distingue un ejecutable de un AUMID.
type IdentificadorWindows struct {
	Tipo string `json:"tipo"`
	ID   string `json:"id"`
}

// Cargar valida y devuelve el catalogo incluido en el binario.
func Cargar() (Catalogo, error) {
	return Validar(datos)
}

// Validar decodifica un catalogo v1 y comprueba su esquema y sus invariantes.
func Validar(contenido []byte) (Catalogo, error) {
	var catalogo Catalogo
	decodificador := json.NewDecoder(bytes.NewReader(contenido))
	decodificador.DisallowUnknownFields()
	if err := decodificador.Decode(&catalogo); err != nil {
		return Catalogo{}, fmt.Errorf("decodificar apps.json: %w", err)
	}
	if err := comprobarFin(decodificador); err != nil {
		return Catalogo{}, err
	}
	if catalogo.Version != 1 {
		return Catalogo{}, fmt.Errorf("version: se esperaba 1, se recibio %d", catalogo.Version)
	}
	if len(catalogo.Apps) == 0 {
		return Catalogo{}, errors.New("apps: debe contener al menos una aplicacion")
	}

	vistos := make(map[string]struct{}, len(catalogo.Apps))
	for indice, app := range catalogo.Apps {
		campo := fmt.Sprintf("apps[%d]", indice)
		if strings.TrimSpace(app.ID) == "" {
			return Catalogo{}, fmt.Errorf("%s.id: es obligatorio", campo)
		}
		if _, existe := vistos[app.ID]; existe {
			return Catalogo{}, fmt.Errorf("%s.id: %q esta duplicado", campo, app.ID)
		}
		vistos[app.ID] = struct{}{}
		if strings.TrimSpace(app.Nombre) == "" {
			return Catalogo{}, fmt.Errorf("%s.nombre: es obligatorio", campo)
		}
		if err := validarWeb(app.Web); err != nil {
			return Catalogo{}, fmt.Errorf("%s.web: %w", campo, err)
		}
		if app.Windows != nil {
			if app.Windows.Tipo != "ejecutable" && app.Windows.Tipo != "aumid" {
				return Catalogo{}, fmt.Errorf("%s.windows.tipo: debe ser ejecutable o aumid", campo)
			}
			if strings.TrimSpace(app.Windows.ID) == "" {
				return Catalogo{}, fmt.Errorf("%s.windows.id: es obligatorio cuando windows esta presente", campo)
			}
		}
		if strings.Contains(app.Linux, "/") || (app.Linux != "" && !strings.HasSuffix(app.Linux, ".desktop")) {
			return Catalogo{}, fmt.Errorf("%s.linux: debe ser un id de .desktop, no una ruta", campo)
		}
	}
	return catalogo, nil
}

func comprobarFin(decodificador *json.Decoder) error {
	var extra any
	if err := decodificador.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("apps.json: contiene datos despues del objeto principal")
		}
		return fmt.Errorf("decodificar apps.json: %w", err)
	}
	return nil
}

func validarWeb(valor string) error {
	if strings.TrimSpace(valor) == "" {
		return errors.New("es obligatoria")
	}
	destino, err := url.ParseRequestURI(valor)
	if err != nil || destino.Host == "" || (destino.Scheme != "https" && destino.Scheme != "http") {
		return errors.New("debe ser una URL http o https absoluta")
	}
	return nil
}
