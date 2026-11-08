// Package sim levanta en local un controlador y N receptores falsos
// (platform/falso) que se descubren por mDNS, se emparejan y se hablan por
// WebSocket sobre TLS con los Sobres de proto/.
// Es lo que corre ./dev sim y sirve para probar el núcleo sin hardware.
package sim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/core/descubre"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

type receptor struct {
	nombre string
	falso  *falso.Falso
	puerta *core.Puerta
	dir    string
	con    *core.Conexion
}

// Opciones de Correr. Red nil usa descubre.Memoria (sin red).
type Opciones struct {
	Receptores int
	Pings      int
	Red        descubre.Red
}

// Correr ejecuta el recorrido completo con un controlador y o.Receptores
// receptores (mínimo 2) y escribe una línea por paso en out: descubrir,
// rechazar al no emparejado, emparejar con código y permiso, rechazar un
// código erróneo, uno caducado, a un intruso y a un impostor, y después las
// Órdenes por TLS. Devuelve error si algo no salió como se espera.
func Correr(ctx context.Context, o Opciones, out io.Writer) error {
	n, pings := o.Receptores, o.Pings
	if n < 2 {
		return fmt.Errorf("hacen falta al menos 2 receptores")
	}
	red, dondeRed, escucha := o.Red, "en memoria", "127.0.0.1:0"
	if red == nil {
		red = &descubre.Memoria{}
	} else {
		// Con mDNS de verdad se anuncia la IP de la red: hay que escuchar ahí.
		dondeRed, escucha = "por la red local", ":0"
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sufijo := fmt.Sprintf("%04x", time.Now().UnixNano()&0xffff)

	// El reloj de los receptores, para mostrar un código caducado.
	var muReloj sync.Mutex
	desfase := time.Duration(0)
	reloj := func() time.Time { muReloj.Lock(); defer muReloj.Unlock(); return time.Now().Add(desfase) }

	ctrl, err := core.NuevaIdentidad("controlador")
	if err != nil {
		return err
	}
	holaC := &lazulv1.Hola{EquipoId: "controlador-" + sufijo, Nombre: "Controlador", Sistema: lazulv1.Sistema_SISTEMA_LINUX, VersionApp: "sim"}
	// La persona en cada receptor ve nombre y huella corta, y la compara con
	// la que muestra el controlador: acepta solo si coincide.
	var vio []string
	permiso := func(nombre string) func(context.Context, core.Solicitud) bool {
		return func(_ context.Context, s core.Solicitud) bool {
			ok := s.Huella == ctrl.Huella
			que := "acepta"
			if !ok {
				que = "rechaza: no es la huella del controlador"
			}
			vio = append(vio, fmt.Sprintf("%s ve «%s» %s y %s", nombre, s.Nombre, s.Huella.Corta(), que))
			return ok
		}
	}

	rs := make([]*receptor, n)
	for i := range rs {
		nombre := fmt.Sprintf("receptor-%d", i+1)
		id, err := core.NuevaIdentidad(nombre)
		if err != nil {
			return err
		}
		f := falso.New(nombre, "firefox")
		ln, err := net.Listen("tcp", escucha)
		if err != nil {
			return err
		}
		r := &core.Receptor{Sistema: f.Sistema(), Hola: &lazulv1.Hola{
			EquipoId: nombre + "-" + sufijo, Nombre: nombre, Sistema: lazulv1.Sistema_SISTEMA_LINUX, VersionApp: "sim",
			Capacidades: []lazulv1.Capacidad{lazulv1.Capacidad_CAPACIDAD_CAPTURA, lazulv1.Capacidad_CAPACIDAD_PORTAPAPELES,
				lazulv1.Capacidad_CAPACIDAD_APP_ACTIVA, lazulv1.Capacidad_CAPACIDAD_ABRIR_APP},
		}}
		p := &core.Puerta{Identidad: id, Pares: core.NuevosPares(), Permiso: permiso(nombre), Reloj: reloj}
		go core.Servir(ctx, ln, r, p)
		detener, err := red.Anunciar(descubre.Anuncio{EquipoID: r.Hola.EquipoId, Nombre: nombre, Huella: id.Huella.String(), Dir: ln.Addr().String()})
		if err != nil {
			return err
		}
		defer detener()
		rs[i] = &receptor{nombre: nombre, falso: f, puerta: p}
	}

	// Descubrir: el controlador busca y se queda con la dirección y la huella
	// anunciadas de cada receptor.
	ctxB, cancelB := context.WithTimeout(ctx, 3*time.Second)
	anuncios, err := red.Buscar(ctxB)
	cancelB()
	if err != nil && len(anuncios) == 0 {
		return err
	}
	anunciadas := map[string]core.Huella{}
	for _, r := range rs {
		for _, a := range anuncios {
			if a.EquipoID != nombreID(r, sufijo) { // en la red de verdad puede haber otros
				continue
			}
			h, err := core.ParsearHuella(a.Huella)
			if err != nil {
				return err
			}
			r.dir, anunciadas[r.nombre] = a.Dir, h
		}
		if r.dir == "" {
			return fmt.Errorf("mDNS (%s) no encontró a %s", dondeRed, r.nombre)
		}
	}
	fmt.Fprintf(out, "descubrir: %d receptores anunciados por mDNS %s; el controlador los encontró a todos\n", n, dondeRed)

	a, b := rs[0], rs[1]
	conectar := func(r *receptor, yo *core.Identidad, h core.Huella) (*core.Conexion, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return core.Conectar(ctx, r.dir, yo, h)
	}
	emparejar := func(r *receptor, yo *core.Identidad, hola *lazulv1.Hola, codigo string) (core.Par, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return core.Emparejar(ctx, r.dir, yo, hola, codigo, anunciadas[r.nombre])
	}

	// 1. Sin emparejar no entra nadie, ni el controlador.
	if _, err := conectar(a, ctrl, anunciadas[a.nombre]); !errors.Is(err, core.ErrNoEmparejado) {
		return fmt.Errorf("sin emparejar debía rechazar: %v", err)
	}
	fmt.Fprintf(out, "rechazo: el controlador sin emparejar llama a %s → rechazado (%v)\n", a.nombre, core.ErrNoEmparejado)

	// 2. Código erróneo y código caducado.
	codigo, err := a.puerta.AbrirEmparejamiento()
	if err != nil {
		return err
	}
	if _, err := emparejar(a, ctrl, holaC, otroCodigo(codigo)); !errors.Is(err, core.ErrCodigoErroneo) {
		return fmt.Errorf("código erróneo: %v", err)
	} else {
		fmt.Fprintf(out, "código erróneo: %s muestra %s, el controlador escribe %s → rechazado (%v)\n", a.nombre, codigo, otroCodigo(codigo), err)
	}
	muReloj.Lock()
	desfase = core.DuracionCodigo + time.Second
	muReloj.Unlock()
	if _, err := emparejar(a, ctrl, holaC, codigo); !errors.Is(err, core.ErrCaducado) {
		return fmt.Errorf("código caducado: %v", err)
	}
	muReloj.Lock()
	desfase = 0
	muReloj.Unlock()
	fmt.Fprintf(out, "código caducado: %s escrito a los %v → rechazado\n", codigo, core.DuracionCodigo+time.Second)

	// 3. Emparejar con cada receptor: código bueno y permiso de la persona.
	pares := core.NuevosPares()
	for _, r := range rs {
		codigo, err := r.puerta.AbrirEmparejamiento()
		if err != nil {
			return err
		}
		par, err := emparejar(r, ctrl, holaC, codigo)
		if err != nil {
			return fmt.Errorf("emparejar %s: %w", r.nombre, err)
		}
		if err := pares.Fijar(par); err != nil {
			return err
		}
		fmt.Fprintf(out, "emparejar: %s muestra %s; %s; huellas fijadas en los dos lados (%s)\n", r.nombre, codigo, vio[len(vio)-1], par.Huella.Corta())
	}

	// 4. Un intruso que vio el código por encima del hombro: la persona en el
	// receptor ve otra huella y dice que no; tampoco puede mandar Órdenes.
	intruso, err := core.NuevaIdentidad("intruso")
	if err != nil {
		return err
	}
	codigo, _ = b.puerta.AbrirEmparejamiento()
	if _, err := emparejar(b, intruso, &lazulv1.Hola{EquipoId: "intruso", Nombre: "Controlador"}, codigo); !errors.Is(err, core.ErrRechazado) {
		return fmt.Errorf("intruso: %v", err)
	}
	fmt.Fprintf(out, "intruso: con el código bueno de %s; %s\n", b.nombre, vio[len(vio)-1])
	if _, err := conectar(b, intruso, anunciadas[b.nombre]); !errors.Is(err, core.ErrNoEmparejado) {
		return fmt.Errorf("intruso conectó: %v", err)
	}
	fmt.Fprintf(out, "intruso: manda una Orden a %s → rechazado (%v)\n", b.nombre, core.ErrNoEmparejado)

	// 5. Un impostor contesta donde estaba el receptor, con otra clave.
	impostorID, _ := core.NuevaIdentidad("impostor")
	lnI, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	pI := &core.Puerta{Identidad: impostorID, Pares: core.NuevosPares()}
	_ = pI.Pares.Fijar(core.Par{Huella: ctrl.Huella, Nombre: "Controlador"})
	go core.Servir(ctx, lnI, &core.Receptor{Sistema: falso.New("impostor").Sistema(), Hola: &lazulv1.Hola{Nombre: a.nombre}}, pI)
	parA, _ := pares.Buscar(anunciadas[a.nombre])
	if _, err := conectar(&receptor{dir: lnI.Addr().String()}, ctrl, parA.Huella); !errors.Is(err, core.ErrHuellaCambiada) {
		return fmt.Errorf("impostor: %v", err)
	}
	fmt.Fprintf(out, "impostor: otro equipo se hace pasar por %s → el controlador lo rechaza (%v)\n", a.nombre, core.ErrHuellaCambiada)

	// 6. Ya emparejados: conexiones TLS con la huella fijada.
	for _, r := range rs {
		par, _ := pares.Buscar(anunciadas[r.nombre])
		con, err := conectar(r, ctrl, par.Huella)
		if err != nil {
			return fmt.Errorf("conectar %s: %w", r.nombre, err)
		}
		defer con.Cerrar()
		r.con = con
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
	fmt.Fprintf(out, "receptores: %d conectados por TLS 1.3 con la huella fijada\n", n)

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

func nombreID(r *receptor, sufijo string) string { return r.nombre + "-" + sufijo }

func otroCodigo(c string) string {
	b := []byte(c)
	b[0] = '0' + (b[0]-'0'+1)%10
	return string(b)
}
