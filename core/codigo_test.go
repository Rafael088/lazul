package core

import (
	"testing"
	"time"
)

func TestCodigoSonSeisDigitos(t *testing.T) {
	vistos := map[string]bool{}
	for range 50 {
		c, err := nuevoCodigo(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(c.digitos) != 6 {
			t.Fatalf("%q no tiene 6 dígitos", c.digitos)
		}
		for _, r := range c.digitos {
			if r < '0' || r > '9' {
				t.Fatalf("%q no es numérico", c.digitos)
			}
		}
		vistos[c.digitos] = true
	}
	if len(vistos) < 45 {
		t.Fatalf("solo %d códigos distintos en 50: ¿no es aleatorio?", len(vistos))
	}
}

func TestCodigoErroneoAgotaLosIntentos(t *testing.T) {
	ahora := time.Now()
	c, _ := nuevoCodigo(ahora)
	mal := func(string) bool { return false }
	for i := IntentosCodigo - 1; i >= 0; i-- {
		res, quedan := c.probar(ahora, mal)
		if res != resErroneo || quedan != i {
			t.Fatalf("intento con %d restantes: %v, quedan %d", i, res, quedan)
		}
	}
	// Agotado: ni el código bueno entra.
	if res, _ := c.probar(ahora, func(string) bool { return true }); res != resCaducado {
		t.Fatalf("tras agotar los intentos: %v", res)
	}
}

func TestCodigoCaducado(t *testing.T) {
	ahora := time.Now()
	c, _ := nuevoCodigo(ahora)
	if res, _ := c.probar(ahora.Add(DuracionCodigo+time.Second), func(string) bool { return true }); res != resCaducado {
		t.Fatalf("vencido: %v", res)
	}
}

func TestCodigoSirveUnaSolaVez(t *testing.T) {
	ahora := time.Now()
	c, _ := nuevoCodigo(ahora)
	bien := func(d string) bool { return d == c.digitos }
	if res, _ := c.probar(ahora, bien); res != resBien {
		t.Fatalf("primer uso: %v", res)
	}
	if res, _ := c.probar(ahora, bien); res != resCaducado {
		t.Fatalf("segundo uso: %v", res)
	}
}

func TestNormalizarCodigo(t *testing.T) {
	for entra, sale := range map[string]string{"123456": "123456", " 123 456 ": "123456", "123-456": "123456"} {
		if got, err := normalizarCodigo(entra); err != nil || got != sale {
			t.Errorf("%q: %q %v", entra, got, err)
		}
	}
	for _, malo := range []string{"", "12345", "1234567", "12a456", "１２３４５６"} {
		if _, err := normalizarCodigo(malo); err == nil {
			t.Errorf("%q debería ser inválido", malo)
		}
	}
}
