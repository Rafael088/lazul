// receptor: un receptor de Lazul de verdad en este equipo (captura,
// portapapeles, ventana activa y abrir apps con el adaptador del sistema), que
// atiende Órdenes y recibe archivos por LocalSend en el mismo puerto TLS.
// Lo usa ./dev e2e para medir entre los 2 PCs.
//
//	receptor [-escucha :53317] [-estado DIR] [-bandeja DIR] [-emparejar]
//	         [-permitir HUELLA] [-abrir=false] [-duracion 2m] [-falso]
//
// Con -emparejar muestra un código de 6 dígitos. El permiso de la persona se
// pide en la terminal (s/n); -permitir HUELLA lo da solo a esa huella (la del
// controlador que se espera), para las mediciones sin nadie al teclado.
// Con -abrir=false no abre nada en pantalla: anota lo que habría abierto
// (la URL, el archivo recibido o la app ya traducida por el catálogo).
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/core/apps"
	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/platform/real"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// anotador cumple LaunchApp sin abrir nada: deja la línea en la salida.
type anotador struct{}

func (anotador) Launch(ctx context.Context, ref platform.AppRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch {
	case ref.URL != "":
		decir("abriría URL %s", ref.URL)
	case ref.Archivo != "":
		info, err := os.Stat(ref.Archivo)
		if err != nil {
			return fmt.Errorf("archivo %q: %w", ref.Archivo, platform.ErrNoEncontrada)
		}
		decir("abriría archivo %s (%d bytes)", filepath.Base(ref.Archivo), info.Size())
	case ref.ID != "":
		decir("abriría app %s", ref.ID)
	default:
		return platform.ErrRefInvalida
	}
	return nil
}

var muSalida sync.Mutex

func decir(f string, a ...any) {
	muSalida.Lock()
	defer muSalida.Unlock()
	fmt.Printf("%s "+f+"\n", append([]any{time.Now().Format("15:04:05.000")}, a...)...)
}

func main() {
	escucha := flag.String("escucha", ":53317", "dirección TCP (53317 es el puerto de LocalSend)")
	estado := flag.String("estado", "", "carpeta con identidad.pem y pares.json (por defecto la de configuración del usuario)")
	bandeja := flag.String("bandeja", "", "dónde quedan los archivos recibidos (por defecto <estado>/recibidos)")
	emparejar := flag.Bool("emparejar", false, "abrir el emparejamiento y mostrar el código")
	permitir := flag.String("permitir", "", "huella (hex) del controlador al que se le da el permiso sin preguntar")
	abrir := flag.Bool("abrir", true, "abrir de verdad lo que llega (false: solo anotarlo)")
	duracion := flag.Duration("duracion", 0, "terminar después de este tiempo (0: hasta Ctrl+C)")
	usarFalso := flag.Bool("falso", false, "usar el sistema en memoria en vez del de verdad")
	nombre := flag.String("nombre", "", "nombre que ve el controlador")
	flag.Parse()
	if err := correr(*escucha, *estado, *bandeja, *emparejar, *permitir, *abrir, *duracion, *usarFalso, *nombre); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func correr(escucha, estado, bandeja string, emparejar bool, permitir string, abrir bool, duracion time.Duration, usarFalso bool, nombre string) error {
	if estado == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		estado = filepath.Join(d, "lazul")
	}
	if bandeja == "" {
		bandeja = filepath.Join(estado, "recibidos")
	}
	if nombre == "" {
		nombre = "PC " + strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
	}
	id, err := core.CargarOCrearIdentidad(filepath.Join(estado, "identidad.pem"), nombre)
	if err != nil {
		return err
	}
	pares, err := core.AbrirPares(filepath.Join(estado, "pares.json"))
	if err != nil {
		return err
	}
	cat, err := apps.Cargar()
	if err != nil {
		return err
	}
	sis := real.New()
	if usarFalso {
		sis = falso.New(nombre, "firefox", "firefox.exe").Sistema()
	}
	if !abrir {
		sis.LaunchApp = anotador{}
	}
	var huellaOK core.Huella
	if permitir != "" {
		if huellaOK, err = core.ParsearHuella(permitir); err != nil {
			return err
		}
	}
	entrada := bufio.NewReader(os.Stdin)
	puerta := &core.Puerta{Identidad: id, Pares: pares, Permiso: func(ctx context.Context, s core.Solicitud) bool {
		if permitir != "" {
			ok := s.Huella == huellaOK
			decir("permiso: «%s» %s → %v (solo se acepta la huella de -permitir)", s.Nombre, s.Huella.Corta(), ok)
			return ok
		}
		decir("«%s» con huella %s quiere emparejarse. ¿Permitir? (s/n)", s.Nombre, s.Huella.Corta())
		l, _ := entrada.ReadString('\n')
		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "s")
	}}
	r := &core.Receptor{Sistema: sis, Catalogo: &cat, Hola: &lazulv1.Hola{EquipoId: id.Huella.String()[:16], Nombre: nombre,
		Sistema: sistemaProto(), VersionApp: "0.1.0-dev",
		Capacidades: []lazulv1.Capacidad{lazulv1.Capacidad_CAPACIDAD_CAPTURA, lazulv1.Capacidad_CAPACIDAD_PORTAPAPELES,
			lazulv1.Capacidad_CAPACIDAD_APP_ACTIVA, lazulv1.Capacidad_CAPACIDAD_ABRIR_APP, lazulv1.Capacidad_CAPACIDAD_RECIBIR_ARCHIVO}},
		Archivos: &localsend.Servidor{Bandeja: bandeja, Info: localsend.Dispositivo{Alias: nombre, Version: "2.0",
			DeviceType: "desktop", Fingerprint: id.Huella.String(), Protocol: "https"}},
		Registro: func(o *lazulv1.Orden, resp *lazulv1.Respuesta, d time.Duration) {
			que := describir(o)
			if resp.GetOk() {
				decir("orden %s ok (aquí %.1f ms)", que, float64(d.Microseconds())/1000)
			} else {
				decir("orden %s FALLA %v %s", que, resp.GetFalla().GetCodigo(), resp.GetFalla().GetDetalle())
			}
		},
	}
	ln, err := net.Listen("tcp", escucha)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if duracion > 0 {
		ctx, cancel = context.WithTimeout(ctx, duracion)
		defer cancel()
	}
	decir("huella %s", id.Huella)
	decir("escucha %s · sistema %s · abrir %v", ln.Addr(), runtime.GOOS, abrir)
	if emparejar {
		codigo, err := puerta.AbrirEmparejamiento()
		if err != nil {
			return err
		}
		decir("codigo %s (vence en %v)", codigo, core.DuracionCodigo)
	}
	err = core.Servir(ctx, ln, r, puerta)
	decir("fin")
	return err
}

func sistemaProto() lazulv1.Sistema {
	switch runtime.GOOS {
	case "windows":
		return lazulv1.Sistema_SISTEMA_WINDOWS
	case "linux":
		return lazulv1.Sistema_SISTEMA_LINUX
	}
	return lazulv1.Sistema_SISTEMA_DESCONOCIDO
}

func describir(o *lazulv1.Orden) string {
	if t := o.GetTomar(); t != nil {
		return "Tomar " + t.GetQue().String()
	}
	obj := o.GetEntregar().GetObjeto()
	switch {
	case obj.GetUrl() != "":
		return "Entregar url " + obj.GetUrl()
	case obj.GetApp() != nil:
		return fmt.Sprintf("Entregar app %s (catálogo %q)", obj.GetApp().GetNombre(), obj.GetApp().GetCatalogoId())
	case obj.GetArchivo() != nil:
		return fmt.Sprintf("Entregar archivo %s (%d bytes)", obj.GetArchivo().GetNombre(), obj.GetArchivo().GetBytes())
	case obj.GetTipo() != nil:
		return "Entregar texto"
	}
	return "vacía"
}
