package navegador

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestCoincideTitulo(t *testing.T) {
	casos := []struct {
		ventana, pestana string
		quiero           bool
	}{
		{"Example Domain - Chromium", "Example Domain", true},
		{"Example Domain — Mozilla Firefox", "Example Domain", true},
		{"Example Domain — Zen Browser", "Example Domain", true},
		{"Nueva pestaña - Personal: Microsoft\u200b Edge", "Nueva pestaña", true},
		{"Example Domain", "Example Domain", true},
		{"Example Domain - Chromium", "Example", false},
		{"Example Domain - Chromium", "", false},
		{"Otra cosa - Chromium", "Example Domain", false},
	}
	for _, c := range casos {
		if got := CoincideTitulo(c.ventana, c.pestana); got != c.quiero {
			t.Errorf("CoincideTitulo(%q, %q) = %v", c.ventana, c.pestana, got)
		}
	}
}

func TestDeBarra(t *testing.T) {
	casos := map[string]string{
		"https://example.org/a?b=1": "https://example.org/a?b=1",
		"http://192.168.1.2:8080/":  "http://192.168.1.2:8080/",
		"example.org/a?b=1":         "https://example.org/a?b=1",
		"localhost:3000/x":          "https://localhost:3000/x",
		"file:///C:/Users/x/a.pdf":  "file:///C:/Users/x/a.pdf",
		"edge://settings":           "edge://settings",
		"about:blank":               "about:blank",
		"cómo hacer pan":            "",
		"buscar":                    "",
		"":                          "",
		`C:\Users\x\a.pdf`:          "",
	}
	for v, quiero := range casos {
		if got := DeBarra(v); got != quiero {
			t.Errorf("DeBarra(%q) = %q, quiero %q", v, got, quiero)
		}
	}
}

func TestRutaDeURL(t *testing.T) {
	casos := map[string]string{
		"file:///home/x/a%20b.pdf": "/home/x/a b.pdf",
		"https://example.org/a":    "",
		"file://otro/x.pdf":        "",
	}
	for u, quiero := range casos {
		if got := RutaDeURL(u); got != quiero {
			t.Errorf("RutaDeURL(%q) = %q, quiero %q", u, got, quiero)
		}
	}
	if got := RutaDeURL("file:///C:/Users/x/a.pdf"); !strings.HasSuffix(got, "a.pdf") || strings.HasPrefix(got, "/") {
		t.Errorf("RutaDeURL de Windows = %q", got)
	}
}

// --- mozLz4 ---

func TestLz4Bloque(t *testing.T) {
	largo := strings.Repeat("x", 20)
	casos := []struct {
		nombre string
		bloque []byte
		quiero string
	}{
		{"solo literales", append([]byte{0x30}, "abc"...), "abc"},
		// "abc" y una copia de 9 desde 3 atrás (solapada), y el cierre sin literales.
		{"copia solapada", append(append([]byte{0x35}, "abc"...), 3, 0, 0x00), "abcabcabcabc"},
		// 20 literales: 15 en el token + 5 en el byte que sigue.
		{"literales largos", append([]byte{0xF0, 5}, largo...), largo},
		// 1 literal y una copia de 4+15+3=22.
		{"copia larga", []byte{0x1F, 'z', 1, 0, 3, 0x00}, strings.Repeat("z", 23)},
	}
	for _, c := range casos {
		got, err := lz4Bloque(c.bloque, len(c.quiero))
		if err != nil || string(got) != c.quiero {
			t.Errorf("%s: %q, %v", c.nombre, got, err)
		}
	}
	malos := map[string][]byte{
		"desplazamiento cero":   {0x14, 'a', 0, 0},
		"desplazamiento lejano": {0x14, 'a', 9, 0},
		"literales cortados":    {0x50, 'a'},
		"más de lo anunciado":   append([]byte{0x30}, "abcd"...),
	}
	for n, b := range malos {
		if _, err := lz4Bloque(b, 3); err == nil {
			t.Errorf("%s: debía fallar", n)
		}
	}
}

// mozLz4 arma un .jsonlz4 válido solo con literales (lz4 lo permite).
func mozLz4(datos []byte) []byte {
	var b bytes.Buffer
	b.Write(magiaMozLz4)
	binary.Write(&b, binary.LittleEndian, uint32(len(datos)))
	n := len(datos)
	if n < 15 {
		b.WriteByte(byte(n << 4))
	} else {
		b.WriteByte(0xF0)
		for n -= 15; n >= 255; n -= 255 {
			b.WriteByte(255)
		}
		b.WriteByte(byte(n))
	}
	b.Write(datos)
	return b.Bytes()
}

func TestDescomprimirMozLz4(t *testing.T) {
	json := []byte(`{"windows":[]}` + strings.Repeat(" ", 600))
	got, err := DescomprimirMozLz4(mozLz4(json))
	if err != nil || !bytes.Equal(got, json) {
		t.Fatalf("%q, %v", got, err)
	}
	if _, err := DescomprimirMozLz4([]byte("{}")); err == nil {
		t.Error("sin la magia debía fallar")
	}
}

// --- sesión de Mozilla ---

type entrada struct {
	URL    string `json:"url"`
	Titulo string `json:"title"`
}
type pestana struct {
	Entradas []entrada `json:"entries"`
	Indice   int       `json:"index"`
}
type ventana struct {
	Pestanas []pestana `json:"tabs"`
	Elegida  int       `json:"selected"`
}

func sesion(t *testing.T, vs ...ventana) []byte {
	b, err := json.Marshal(map[string]any{"windows": vs, "selectedWindow": 1, "version": []any{"sessionrestore", 1}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestURLMozilla(t *testing.T) {
	ej := entrada{"https://example.org/?a=1", "Example Domain"}
	otra := entrada{"https://otra.test/", "Otra"}
	conHistorial := pestana{Entradas: []entrada{otra, ej}, Indice: 2}
	atras := pestana{Entradas: []entrada{otra, ej}, Indice: 1}
	casos := []struct {
		nombre, titulo string
		s              []ventana
		quiero         string
	}{
		{"pestaña elegida", "Example Domain — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{{Entradas: []entrada{otra}, Indice: 1}, conHistorial}, Elegida: 2}}, ej.URL},
		{"volvió atrás en el historial", "Otra — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{atras}, Elegida: 1}}, otra.URL},
		{"sesión atrasada tras cambiar de pestaña", "Example Domain — Zen Browser",
			[]ventana{{Pestanas: []pestana{{Entradas: []entrada{otra}, Indice: 1}, conHistorial}, Elegida: 1}}, ej.URL},
		{"segunda ventana", "Example Domain — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{{Entradas: []entrada{otra}, Indice: 1}}, Elegida: 1},
				{Pestanas: []pestana{conHistorial}, Elegida: 1}}, ej.URL},
		{"ninguna con ese título (ventana privada o sesión vieja)", "Nada — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{conHistorial}, Elegida: 1}}, ""},
		{"dos pestañas con el título y URL distinta", "Example Domain — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{
				{Entradas: []entrada{ej}, Indice: 1},
				{Entradas: []entrada{{"https://example.com/", "Example Domain"}}, Indice: 1}}, Elegida: 3}}, ""},
		{"dos pestañas iguales", "Example Domain — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{{Entradas: []entrada{ej}, Indice: 1}, conHistorial}, Elegida: 3}}, ej.URL},
		{"índice fuera de rango", "Example Domain — Mozilla Firefox",
			[]ventana{{Pestanas: []pestana{{Entradas: []entrada{otra, ej}, Indice: 9}}, Elegida: 1}}, ej.URL},
	}
	for _, c := range casos {
		if got := URLMozilla(sesion(t, c.s...), c.titulo); got != c.quiero {
			t.Errorf("%s: %q, quiero %q", c.nombre, got, c.quiero)
		}
	}
	if got := URLMozilla([]byte("no es json"), "x"); got != "" {
		t.Errorf("con basura: %q", got)
	}
}

// --- sesión de Chromium ---

type snss struct{ b bytes.Buffer }

func nuevoSNSS(version uint32) *snss {
	s := &snss{}
	s.b.WriteString("SNSS")
	binary.Write(&s.b, binary.LittleEndian, version)
	return s
}

func (s *snss) orden(id byte, carga []byte) *snss {
	binary.Write(&s.b, binary.LittleEndian, uint16(len(carga)+1))
	s.b.WriteByte(id)
	s.b.Write(carga)
	return s
}

func enteros(vs ...int32) []byte {
	var b bytes.Buffer
	for _, v := range vs {
		binary.Write(&b, binary.LittleEndian, v)
	}
	return b.Bytes()
}

func alinear(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

func (s *snss) nav(tab, indice int32, url, titulo string) *snss {
	var p bytes.Buffer
	p.Write(enteros(tab, indice, int32(len(url))))
	p.WriteString(url)
	alinear(&p)
	u := utf16.Encode([]rune(titulo))
	p.Write(enteros(int32(len(u))))
	binary.Write(&p, binary.LittleEndian, u)
	alinear(&p)
	p.Write(enteros(0, 0)) // lo que sigue (estado de la página...) no se lee
	return s.orden(crUpdateNavigation, append(enteros(int32(p.Len())), p.Bytes()...))
}

func (s *snss) bytes() []byte { return s.b.Bytes() }

// ventanaCon: la ventana w con las pestañas dadas en ese orden.
func (s *snss) ventanaCon(w int32, tabs ...int32) *snss {
	for i, t := range tabs {
		s.orden(crTabWindow, enteros(w, t)).orden(crTabIndexInWindow, enteros(t, int32(i)))
	}
	return s
}

func TestURLChromium(t *testing.T) {
	const titulo = "Example Domain - Chromium"
	ej, otra := "https://example.org/?a=1", "https://otra.test/"
	base := func() *snss {
		return nuevoSNSS(3).ventanaCon(1, 10, 11).
			nav(10, 0, otra, "Otra").
			nav(11, 0, otra, "Otra").nav(11, 1, ej, "").
			orden(crSelectedNavIndex, enteros(11, 1)).
			orden(crSelectedTabIndex, enteros(1, 1)).
			orden(crActiveWindow, enteros(1))
	}
	casos := []struct {
		nombre string
		s      *snss
		titulo string
		quiero string
	}{
		{"recién cargada, sin título guardado", base(), titulo, ej},
		{"con título guardado", base().nav(11, 1, ej, "Example Domain"), titulo, ej},
		{"título guardado distinto: sesión atrasada", base().nav(11, 1, ej, "Otra cosa"), titulo, ""},
		{"volvió atrás", base().orden(crSelectedNavIndex, enteros(11, 0)), "Otra - Chromium", otra},
		{"otra pestaña elegida", base().orden(crSelectedTabIndex, enteros(1, 0)), "Otra - Chromium", otra},
		{"se cerró la primera pestaña: la posición manda",
			base().orden(crTabClosed, append(enteros(10, 0), make([]byte, 8)...)).orden(crSelectedTabIndex, enteros(1, 0)), titulo, ej},
		{"otra ventana activa", base().ventanaCon(2, 20).nav(20, 0, otra, "").
			orden(crSelectedTabIndex, enteros(2, 0)).orden(crActiveWindow, enteros(2)), titulo, otra},
		{"sin ventana activa y una sola abierta", nuevoSNSS(1).ventanaCon(1, 10).nav(10, 0, ej, ""), titulo, ej},
		{"sin ventana activa y dos abiertas", nuevoSNSS(1).ventanaCon(1, 10).ventanaCon(2, 20).nav(10, 0, ej, ""), titulo, ""},
		{"historial podado", base().orden(crNavigationPruned, enteros(11, 0, 1)).orden(crSelectedNavIndex, enteros(11, 0)), titulo, ej},
		{"podado desde atrás", base().orden(crPrunedFromBack, enteros(11, 1)).orden(crSelectedNavIndex, enteros(11, 1)), titulo, ""},
		{"cifrada", func() *snss { s := base(); s.b.Bytes()[4] = 2; return s }(), titulo, ""},
		{"orden cortada al final", func() *snss {
			s := base()
			s.b.Write([]byte{200, 0, crUpdateNavigation, 1, 2})
			return s
		}(), titulo, ej},
	}
	for _, c := range casos {
		if got := URLChromium(c.s.bytes(), c.titulo); got != c.quiero {
			t.Errorf("%s: %q, quiero %q", c.nombre, got, c.quiero)
		}
	}
	if got := URLChromium([]byte("basura"), titulo); got != "" {
		t.Errorf("con basura: %q", got)
	}
}
