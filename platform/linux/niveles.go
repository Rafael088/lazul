//go:build linux

package linux

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Rafael088/lazul/platform/internal/documento"
	"github.com/Rafael088/lazul/platform/internal/navegador"
)

// Niveles 1 y 2 del gesto 4 (dec-r2ii) en Linux: la URL de la pestaña que se
// ve y el documento abierto. hyprctl solo da clase, título y PID; lo demás
// sale de lo que el proceso de la ventana tiene abierto (/proc/<pid>/fd), sin
// tocar la configuración del navegador:
//
//   - Firefox y derivados (Zen...): tienen abierto <perfil>/places.sqlite; la
//     URL sale de <perfil>/sessionstore-backups/recovery.jsonlz4, que el
//     navegador reescribe como mucho cada 15 s.
//   - Chromium y derivados (Chrome, Edge, Brave...): tienen abierto
//     <perfil>/Sessions/Session_<n>, que escriben con unos 2,5 s de retraso.
//   - Documento: una ruta del título, de los argumentos del proceso o de sus
//     archivos abiertos, cuyo nombre sale en el título (platform/internal/documento).
//
// Cualquier fallo deja el nivel vacío y el gesto cae al siguiente.

// maxLeer limita lo que se lee de un archivo de sesión.
const maxLeer = 64 << 20

const raizProc = "/proc"

// niveles devuelve la URL o el documento de la ventana del proceso pid.
func niveles(pid int, titulo string) (url, archivo string) {
	if pid <= 0 {
		return "", ""
	}
	abiertos := archivosAbiertos(pid)
	if u := urlNavegador(abiertos, titulo); u != "" {
		// Un PDF abierto en el navegador es un documento, no una página.
		if r := navegador.RutaDeURL(u); r != "" {
			if documento.Es(r) && esArchivo(r) {
				return "", r
			}
			return "", ""
		}
		return u, ""
	}
	return "", documentoAbierto(pid, titulo, abiertos)
}

// archivosAbiertos: a qué apunta cada descriptor del proceso.
func archivosAbiertos(pid int) []string {
	dir := filepath.Join(raizProc, strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	rutas := make([]string, 0, len(fds))
	for _, fd := range fds {
		r, err := os.Readlink(filepath.Join(dir, fd.Name()))
		if err != nil || !strings.HasPrefix(r, "/") || strings.HasSuffix(r, " (deleted)") {
			continue
		}
		rutas = append(rutas, r)
	}
	return rutas
}

func urlNavegador(abiertos []string, titulo string) string {
	var urls []string
	vistos := map[string]bool{}
	for _, r := range abiertos {
		if vistos[r] {
			continue
		}
		vistos[r] = true
		switch {
		case filepath.Base(r) == "places.sqlite":
			datos := leer(filepath.Join(filepath.Dir(r), navegador.ArchivoSesionMozilla))
			if datos == nil {
				continue
			}
			if json, err := navegador.DescomprimirMozLz4(datos); err == nil {
				urls = append(urls, navegador.URLMozilla(json, titulo))
			}
		case strings.Contains(r, navegador.MarcaSesionChromium):
			if datos := leer(r); datos != nil {
				urls = append(urls, navegador.URLChromium(datos, titulo))
			}
		}
	}
	// Varios perfiles abiertos a la vez: solo si todos dicen lo mismo.
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

func leer(ruta string) []byte {
	f, err := os.Open(ruta)
	if err != nil {
		return nil
	}
	defer f.Close()
	datos, err := io.ReadAll(io.LimitReader(f, maxLeer+1))
	if err != nil || len(datos) > maxLeer {
		return nil
	}
	return datos
}

func esArchivo(ruta string) bool {
	info, err := os.Stat(ruta)
	return err == nil && info.Mode().IsRegular()
}

// documentoAbierto busca el documento entre las rutas del título, los
// argumentos del proceso y sus archivos abiertos.
func documentoAbierto(pid int, titulo string, abiertos []string) string {
	candidatas := documento.RutasEnTitulo(titulo, esArchivo)
	proc := filepath.Join(raizProc, strconv.Itoa(pid))
	cwd, _ := os.Readlink(filepath.Join(proc, "cwd"))
	if linea, err := os.ReadFile(filepath.Join(proc, "cmdline")); err == nil {
		args := bytes.Split(bytes.TrimRight(linea, "\x00"), []byte{0})
		for _, a := range args[min(1, len(args)):] {
			arg := string(a)
			if r := navegador.RutaDeURL(arg); r != "" {
				arg = r
			}
			if arg == "" || strings.HasPrefix(arg, "-") {
				continue
			}
			if !filepath.IsAbs(arg) {
				if cwd == "" {
					continue
				}
				arg = filepath.Join(cwd, arg)
			}
			if documento.Es(arg) && esArchivo(arg) {
				candidatas = append(candidatas, filepath.Clean(arg))
			}
		}
	}
	for _, r := range abiertos {
		if documento.Es(r) && esArchivo(r) {
			candidatas = append(candidatas, r)
		}
	}
	return documento.Elegir(titulo, candidatas)
}
