// Package sim levanta en local un controlador y N receptores falsos
// (platform/falso) que se hablan por WebSocket con los Sobres de proto/.
// Es lo que corre ./dev sim y sirve para probar el núcleo sin hardware.
package sim

import (
	"context"
	"fmt"
	"io"
	"net"
	"slices"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

type receptor struct {
	nombre string
	falso  *falso.Falso
	con    *core.Conexion
}

// Correr ejecuta el recorrido completo con n receptores (mínimo 2) y escribe
// una línea por paso en out. Devuelve error si algo no salió como se espera.
func Correr(ctx context.Context, n int, pings int, out io.Writer) error {
	if n < 2 {
		return fmt.Errorf("hacen falta al menos 2 receptores")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rs := make([]*receptor, n)
	for i := range rs {
		nombre := fmt.Sprintf("receptor-%d", i+1)
		f := falso.New(nombre, "firefox")
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		r := &core.Receptor{Sistema: f.Sistema(), Hola: &lazulv1.Hola{
			EquipoId: nombre, Nombre: nombre, Sistema: lazulv1.Sistema_SISTEMA_LINUX, VersionApp: "sim",
			Capacidades: []lazulv1.Capacidad{lazulv1.Capacidad_CAPACIDAD_CAPTURA, lazulv1.Capacidad_CAPACIDAD_PORTAPAPELES,
				lazulv1.Capacidad_CAPACIDAD_APP_ACTIVA, lazulv1.Capacidad_CAPACIDAD_ABRIR_APP},
		}}
		go core.Servir(ctx, ln, r)
		con, err := core.Conectar(ctx, "ws://"+ln.Addr().String()+"/lazul")
		if err != nil {
			return err
		}
		defer con.Cerrar()
		rs[i] = &receptor{nombre: nombre, falso: f, con: con}
	}

	c := &controlador{}
	for _, r := range rs {
		resp, err := c.pedir(ctx, r.con, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Hola{Hola: &lazulv1.Hola{EquipoId: "controlador"}}})
		if err != nil {
			return err
		}
		if resp.GetHola().GetNombre() != r.nombre {
			return fmt.Errorf("Hola de %s trajo %q", r.nombre, resp.GetHola().GetNombre())
		}
	}
	fmt.Fprintf(out, "receptores: %d conectados por WebSocket en 127.0.0.1\n", n)

	// Latencia de ida y vuelta de un comando (meta de la fase: < 100 ms).
	var tiempos []time.Duration
	for range pings {
		for _, r := range rs {
			ini := time.Now()
			resp, err := c.pedir(ctx, r.con, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Ping{Ping: &lazulv1.Ping{}}})
			if err != nil {
				return err
			}
			if resp.GetPong() == nil {
				return fmt.Errorf("Ping sin Pong: %v", resp)
			}
			tiempos = append(tiempos, time.Since(ini))
		}
	}
	slices.Sort(tiempos)
	p := func(q float64) time.Duration { return tiempos[int(q*float64(len(tiempos)-1))] }
	fmt.Fprintf(out, "ping: %d idas y vueltas, p50 %v, p95 %v\n", len(tiempos), p(0.5).Round(time.Microsecond), p(0.95).Round(time.Microsecond))

	a, b := rs[0], rs[1]

	// Agarrar el texto del portapapeles de A y soltarlo en B.
	texto := "hola desde " + a.nombre
	_ = a.falso.WriteText(ctx, texto)
	obj, err := c.tomar(ctx, a.con, lazulv1.Tomar_QUE_PORTAPAPELES)
	if err != nil {
		return err
	}
	if err := c.entregar(ctx, b.con, obj); err != nil {
		return err
	}
	if got, _ := b.falso.ReadText(ctx); got != texto {
		return fmt.Errorf("portapapeles de %s: %q, esperaba %q", b.nombre, got, texto)
	}
	fmt.Fprintf(out, "portapapeles: %s → %s ok\n", a.nombre, b.nombre)

	// Captura: solo metadatos; los bytes irán por LocalSend.
	obj, err = c.tomar(ctx, a.con, lazulv1.Tomar_QUE_CAPTURA)
	if err != nil {
		return err
	}
	if obj.GetArchivo().GetBytes() == 0 || len(obj.GetArchivo().GetSha256()) != 32 {
		return fmt.Errorf("captura sin datos: %v", obj)
	}
	fmt.Fprintf(out, "captura: %s, %d bytes (el archivo viajará por LocalSend)\n", obj.GetArchivo().GetNombre(), obj.GetArchivo().GetBytes())

	// Gesto 4, nivel 1: la URL del navegador de A se abre en B.
	a.falso.PonerApp(platform.App{ID: "firefox", Nombre: "Firefox", URL: "https://lazul.dev/sim"})
	obj, err = c.tomar(ctx, a.con, lazulv1.Tomar_QUE_APP_ACTIVA)
	if err != nil {
		return err
	}
	if err := c.entregar(ctx, b.con, obj); err != nil {
		return err
	}
	if ab := b.falso.Abiertas(); len(ab) != 1 || ab[0].URL != "https://lazul.dev/sim" {
		return fmt.Errorf("%s abrió %v", b.nombre, ab)
	}
	// Nivel 3: una app que B no tiene cae a su web de respaldo.
	if err := c.entregar(ctx, b.con, &lazulv1.Objeto{Tipo: &lazulv1.Objeto_App{App: &lazulv1.AppRef{
		NativoId: "claude", Nombre: "Claude", Web: "https://claude.ai"}}}); err != nil {
		return err
	}
	if ab := b.falso.Abiertas(); len(ab) != 2 || ab[1].URL != "https://claude.ai" {
		return fmt.Errorf("respaldo web: %s abrió %v", b.nombre, ab)
	}
	fmt.Fprintf(out, "gesto 4: URL %s → %s ok; app ausente cae a la web ok\n", a.nombre, b.nombre)
	return nil
}

type controlador struct{ sig int }

func (c *controlador) pedir(ctx context.Context, con *core.Conexion, s *lazulv1.Sobre) (*lazulv1.Sobre, error) {
	c.sig++
	s.Version = uint32(lazulv1.Version_VERSION_ACTUAL)
	s.Id = fmt.Sprintf("c-%d", c.sig)
	s.EnviadoMs = time.Now().UnixMilli()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := con.Enviar(ctx, s); err != nil {
		return nil, err
	}
	resp, err := con.Recibir(ctx)
	if err != nil {
		return nil, err
	}
	if resp.GetEnRespuestaA() != s.Id {
		return nil, fmt.Errorf("respuesta a %q, esperaba %q", resp.GetEnRespuestaA(), s.Id)
	}
	return resp, nil
}

func (c *controlador) orden(ctx context.Context, con *core.Conexion, o *lazulv1.Orden) (*lazulv1.Respuesta, error) {
	resp, err := c.pedir(ctx, con, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Orden{Orden: o}})
	if err != nil {
		return nil, err
	}
	r := resp.GetRespuesta()
	if !r.GetOk() {
		return nil, fmt.Errorf("orden rechazada: %v %s", r.GetFalla().GetCodigo(), r.GetFalla().GetDetalle())
	}
	return r, nil
}

func (c *controlador) tomar(ctx context.Context, con *core.Conexion, que lazulv1.Tomar_Que) (*lazulv1.Objeto, error) {
	r, err := c.orden(ctx, con, &lazulv1.Orden{Accion: &lazulv1.Orden_Tomar{Tomar: &lazulv1.Tomar{Que: que}}})
	return r.GetObjeto(), err
}

func (c *controlador) entregar(ctx context.Context, con *core.Conexion, obj *lazulv1.Objeto) error {
	_, err := c.orden(ctx, con, &lazulv1.Orden{Accion: &lazulv1.Orden_Entregar{Entregar: &lazulv1.Entregar{Objeto: obj}}})
	return err
}
