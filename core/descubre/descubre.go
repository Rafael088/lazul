// Package descubre encuentra los equipos Lazul de la red local.
//
// Red es la interfaz: MDNS la cumple con mDNS de verdad (servicio
// _lazul._tcp, github.com/hashicorp/mdns) y Memoria con un tablero en el
// proceso, para ./dev sim y pruebas que no dependen de la red.
//
// Lo anunciado no es de confianza: cualquiera en la red puede anunciar
// cualquier cosa. Sirve para encontrar la dirección; la confianza la da la
// huella TLS fijada al emparejar (core.Emparejar la compara con la anunciada
// y core.Conectar con la fijada).
package descubre

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Servicio es el tipo DNS-SD que anuncian los receptores.
const Servicio = "_lazul._tcp"

// Anuncio: lo que un receptor dice de sí mismo.
type Anuncio struct {
	EquipoID string
	Nombre   string
	Huella   string // hex, SHA-256 de la clave pública (core.Huella)
	Dir      string // host:puerto donde escucha
}

// Red anuncia este equipo y busca los demás.
type Red interface {
	// Anunciar publica a hasta que se llame a la función devuelta.
	Anunciar(a Anuncio) (detener func(), err error)
	// Buscar devuelve lo que se oyó hasta que ctx termine (con plazo).
	Buscar(ctx context.Context) ([]Anuncio, error)
}

// txt y deTxt: el registro TXT de DNS-SD (RFC 6763 §6), clave=valor.
func txt(a Anuncio) []string {
	return []string{"v=1", "id=" + a.EquipoID, "nombre=" + a.Nombre, "huella=" + a.Huella}
}

func deTxt(campos []string, host string, puerto int) (Anuncio, error) {
	a := Anuncio{Dir: net.JoinHostPort(host, strconv.Itoa(puerto))}
	v := ""
	for _, c := range campos {
		k, val, _ := strings.Cut(c, "=")
		switch k {
		case "v":
			v = val
		case "id":
			a.EquipoID = val
		case "nombre":
			a.Nombre = val
		case "huella":
			a.Huella = val
		}
	}
	if v != "1" || a.EquipoID == "" || len(a.Huella) != 64 || puerto <= 0 || host == "" {
		return Anuncio{}, fmt.Errorf("anuncio incompleto: %q en %s:%d", campos, host, puerto)
	}
	return a, nil
}

// Memoria: tablero compartido en el proceso. El valor cero sirve.
type Memoria struct {
	mu sync.Mutex
	m  map[int]Anuncio
	n  int
}

func (r *Memoria) Anunciar(a Anuncio) (func(), error) {
	// Pasa por el mismo TXT que mDNS para que las pruebas cubran el formato.
	host, p, err := net.SplitHostPort(a.Dir)
	if err != nil {
		return nil, err
	}
	puerto, _ := strconv.Atoi(p)
	if _, err := deTxt(txt(a), host, puerto); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = map[int]Anuncio{}
	}
	r.n++
	k := r.n
	r.m[k] = a
	return func() { r.mu.Lock(); delete(r.m, k); r.mu.Unlock() }, nil
}

func (r *Memoria) Buscar(ctx context.Context) ([]Anuncio, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l := make([]Anuncio, 0, len(r.m))
	for _, a := range r.m {
		l = append(l, a)
	}
	slices.SortFunc(l, func(a, b Anuncio) int { return strings.Compare(a.Nombre, b.Nombre) })
	return l, nil
}
