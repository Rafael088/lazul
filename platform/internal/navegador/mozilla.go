package navegador

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// Firefox y sus derivados (Zen, LibreWolf, Floorp...) guardan la sesión en
// <perfil>/sessionstore-backups/recovery.jsonlz4: JSON comprimido con LZ4
// ("mozLz4"). Lo reescriben como mucho cada 15 s (browser.sessionstore.
// interval), así que puede ir atrasado; por eso solo se cree la URL de una
// pestaña cuyo título guardado es el de la ventana. Las ventanas privadas no
// se guardan: de ellas nunca sale URL.

// ArchivoSesionMozilla es la ruta de la sesión dentro del perfil.
const ArchivoSesionMozilla = "sessionstore-backups/recovery.jsonlz4"

// MaxSesion limita lo que se descomprime (una sesión grande ronda pocos MB).
const MaxSesion = 256 << 20

var magiaMozLz4 = []byte("mozLz40\x00")

// DescomprimirMozLz4 abre un archivo .jsonlz4 de Mozilla.
func DescomprimirMozLz4(b []byte) ([]byte, error) {
	if len(b) < 12 || !bytes.Equal(b[:8], magiaMozLz4) {
		return nil, errors.New("mozlz4: no es un archivo mozLz4")
	}
	tam := int(binary.LittleEndian.Uint32(b[8:12]))
	if tam > MaxSesion {
		return nil, fmt.Errorf("mozlz4: %d bytes descomprimidos, más de %d", tam, MaxSesion)
	}
	return lz4Bloque(b[12:], tam)
}

// lz4Bloque descomprime un bloque LZ4 (sin marco) de tam bytes.
func lz4Bloque(src []byte, tam int) ([]byte, error) {
	dst := make([]byte, 0, tam)
	errCorto := errors.New("lz4: bloque cortado")
	largo := func(i int, n int) (int, int, error) {
		if n != 15 {
			return n, i, nil
		}
		for {
			if i >= len(src) {
				return 0, i, errCorto
			}
			b := src[i]
			i++
			n += int(b)
			if b != 255 {
				return n, i, nil
			}
		}
	}
	for i := 0; i < len(src); {
		tok := src[i]
		i++
		lit, j, err := largo(i, int(tok>>4))
		if err != nil {
			return nil, err
		}
		i = j
		if lit > len(src)-i || len(dst)+lit > tam {
			return nil, errCorto
		}
		dst = append(dst, src[i:i+lit]...)
		i += lit
		if i == len(src) {
			break // la última secuencia solo trae literales
		}
		if i+2 > len(src) {
			return nil, errCorto
		}
		off := int(src[i]) | int(src[i+1])<<8
		i += 2
		if off == 0 || off > len(dst) {
			return nil, errors.New("lz4: desplazamiento inválido")
		}
		ml, j, err := largo(i, int(tok&15))
		if err != nil {
			return nil, err
		}
		i = j
		ml += 4
		if len(dst)+ml > tam {
			return nil, errors.New("lz4: más grande de lo anunciado")
		}
		ini := len(dst) - off
		for k := 0; k < ml; k++ { // byte a byte: la copia puede solaparse
			dst = append(dst, dst[ini+k])
		}
	}
	if len(dst) != tam {
		return nil, fmt.Errorf("lz4: %d bytes, se anunciaron %d", len(dst), tam)
	}
	return dst, nil
}

type sesionMozilla struct {
	Ventanas []struct {
		Pestanas []struct {
			Entradas []struct {
				URL    string `json:"url"`
				Titulo string `json:"title"`
			} `json:"entries"`
			Indice int `json:"index"` // desde 1
		} `json:"tabs"`
		Elegida int `json:"selected"` // desde 1
	} `json:"windows"`
}

// URLMozilla devuelve la URL de la pestaña que se ve en la ventana con ese
// título, a partir de la sesión ya descomprimida; "" si no se sabe.
//
// Primero mira la pestaña elegida de cada ventana; si ninguna tiene el título
// de la ventana (la sesión va atrasada tras cambiar de pestaña), cualquier
// pestaña con ese título. Si dos pestañas con el título tienen URL distinta,
// no se adivina.
func URLMozilla(sesion []byte, tituloVentana string) string {
	var s sesionMozilla
	if err := json.Unmarshal(sesion, &s); err != nil {
		return ""
	}
	var elegidas, todas []string
	for _, v := range s.Ventanas {
		for pi, p := range v.Pestanas {
			if len(p.Entradas) == 0 {
				continue
			}
			i := p.Indice - 1
			if i < 0 || i >= len(p.Entradas) {
				i = len(p.Entradas) - 1
			}
			e := p.Entradas[i]
			if !CoincideTitulo(tituloVentana, e.Titulo) {
				continue
			}
			todas = append(todas, e.URL)
			if pi == v.Elegida-1 {
				elegidas = append(elegidas, e.URL)
			}
		}
	}
	if len(elegidas) > 0 {
		return unica(elegidas)
	}
	return unica(todas)
}
