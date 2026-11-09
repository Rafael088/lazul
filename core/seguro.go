package core

import (
	"net/url"
	"path/filepath"
	"strings"
)

// Lo que llega de otro equipo se abre aquí, así que no todo se abre. Un
// equipo emparejado puede pedir "abre esta URL", "abre este archivo" o "abre
// esta app", pero no "ejecuta esto": las URL son solo web, los archivos solo
// de un tipo de documento, imagen o medio, y las apps solo las del catálogo.

// urlSegura: solo http y https, con host, sin credenciales dentro de la URL.
// Cualquier otro esquema (file:, ms-msdt:, search-ms:, javascript:, etc.) lo
// resuelve el sistema con un programa que el otro equipo no debería elegir.
func urlSegura(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// extensionesAbribles son los tipos de archivo que Lazul abre por orden de
// otro equipo: documentos, imágenes, audio y video. Lo demás llega a la
// bandeja y se queda ahí, sin abrirse (ejecutables, scripts, accesos
// directos, instaladores, macros...).
var extensionesAbribles = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true,
	".pdf": true, ".txt": true, ".md": true, ".csv": true, ".rtf": true,
	".odt": true, ".ods": true, ".odp": true,
	".docx": true, ".xlsx": true, ".pptx": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true,
	".mp4": true, ".webm": true, ".mkv": true, ".mov": true,
}

// archivoAbrible dice si se puede abrir el archivo con ese nombre.
func archivoAbrible(ruta string) bool {
	return extensionesAbribles[strings.ToLower(filepath.Ext(ruta))]
}
