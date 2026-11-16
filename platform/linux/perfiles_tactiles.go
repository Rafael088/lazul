//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"strings"
)

// perfilTactil dice cómo se traduce cada gesto en la app que tiene el foco.
// La misma intención («bajar», «siguiente») pide teclas y cantidades
// distintas: una flecha en Evince baja bastante, en un navegador casi nada.
type perfilTactil struct {
	nombre      string
	scrollVeces int    // flechas por gesto de scroll
	navAtras    string // tecla del barrido hacia atrás; "" = según el modo
	navAdelante string
}

var (
	perfilNavegador = perfilTactil{nombre: "navegador", scrollVeces: 8, navAtras: "Left", navAdelante: "Right"}
	perfilPDF       = perfilTactil{nombre: "pdf", scrollVeces: 3, navAtras: "Page_Up", navAdelante: "Page_Down"}
	perfilDiapos    = perfilTactil{nombre: "diapositivas", scrollVeces: 1, navAtras: "Page_Up", navAdelante: "Page_Down"}
	perfilGeneral   = perfilTactil{nombre: "general", scrollVeces: ScrollPulsaciones}
)

// clasesTactiles: fragmento de la clase de ventana (en minúsculas) → perfil.
// Se recorre en orden: el primero que aparezca en la clase gana.
var clasesTactiles = []struct {
	fragmento string
	perfil    perfilTactil
}{
	{"libreoffice", perfilDiapos}, {"soffice", perfilDiapos}, {"impress", perfilDiapos},
	{"onlyoffice", perfilDiapos}, {"wps", perfilDiapos},
	{"evince", perfilPDF}, {"papers", perfilPDF}, {"okular", perfilPDF}, {"zathura", perfilPDF},
	{"sioyek", perfilPDF}, {"atril", perfilPDF}, {"xreader", perfilPDF}, {"pdf", perfilPDF},
	{"zen", perfilNavegador}, {"firefox", perfilNavegador}, {"librewolf", perfilNavegador},
	{"chrom", perfilNavegador}, {"brave", perfilNavegador}, {"edge", perfilNavegador},
	{"vivaldi", perfilNavegador}, {"opera", perfilNavegador}, {"web", perfilNavegador},
}

func perfilDeClase(clase string) perfilTactil {
	c := strings.ToLower(clase)
	for _, e := range clasesTactiles {
		if strings.Contains(c, e.fragmento) {
			return e.perfil
		}
	}
	return perfilGeneral
}

// perfil pregunta a Hyprland qué ventana tiene el foco. Si no puede saberlo,
// usa el perfil general: mejor un scroll corto que ninguno.
func (a accionesTactiles) perfil(ctx context.Context) perfilTactil {
	if ctx.Err() != nil {
		return perfilGeneral // la acción tampoco se enviará
	}
	ejecutar := a.ejecutar
	if ejecutar == nil {
		ejecutar = ejecutarTactilReal
	}
	salida, err := ejecutar(ctx, "hyprctl", "activewindow", "-j")
	if err != nil {
		return perfilGeneral
	}
	var v struct {
		Class string `json:"class"`
	}
	if json.Unmarshal(salida, &v) != nil {
		return perfilGeneral
	}
	return perfilDeClase(v.Class)
}
