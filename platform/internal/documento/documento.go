// Package documento elige el documento abierto en una ventana (nivel 2 del
// gesto 4, dec-r2ii) entre las rutas que el adaptador encuentra (las del
// título, los argumentos del proceso, sus archivos abiertos).
//
// La regla para que sea fiable: el nombre del archivo tiene que salir en el
// título de la ventana (casi todos los visores y editores lo ponen ahí), y
// tiene que ser un documento, una imagen o un medio, los mismos tipos que el
// receptor acepta abrir (core/seguro.go). Lo demás (código, configuración,
// bases de datos del programa) no se manda: cae al nivel de la app.
package documento

import (
	"path/filepath"
	"strings"
)

// extensiones: las mismas que extensionesAbribles en core/seguro.go.
var extensiones = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true,
	".pdf": true, ".txt": true, ".md": true, ".csv": true, ".rtf": true,
	".odt": true, ".ods": true, ".odp": true,
	".docx": true, ".xlsx": true, ".pptx": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true,
	".mp4": true, ".webm": true, ".mkv": true, ".mov": true,
}

// Es dice si la ruta es de un documento, una imagen o un medio.
func Es(ruta string) bool { return extensiones[strings.ToLower(filepath.Ext(ruta))] }

// separadores con que las apps pegan el documento y su nombre en el título:
// "informe - Word", "foto.png — Visor".
var separadores = []string{" - ", " — ", " – ", " | "}

// enTitulo dice si el documento de esa ruta sale en el título: su nombre
// completo, o el nombre sin extensión (Word, Excel) seguido del nombre de la
// app o al final.
func enTitulo(titulo, ruta string) bool {
	base := nombreBase(ruta)
	if base == "" {
		return false
	}
	if strings.Contains(titulo, base) {
		return true
	}
	tallo := strings.TrimSuffix(base, filepath.Ext(base))
	if len([]rune(tallo)) < 3 {
		return false
	}
	t := strings.TrimSpace(strings.TrimLeft(titulo, "*● "))
	if t == tallo {
		return true
	}
	for _, s := range separadores {
		if strings.HasPrefix(t, tallo+s) || strings.HasSuffix(t, s+tallo) {
			return true
		}
	}
	return false
}

// nombreBase sirve para rutas de Linux y de Windows en cualquier sistema.
func nombreBase(ruta string) string {
	if i := strings.LastIndexAny(ruta, `/\`); i >= 0 {
		return ruta[i+1:]
	}
	return ruta
}

// Elegir devuelve la ruta del documento que se ve en la ventana con ese
// título, o "" si ninguna candidata cumple la regla o si hay dos documentos
// distintos que podrían ser. Las candidatas ya existen y son absolutas.
// Si varias cumplen, gana la de nombre más largo ("informe-final.pdf" sobre
// "final.pdf" en el título "informe-final.pdf - Visor").
func Elegir(titulo string, candidatas []string) string {
	mejor, largo, empate := "", -1, false
	vistas := map[string]bool{}
	for _, c := range candidatas {
		if c == "" || vistas[c] || !Es(c) || !enTitulo(titulo, c) {
			continue
		}
		vistas[c] = true
		n := len(nombreBase(c))
		switch {
		case n > largo:
			mejor, largo, empate = c, n, false
		case n == largo:
			empate = true
		}
	}
	if empate {
		return ""
	}
	return mejor
}

// RutasEnTitulo saca del título las rutas absolutas de documentos que
// existen (imv, mpv o los editores ponen la ruta completa). existe dice si la
// ruta es un archivo normal.
func RutasEnTitulo(titulo string, existe func(string) bool) []string {
	var inicios, finales []int
	for i := 0; i < len(titulo); i++ {
		antes := i == 0 || strings.ContainsRune(` ([{"'«:`, rune(titulo[i-1]))
		if !antes {
			continue
		}
		r := titulo[i:]
		if strings.HasPrefix(r, "/") || len(r) >= 3 && r[1] == ':' && (r[2] == '\\' || r[2] == '/') &&
			(r[0] >= 'A' && r[0] <= 'Z' || r[0] >= 'a' && r[0] <= 'z') {
			inicios = append(inicios, i)
		}
	}
	if len(inicios) == 0 {
		return nil
	}
	minus := strings.ToLower(titulo)
	for ext := range extensiones {
		for desde := 0; ; {
			j := strings.Index(minus[desde:], ext)
			if j < 0 {
				break
			}
			fin := desde + j + len(ext)
			if fin == len(minus) || !esLetraODigito(minus[fin]) {
				finales = append(finales, fin)
			}
			desde = desde + j + 1
		}
	}
	var rutas []string
	for _, i := range inicios {
		// la ruta más larga que existe desde este inicio
		hallada := ""
		for _, f := range finales {
			if f > i && f-i > len(hallada) && existe(titulo[i:f]) {
				hallada = titulo[i:f]
			}
		}
		if hallada != "" {
			rutas = append(rutas, hallada)
		}
	}
	return rutas
}

func esLetraODigito(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_'
}
