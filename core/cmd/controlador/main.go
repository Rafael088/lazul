// controlador: el lado de la cámara, contra un receptor de verdad en otro
// equipo. Se empareja (si hace falta), mide el ping y hace el recorrido de
// punta a punta con las grabaciones reales: cada gesto reconocido se vuelve
// una Orden que se ejecuta en el otro equipo. Lo usa ./dev e2e.
//
//	controlador -receptor HOST:PUERTO -huella HEX [-codigo NNNNNN]
//	            [-estado DIR] [-grabaciones fixtures/gestos] [-intentos N]
//	            [-casos captura,gesto4] [-adelantar] [-ventanas real|url:U|archivo:R|app:ID,...]
//	controlador -solo-huella [-estado DIR]
//
// -huella es la del receptor (la que daría mDNS): el emparejamiento y cada
// conexión fallan si el otro no tiene esa. -ventanas decide qué ventana
// activa ve el gesto 4: "real" (la de este escritorio) o una puesta a mano,
// rotando en cada intento; sirve para probar los niveles URL y archivo, que
// los adaptadores todavía no saben leer.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/core/apps"
	"github.com/Rafael088/lazul/core/ensayo"
	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/gestures"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/platform/real"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

func main() {
	dir := flag.String("receptor", "", "host:puerto del receptor")
	huellaR := flag.String("huella", "", "huella (hex) del receptor")
	codigo := flag.String("codigo", "", "código de 6 dígitos que muestra el receptor (si no está emparejado)")
	estado := flag.String("estado", "", "carpeta con identidad.pem y pares.json del controlador")
	grab := flag.String("grabaciones", "fixtures/gestos", "grabaciones reales de gestos")
	intentos := flag.Int("intentos", 0, "intentos por caso (0: todas las grabaciones)")
	casos := flag.String("casos", "captura,gesto4", "casos de punta a punta")
	ventanas := flag.String("ventanas", "real", "ventana activa del gesto 4: real, url:U, archivo:RUTA o app:ID (id nativo de este sistema), separadas por comas")
	pings := flag.Int("pings", 50, "idas y vueltas para medir el ping")
	soloHuella := flag.Bool("solo-huella", false, "solo mostrar la huella de este controlador")
	nombre := flag.String("nombre", "", "nombre que ve el receptor")
	adelantar := flag.Bool("adelantar", false, "subir la captura al agarrar (y pasar el soltar a su ritmo real)")
	flag.Parse()
	if err := correr(*dir, *huellaR, *codigo, *estado, *grab, *intentos, *casos, *ventanas, *pings, *soloHuella, *nombre, *adelantar); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func correr(dir, huellaR, codigo, estado, grab string, intentos int, casos, ventanas string, pings int, soloHuella bool, nombre string, adelantar bool) error {
	if estado == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		estado = filepath.Join(d, "lazul-controlador")
	}
	if nombre == "" {
		nombre = "Controlador " + runtime.GOOS
	}
	yo, err := core.CargarOCrearIdentidad(filepath.Join(estado, "identidad.pem"), nombre)
	if err != nil {
		return err
	}
	if soloHuella {
		fmt.Println(yo.Huella)
		return nil
	}
	hr, err := core.ParsearHuella(huellaR)
	if err != nil {
		return fmt.Errorf("-huella: %w", err)
	}
	pares, err := core.AbrirPares(filepath.Join(estado, "pares.json"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if _, ok := pares.Buscar(hr); !ok {
		if codigo == "" {
			return fmt.Errorf("no está emparejado con %s: pasa -codigo", hr.Corta())
		}
		ctxE, c := context.WithTimeout(ctx, core.EsperaPermiso+10*time.Second)
		t := time.Now()
		par, err := core.Emparejar(ctxE, dir, yo, &lazulv1.Hola{EquipoId: yo.Huella.String()[:16], Nombre: nombre}, codigo, hr)
		c()
		if err != nil {
			return fmt.Errorf("emparejar: %w", err)
		}
		if err := pares.Fijar(par); err != nil {
			return err
		}
		fmt.Printf("emparejado con «%s» %s en %v (código, permiso y huellas fijadas)\n", par.Nombre, par.Huella.Corta(), time.Since(t).Round(time.Millisecond))
	}
	ctxC, c := context.WithTimeout(ctx, 10*time.Second)
	con, err := core.Conectar(ctxC, dir, yo, hr)
	c()
	if err != nil {
		return fmt.Errorf("conectar: %w", err)
	}
	defer con.Cerrar()
	otro := &core.Remoto{Nombre: "receptor", Con: con, HTTP: core.ClienteHTTP(yo, hr), Base: "https://" + dir}
	resp, err := otro.Pedir(ctx, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Hola{Hola: &lazulv1.Hola{Nombre: nombre}}})
	if err != nil {
		return err
	}
	otro.Nombre = resp.GetHola().GetNombre()
	fmt.Printf("conectado a «%s» (%v) por TLS 1.3 con la huella fijada\n", otro.Nombre, resp.GetHola().GetSistema())

	var ts []time.Duration
	for range pings {
		t := time.Now()
		if r, err := otro.Pedir(ctx, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Ping{Ping: &lazulv1.Ping{}}}); err != nil || r.GetPong() == nil {
			return fmt.Errorf("ping: %v", err)
		}
		ts = append(ts, time.Since(t))
	}
	if len(ts) > 0 {
		fmt.Printf("ping: %d idas y vueltas, p50 %s · p95 %s · máx %s\n", len(ts), ms(ensayo.Percentil(ts, .5)), ms(ensayo.Percentil(ts, .95)), ms(ensayo.Percentil(ts, 1)))
	}

	gs, err := gestures.LeerDirectorio(grab)
	if err != nil {
		return err
	}
	cat, err := apps.Cargar()
	if err != nil {
		return err
	}
	local := real.New()
	vs, err := leerVentanas(ventanas)
	if err != nil {
		return err
	}
	var puesta *falso.Falso
	activaReal := local.ActiveApp
	if len(vs) > 0 {
		puesta = falso.New("ventana")
	}
	e := &core.Enrutador{Local: local, Catalogo: &cat, PorDefecto: otro, Adelantar: adelantar,
		Yo: localsend.Dispositivo{Alias: nombre, Version: "2.0", DeviceType: "desktop", Fingerprint: yo.Huella.String(), Protocol: "https"}}
	antes := func(_ string, i int) {
		v := vs[i%len(vs)]
		if v == nil {
			e.Local.ActiveApp = activaReal
			return
		}
		puesta.PonerApp(*v)
		e.Local.ActiveApp = puesta
	}
	if len(vs) == 0 {
		antes = nil
	}
	modo := "captura subida al soltar"
	if adelantar {
		modo = "captura adelantada al agarrar, soltar a ritmo real"
	}
	fmt.Printf("punta a punta con %s (%d grabaciones, %s): gesto reconocido aquí → Orden ejecutada en «%s»\n", grab, len(gs), modo, otro.Nombre)
	cs := ensayo.Correr(ctx, e, gs, ensayo.Opciones{Max: intentos, Casos: strings.Split(casos, ","), Antes: antes, Ritmo: adelantar}, os.Stdout)
	ensayo.Resumen(os.Stdout, cs)
	return nil
}

// leerVentanas: nil en la lista es "la ventana real". Lista vacía si solo
// hay "real".
func leerVentanas(s string) ([]*platform.App, error) {
	var vs []*platform.App
	soloReal := true
	for _, v := range strings.Split(s, ",") {
		tipo, valor, _ := strings.Cut(strings.TrimSpace(v), ":")
		switch tipo {
		case "real":
			vs = append(vs, nil)
		case "url":
			soloReal = false
			vs = append(vs, &platform.App{ID: "navegador", Nombre: "navegador", URL: valor})
		case "archivo":
			soloReal = false
			vs = append(vs, &platform.App{ID: "editor", Nombre: "editor", Archivo: valor})
		case "app":
			soloReal = false
			vs = append(vs, &platform.App{ID: valor, Nombre: valor})
		default:
			return nil, fmt.Errorf("-ventanas: %q no es real, url:U, archivo:R ni app:ID", v)
		}
	}
	if soloReal {
		return nil, nil
	}
	return vs, nil
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000) }
