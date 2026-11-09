package navegador

import (
	"bytes"
	"encoding/binary"
	"sort"
	"unicode/utf16"
)

// Chromium y sus derivados (Chrome, Edge, Brave, Vivaldi...) guardan la
// sesión en <perfil>/Sessions/Session_<n>, un registro de órdenes ("SNSS")
// al que añaden al vuelo, con unos 2,5 s de retraso. El proceso del
// navegador lo tiene abierto, así que se encuentra por sus descriptores.
//
// La pestaña que se ve sale de reconstruir el estado: la ventana activa, su
// pestaña elegida y la entrada de historial elegida de esa pestaña. El título
// de la entrada a veces no está guardado todavía (la página acaba de cargar);
// si está y no es el de la ventana, la sesión va atrasada y no se manda nada.

// MarcaSesionChromium es lo que lleva la ruta de un archivo de sesión.
const MarcaSesionChromium = "/Sessions/Session_"

// Órdenes del registro (components/sessions/core/session_service_commands.cc).
const (
	crTabWindow         = 0
	crTabIndexInWindow  = 2
	crPrunedFromBack    = 5
	crUpdateNavigation  = 6
	crSelectedNavIndex  = 7
	crSelectedTabIndex  = 8
	crTabClosed         = 16
	crWindowClosed      = 17
	crActiveWindow      = 20
	crNavigationPruned  = 24
	versionSNSS         = 1
	versionSNSSConMarca = 3 // 2 y 4 son cifradas: no se leen
)

type navCr struct{ url, titulo string }

type pestanaCr struct {
	ventana, indice, navElegida int32
	conVentana, conElegida      bool
	navs                        map[int32]navCr
	cerrada                     bool
}

// leerPickle saca enteros y cadenas de un Pickle de Chromium (alineado a 4).
type leerPickle struct {
	b   []byte
	mal bool
}

func (p *leerPickle) entero() int32 {
	if len(p.b) < 4 {
		p.mal = true
		return 0
	}
	v := int32(binary.LittleEndian.Uint32(p.b))
	p.b = p.b[4:]
	return v
}

func (p *leerPickle) bytes(n int) []byte {
	ocupa := (n + 3) &^ 3
	if n < 0 || ocupa > len(p.b) {
		p.mal = true
		return nil
	}
	v := p.b[:n]
	p.b = p.b[ocupa:]
	return v
}

func (p *leerPickle) cadena() string { return string(p.bytes(int(p.entero()))) }

func (p *leerPickle) cadena16() string {
	n := int(p.entero())
	if n < 0 || n > len(p.b)/2 {
		p.mal = true
		return ""
	}
	b := p.bytes(2 * n)
	u := make([]uint16, n)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

// URLChromium devuelve la URL de la pestaña que se ve en la ventana activa
// del registro de sesión, o "" si no se sabe (ver arriba).
func URLChromium(snss []byte, tituloVentana string) string {
	nav, ok := pestanaVisibleChromium(snss)
	if !ok || nav.url == "" {
		return ""
	}
	if nav.titulo != "" && !CoincideTitulo(tituloVentana, nav.titulo) {
		return ""
	}
	return nav.url
}

func pestanaVisibleChromium(b []byte) (navCr, bool) {
	if len(b) < 8 || !bytes.Equal(b[:4], []byte("SNSS")) {
		return navCr{}, false
	}
	if v := binary.LittleEndian.Uint32(b[4:8]); v != versionSNSS && v != versionSNSSConMarca {
		return navCr{}, false
	}
	pestanas := map[int32]*pestanaCr{}
	pestana := func(id int32) *pestanaCr {
		p := pestanas[id]
		if p == nil {
			p = &pestanaCr{navs: map[int32]navCr{}}
			pestanas[id] = p
		}
		return p
	}
	elegidaEn := map[int32]int32{}
	cerradas := map[int32]bool{}
	activa, hayActiva := int32(0), false
	dos := func(c []byte) (int32, int32, bool) {
		if len(c) < 8 {
			return 0, 0, false
		}
		return int32(binary.LittleEndian.Uint32(c)), int32(binary.LittleEndian.Uint32(c[4:])), true
	}
	for i := 8; i+2 <= len(b); {
		n := int(binary.LittleEndian.Uint16(b[i:]))
		if n == 0 || i+2+n > len(b) {
			break // orden cortada: el navegador la está escribiendo
		}
		id, c := b[i+2], b[i+3:i+2+n]
		i += 2 + n
		switch id {
		case crTabWindow:
			if v, t, ok := dos(c); ok {
				p := pestana(t)
				p.ventana, p.conVentana = v, true
			}
		case crTabIndexInWindow:
			if t, x, ok := dos(c); ok {
				pestana(t).indice = x
			}
		case crSelectedNavIndex:
			if t, x, ok := dos(c); ok {
				p := pestana(t)
				p.navElegida, p.conElegida = x, true
			}
		case crSelectedTabIndex:
			if v, x, ok := dos(c); ok {
				elegidaEn[v] = x
			}
		case crActiveWindow:
			if len(c) >= 4 {
				activa, hayActiva = int32(binary.LittleEndian.Uint32(c)), true
			}
		case crTabClosed:
			if len(c) >= 4 {
				pestana(int32(binary.LittleEndian.Uint32(c))).cerrada = true
			}
		case crWindowClosed:
			if len(c) >= 4 {
				cerradas[int32(binary.LittleEndian.Uint32(c))] = true
			}
		case crPrunedFromBack:
			if t, x, ok := dos(c); ok {
				for k := range pestana(t).navs {
					if k >= x {
						delete(pestana(t).navs, k)
					}
				}
			}
		case crNavigationPruned:
			if t, x, ok := dos(c); ok && len(c) >= 12 {
				cuantas := int32(binary.LittleEndian.Uint32(c[8:]))
				p := pestana(t)
				navs := map[int32]navCr{}
				for k, v := range p.navs {
					switch {
					case k < x:
						navs[k] = v
					case k >= x+cuantas:
						navs[k-cuantas] = v
					}
				}
				p.navs = navs
			}
		case crUpdateNavigation:
			pk := &leerPickle{b: c}
			pk.entero() // tamaño del Pickle
			t, x := pk.entero(), pk.entero()
			u := pk.cadena()
			titulo := pk.cadena16()
			if !pk.mal {
				pestana(t).navs[x] = navCr{url: u, titulo: titulo}
			}
		}
	}
	if !hayActiva || cerradas[activa] {
		// Sin ventana activa anotada: vale solo si queda una ventana abierta.
		abiertas := map[int32]bool{}
		for _, p := range pestanas {
			if p.conVentana && !p.cerrada && !cerradas[p.ventana] {
				abiertas[p.ventana] = true
			}
		}
		if len(abiertas) != 1 {
			return navCr{}, false
		}
		for v := range abiertas {
			activa = v
		}
	}
	// El índice elegido es la posición entre las pestañas abiertas de la
	// ventana, en el orden de sus índices guardados: al cerrar una pestaña no
	// se reescriben los índices de las demás, pero el orden se mantiene (así
	// las restaura el propio Chromium).
	var enVentana []*pestanaCr
	for _, p := range pestanas {
		if p.conVentana && p.ventana == activa && !p.cerrada {
			enVentana = append(enVentana, p)
		}
	}
	sort.SliceStable(enVentana, func(i, j int) bool { return enVentana[i].indice < enVentana[j].indice })
	elegida := int(elegidaEn[activa])
	if elegida < 0 || elegida >= len(enVentana) {
		return navCr{}, false
	}
	if elegida+1 < len(enVentana) && enVentana[elegida+1].indice == enVentana[elegida].indice ||
		elegida > 0 && enVentana[elegida-1].indice == enVentana[elegida].indice {
		return navCr{}, false // dos pestañas en el mismo sitio: no se adivina
	}
	visible := enVentana[elegida]
	if visible == nil || len(visible.navs) == 0 {
		return navCr{}, false
	}
	x := visible.navElegida
	if !visible.conElegida {
		for k := range visible.navs {
			if k > x {
				x = k
			}
		}
	}
	nav, ok := visible.navs[x]
	return nav, ok
}
