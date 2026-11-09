package documento

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// Las extensiones son las mismas que acepta abrir el receptor: si el lado que
// lee manda un tipo que el receptor no abre, el gesto no cae al nivel de la app.
func TestExtensionesComoElReceptor(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "core", "seguro.go"))
	if err != nil {
		t.Fatal(err)
	}
	bloque := regexp.MustCompile(`(?s)var extensionesAbribles = map\[string\]bool\{(.*?)\n\}`).FindSubmatch(src)
	if bloque == nil {
		t.Fatal("no encontré extensionesAbribles en core/seguro.go")
	}
	var receptor, aqui []string
	for _, m := range regexp.MustCompile(`"(\.[a-z0-9]+)"`).FindAllSubmatch(bloque[1], -1) {
		receptor = append(receptor, string(m[1]))
	}
	for e := range extensiones {
		aqui = append(aqui, e)
	}
	sort.Strings(receptor)
	sort.Strings(aqui)
	if !reflect.DeepEqual(receptor, aqui) {
		t.Errorf("core/seguro.go abre %v y aquí se mandan %v", receptor, aqui)
	}
}

func TestElegir(t *testing.T) {
	casos := []struct {
		nombre, titulo string
		candidatas     []string
		quiero         string
	}{
		{"nombre en el título", "informe.pdf — Papers", []string{"/home/a/.cache/x.db", "/home/a/informe.pdf"}, "/home/a/informe.pdf"},
		{"sin extensión, estilo Word", "informe - Word", []string{`C:\Users\a\informe.docx`}, `C:\Users\a\informe.docx`},
		{"con asterisco de sin guardar", "*notas - Bloc de notas", []string{`C:\x\notas.txt`}, `C:\x\notas.txt`},
		{"no sale en el título", "Visor", []string{"/home/a/informe.pdf"}, ""},
		{"no es documento", "main.go - nvim", []string{"/home/a/main.go"}, ""},
		{"gana el nombre más largo", "informe-final.pdf - Visor", []string{"/a/final.pdf", "/a/informe-final.pdf"}, "/a/informe-final.pdf"},
		{"dos que empatan", "a.png b.png - imv", []string{"/x/a.png", "/x/b.png"}, ""},
		{"la misma dos veces", "a.png - imv", []string{"/x/a.png", "/x/a.png"}, "/x/a.png"},
		{"tallo muy corto", "a - Word", []string{"/x/a.docx"}, ""},
	}
	for _, c := range casos {
		if got := Elegir(c.titulo, c.candidatas); got != c.quiero {
			t.Errorf("%s: %q, quiero %q", c.nombre, got, c.quiero)
		}
	}
}

func TestRutasEnTitulo(t *testing.T) {
	existen := map[string]bool{
		"/home/a/fotos/mi foto.png": true,
		"/home/a/b.pdf":             true,
		`C:\Users\a\c.txt`:          true,
	}
	existe := func(r string) bool { return existen[r] }
	casos := map[string][]string{
		"imv - [1/1] [1280x720] [136%] /home/a/fotos/mi foto.png": {"/home/a/fotos/mi foto.png"},
		"/home/a/b.pdf - mpv":                    {"/home/a/b.pdf"},
		`C:\Users\a\c.txt - Bloc de notas`:       {`C:\Users\a\c.txt`},
		"nvim /home/a/no-existe.pdf":             nil,
		"sin rutas.pdf":                          nil,
		"a/home/a/b.pdf (no empieza en espacio)": nil,
	}
	for titulo, quiero := range casos {
		if got := RutasEnTitulo(titulo, existe); !reflect.DeepEqual(got, quiero) {
			t.Errorf("RutasEnTitulo(%q) = %q, quiero %q", titulo, got, quiero)
		}
	}
}
