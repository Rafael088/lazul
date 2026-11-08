package core

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// Par es un equipo emparejado: su huella fijada y cómo se llama.
type Par struct {
	Huella   Huella    `json:"-"`
	EquipoID string    `json:"equipo_id"`
	Nombre   string    `json:"nombre"`
	Desde    time.Time `json:"desde"`
}

type parJSON struct {
	Huella string `json:"huella"`
	Par
}

// Pares son los equipos en los que este confía. Con ruta, cada cambio se
// guarda en disco (JSON, 0600); sin ruta viven en memoria (sim y pruebas).
type Pares struct {
	ruta string
	mu   sync.Mutex
	m    map[Huella]Par
}

func NuevosPares() *Pares { return &Pares{m: map[Huella]Par{}} }

func AbrirPares(ruta string) (*Pares, error) {
	p := &Pares{ruta: ruta, m: map[Huella]Par{}}
	b, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	var lista []parJSON
	if err := json.Unmarshal(b, &lista); err != nil {
		return nil, err
	}
	for _, pj := range lista {
		h, err := ParsearHuella(pj.Huella)
		if err != nil {
			return nil, err
		}
		pj.Par.Huella = h
		p.m[h] = pj.Par
	}
	return p, nil
}

func (p *Pares) Buscar(h Huella) (Par, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	par, ok := p.m[h]
	return par, ok
}

func (p *Pares) Fijar(par Par) error {
	if par.Huella.cero() {
		return errors.New("par sin huella")
	}
	if par.Desde.IsZero() {
		par.Desde = time.Now()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[par.Huella] = par
	return p.guardar()
}

// Olvidar quita la confianza: el equipo vuelve a ser un desconocido y sus
// conexiones abiertas se cortan en el siguiente mensaje.
func (p *Pares) Olvidar(h Huella) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, h)
	return p.guardar()
}

func (p *Pares) Lista() []Par {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := make([]Par, 0, len(p.m))
	for _, par := range p.m {
		l = append(l, par)
	}
	slices.SortFunc(l, func(a, b Par) int { return strings.Compare(a.Huella.String(), b.Huella.String()) })
	return l
}

func (p *Pares) guardar() error {
	if p.ruta == "" {
		return nil
	}
	l := make([]parJSON, 0, len(p.m))
	for h, par := range p.m {
		l = append(l, parJSON{Huella: h.String(), Par: par})
	}
	slices.SortFunc(l, func(a, b parJSON) int { return strings.Compare(a.Huella, b.Huella) })
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return escribirPrivado(p.ruta, b)
}
