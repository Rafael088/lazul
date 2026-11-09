// Package navegador lee la URL de la pestaña que se ve en un navegador, a
// partir de lo que el propio navegador ya guarda en el disco (Linux) o de lo
// que muestra en la barra de direcciones (Windows). Es el nivel 1 del gesto 4
// (dec-r2ii).
//
// Todo es de solo lectura y sin tocar la configuración del navegador: ni
// depuración remota, ni puertos, ni extensiones. Si no se puede saber con
// seguridad qué URL se está viendo, las funciones devuelven "" y el gesto
// cae al nivel siguiente. Nada de lo leído se registra: la URL puede llevar
// tokens o datos personales.
//
// El paquete no depende del sistema: los adaptadores (platform/linux,
// platform/windows) le pasan los bytes y el título de la ventana.
package navegador

import (
	"net/url"
	"path/filepath"
	"strings"
)

// separadores con que los navegadores pegan el título de la pestaña y su
// nombre en el título de la ventana: "Página - Chromium", "Página — Mozilla
// Firefox", "Página - Personal: Microsoft Edge".
var separadores = []string{" - ", " — ", " – "}

// CoincideTitulo dice si el título de la ventana es el de esa pestaña: igual,
// o la pestaña seguida del nombre del navegador.
func CoincideTitulo(ventana, pestana string) bool {
	ventana, pestana = strings.TrimSpace(ventana), strings.TrimSpace(pestana)
	if pestana == "" {
		return false
	}
	if ventana == pestana {
		return true
	}
	for _, s := range separadores {
		if strings.HasPrefix(ventana, pestana+s) {
			return true
		}
	}
	return false
}

// unica devuelve la URL si todas las candidatas son la misma; si no, "".
func unica(urls []string) string {
	u := ""
	for _, c := range urls {
		switch {
		case c == "":
		case u == "":
			u = c
		case u != c:
			return ""
		}
	}
	return u
}

// DeBarra convierte lo que muestra la barra de direcciones en una URL
// absoluta, o "" si no parece una URL. Chrome, Edge y Firefox esconden el
// "https://" (y Chrome también el "http://"): sin esquema se supone https.
// Lo que tenga espacios es una búsqueda a medio escribir, no una URL.
func DeBarra(valor string) string {
	v := strings.TrimSpace(valor)
	if v == "" || strings.ContainsAny(v, " \t\r\n") {
		return ""
	}
	if strings.Contains(v, "://") {
		if u, err := url.Parse(v); err == nil && len(u.Scheme) > 1 {
			return v
		}
		return ""
	}
	if strings.HasPrefix(strings.ToLower(v), "about:") {
		return v
	}
	host := v
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if host == "" || (!strings.Contains(host, ".") && !strings.HasPrefix(host, "localhost") && !strings.HasPrefix(host, "[")) {
		return ""
	}
	u, err := url.Parse("https://" + v)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.String()
}

// RutaDeURL: si la URL es file:// de este equipo, la ruta del archivo que
// muestra (un PDF abierto en el navegador es un documento, no una página);
// si no, "".
func RutaDeURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") {
		return ""
	}
	ruta := u.Path
	// file:///C:/x en Windows: la ruta llega como "/C:/x".
	if len(ruta) >= 3 && ruta[0] == '/' && ruta[2] == ':' {
		ruta = filepath.FromSlash(ruta[1:])
	}
	return ruta
}
