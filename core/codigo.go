package core

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// El código de 6 dígitos que muestra el receptor al emparejar.
const (
	DuracionCodigo = 2 * time.Minute
	IntentosCodigo = 3
)

type resultadoCodigo int

const (
	resBien resultadoCodigo = iota
	resErroneo
	resCaducado // vencido, agotado o ya usado
)

// codigo vive en el receptor; nunca viaja por la red.
type codigo struct {
	digitos string
	vence   time.Time
	quedan  int
	gastado bool
}

func nuevoCodigo(ahora time.Time) (*codigo, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return nil, err
	}
	return &codigo{digitos: fmt.Sprintf("%06d", n.Int64()), vence: ahora.Add(DuracionCodigo), quedan: IntentosCodigo}, nil
}

// probar gasta un intento. coincide dice si la prueba recibida corresponde a
// estos dígitos. Un acierto gasta el código: sirve una sola vez.
func (c *codigo) probar(ahora time.Time, coincide func(digitos string) bool) (resultadoCodigo, int) {
	if c.gastado || c.quedan <= 0 || !ahora.Before(c.vence) {
		return resCaducado, 0
	}
	if coincide(c.digitos) {
		c.gastado = true
		return resBien, c.quedan
	}
	c.quedan--
	return resErroneo, c.quedan
}

// normalizarCodigo acepta espacios y guiones de quien lo escribe; nada más.
func normalizarCodigo(s string) (string, error) {
	s = strings.NewReplacer(" ", "", "-", "").Replace(s)
	if len(s) != 6 {
		return "", fmt.Errorf("el código tiene 6 dígitos")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return "", fmt.Errorf("el código tiene solo dígitos")
		}
	}
	return s, nil
}
