package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rafael088/lazul/core"
	"github.com/Rafael088/lazul/core/apps"
	"github.com/Rafael088/lazul/core/descubre"
	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// PuertoLazul es el de LocalSend: el que el cortafuegos de Windows ya deja
// entrar desde la red local.
const PuertoLazul = 53317

// Opciones del Nucleo.
type Opciones struct {
	// Dir guarda identidad.pem, pares.json (quién puede mandarle a este
	// equipo), destinos.json (a quién manda este) y recibidos/.
	Dir     string
	Nombre  string
	Escucha string // por defecto ":53317"
	Sistema platform.Sistema
	Reg     *Registro
	// Permiso le pregunta a la persona en este equipo si deja entrar a un
	// controlador que dio bien el código. nil rechaza siempre.
	Permiso func(context.Context, core.Solicitud) bool
	// Adelantar sube la captura al agarrar (ver core.Enrutador).
	Adelantar bool
	// MDNS anuncia este equipo y deja buscar los demás en la red local.
	MDNS bool
}

// Destino es un receptor al que este equipo manda (lo emparejó con su código).
type Destino struct {
	Huella string    `json:"huella"`
	Nombre string    `json:"nombre"`
	Dir    string    `json:"dir"` // host:puerto
	Desde  time.Time `json:"desde"`
}

// Equipo es una fila de la lista de equipos de la ventana.
type Equipo struct {
	Nombre     string `json:"nombre"`
	Huella     string `json:"huella"` // corta
	Dir        string `json:"dir,omitempty"`
	Rol        string `json:"rol"` // "le mando" o "me manda"
	Elegido    bool   `json:"elegido,omitempty"`
	Conectado  bool   `json:"conectado,omitempty"`
	Encontrado bool   `json:"encontrado,omitempty"` // por mDNS, sin emparejar
}

// EstadoRed es lo que la ventana muestra de la red.
type EstadoRed struct {
	Nombre      string   `json:"nombre"`
	Huella      string   `json:"huella"` // corta, para comparar en las dos pantallas
	Direcciones []string `json:"direcciones"`
	Escuchando  bool     `json:"escuchando"`
	Error       string   `json:"error,omitempty"`
	Codigo      string   `json:"codigo,omitempty"`
	CodigoVence string   `json:"codigo_vence,omitempty"`
	Destino     string   `json:"destino,omitempty"`
	Conectado   bool     `json:"conectado"`
	ErrorDest   string   `json:"error_destino,omitempty"`
	Accion      string   `json:"accion,omitempty"` // la última, en palabras
	AccionHora  string   `json:"accion_hora,omitempty"`
	Recibido    string   `json:"recibido,omitempty"`
	RecibidoH   string   `json:"recibido_hora,omitempty"`
	Equipos     []Equipo `json:"equipos"`
}

// Nucleo junta el receptor y el controlador de este equipo.
type Nucleo struct {
	op       Opciones
	id       *core.Identidad
	pares    *core.Pares
	puerta   *core.Puerta
	receptor *core.Receptor
	cat      apps.Catalogo
	enr      *core.Enrutador // solo lo toca trabajar
	gestos   chan Disparo

	mu          sync.Mutex
	ln          net.Listener
	destinos    []Destino
	elegido     string // huella hex del destino
	remoto      *core.Remoto
	remotoDe    string
	conectado   bool
	errDest     string
	errRed      string
	codigo      string
	codigoVence time.Time
	accion      string
	accionT     time.Time
	recibido    string
	recibidoT   time.Time
	encontrados []descubre.Anuncio
}

// NuevoNucleo carga (o crea) la identidad y los equipos de op.Dir.
func NuevoNucleo(op Opciones) (*Nucleo, error) {
	if op.Escucha == "" {
		op.Escucha = ":" + strconv.Itoa(PuertoLazul)
	}
	if op.Nombre == "" {
		op.Nombre = NombrePorDefecto()
	}
	if err := os.MkdirAll(op.Dir, 0o700); err != nil {
		return nil, err
	}
	id, err := core.CargarOCrearIdentidad(filepath.Join(op.Dir, "identidad.pem"), op.Nombre)
	if err != nil {
		return nil, err
	}
	pares, err := core.AbrirPares(filepath.Join(op.Dir, "pares.json"))
	if err != nil {
		return nil, err
	}
	cat, err := apps.Cargar()
	if err != nil {
		return nil, err
	}
	n := &Nucleo{op: op, id: id, pares: pares, cat: cat, gestos: make(chan Disparo, 8)}
	if err := n.leerDestinos(); err != nil {
		return nil, err
	}
	if len(n.destinos) > 0 {
		n.elegido = n.destinos[len(n.destinos)-1].Huella
	}
	n.puerta = &core.Puerta{Identidad: id, Pares: pares, Permiso: func(ctx context.Context, s core.Solicitud) bool {
		// Si se pregunta es que el código estuvo bien, y ya se gastó (un solo uso).
		n.mu.Lock()
		n.codigo = ""
		n.mu.Unlock()
		return op.Permiso != nil && op.Permiso(ctx, s)
	}}
	n.receptor = &core.Receptor{Sistema: op.Sistema, Catalogo: &n.cat,
		Hola: &lazulv1.Hola{EquipoId: id.Huella.String()[:16], Nombre: op.Nombre, Sistema: sistemaProto(), VersionApp: "0.1.0-prototipo",
			Capacidades: []lazulv1.Capacidad{lazulv1.Capacidad_CAPACIDAD_CAPTURA, lazulv1.Capacidad_CAPACIDAD_APP_ACTIVA,
				lazulv1.Capacidad_CAPACIDAD_ABRIR_APP, lazulv1.Capacidad_CAPACIDAD_RECIBIR_ARCHIVO}},
		Archivos: &localsend.Servidor{Bandeja: filepath.Join(op.Dir, "recibidos"), Info: localsend.Dispositivo{Alias: op.Nombre, Version: "2.0",
			DeviceType: "desktop", Fingerprint: id.Huella.String(), Protocol: "https"}},
		Registro: n.alRecibir,
	}
	n.enr = &core.Enrutador{Local: op.Sistema, Catalogo: &n.cat, Adelantar: op.Adelantar,
		Yo: localsend.Dispositivo{Alias: op.Nombre, Version: "2.0", DeviceType: "desktop", Fingerprint: id.Huella.String(), Protocol: "https"}}
	return n, nil
}

// NombrePorDefecto: «PC Linux» o «PC Windows».
func NombrePorDefecto() string {
	return "PC " + strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
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

// Huella completa (hex) de este equipo.
func (n *Nucleo) Huella() string { return n.id.Huella.String() }

// Dir es la dirección en que escucha (útil con Escucha ":0").
func (n *Nucleo) Dir() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.ln == nil {
		return ""
	}
	return n.ln.Addr().String()
}

// Arrancar escucha (receptor), anuncia por mDNS si se pidió y deja corriendo
// el trabajo del controlador hasta que ctx termine. Si no puede escuchar
// (puerto ocupado), lo dice en el estado y sigue como solo controlador.
func (n *Nucleo) Arrancar(ctx context.Context) {
	ln, err := net.Listen("tcp", n.op.Escucha)
	if err != nil {
		n.mu.Lock()
		n.errRed = "no puedo escuchar en " + n.op.Escucha + ": " + err.Error()
		n.mu.Unlock()
	} else {
		n.mu.Lock()
		n.ln = ln
		n.mu.Unlock()
		go func() {
			if err := core.Servir(ctx, ln, n.receptor, n.puerta); err != nil {
				n.mu.Lock()
				n.errRed = "receptor: " + err.Error()
				n.mu.Unlock()
			}
		}()
		if n.op.MDNS {
			detener, err := descubre.MDNS{}.Anunciar(descubre.Anuncio{EquipoID: n.id.Huella.String()[:16], Nombre: n.op.Nombre,
				Huella: n.id.Huella.String(), Dir: ln.Addr().String()})
			if err == nil {
				go func() { <-ctx.Done(); detener() }()
			}
		}
	}
	go n.trabajar(ctx)
}

// AbrirCodigo abre el emparejamiento (anula el código anterior).
func (n *Nucleo) AbrirCodigo() (string, error) {
	c, err := n.puerta.AbrirEmparejamiento()
	if err != nil {
		return "", err
	}
	n.mu.Lock()
	n.codigo, n.codigoVence = c, time.Now().Add(core.DuracionCodigo)
	n.mu.Unlock()
	return c, nil
}

// CerrarCodigo cierra el emparejamiento.
func (n *Nucleo) CerrarCodigo() {
	n.puerta.CerrarEmparejamiento()
	n.mu.Lock()
	n.codigo = ""
	n.mu.Unlock()
}

// Emparejar pide emparejarse al receptor en dir con su código. anunciada es
// la huella que vino por mDNS (vacía si la dirección se escribió a mano).
// El receptor pregunta a la persona; esto espera hasta su respuesta.
func (n *Nucleo) Emparejar(ctx context.Context, dir, codigo, anunciada string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("falta la dirección del otro equipo")
	}
	if _, _, err := net.SplitHostPort(dir); err != nil {
		dir = net.JoinHostPort(dir, strconv.Itoa(PuertoLazul))
	}
	var h core.Huella
	if anunciada != "" {
		var err error
		if h, err = core.ParsearHuella(anunciada); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, core.EsperaPermiso+15*time.Second)
	defer cancel()
	par, err := core.Emparejar(ctx, dir, n.id, &lazulv1.Hola{EquipoId: n.id.Huella.String()[:16], Nombre: n.op.Nombre, Sistema: sistemaProto()}, codigo, h)
	if err != nil {
		return err
	}
	d := Destino{Huella: par.Huella.String(), Nombre: par.Nombre, Dir: dir, Desde: time.Now()}
	n.mu.Lock()
	n.destinos = slices.DeleteFunc(n.destinos, func(x Destino) bool { return x.Huella == d.Huella })
	n.destinos = append(n.destinos, d)
	n.elegido = d.Huella
	n.conectado, n.errDest = false, ""
	err = n.guardarDestinos()
	n.mu.Unlock()
	return err
}

// Elegir decide a qué equipo van los gestos (huella hex o corta).
func (n *Nucleo) Elegir(huella string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, d := range n.destinos {
		h, _ := core.ParsearHuella(d.Huella)
		if d.Huella == huella || h.Corta() == huella {
			n.elegido = d.Huella
			return nil
		}
	}
	return errors.New("ese equipo no está emparejado")
}

// Buscar escucha mDNS unos segundos y guarda lo encontrado para la lista.
func (n *Nucleo) Buscar(ctx context.Context) ([]descubre.Anuncio, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	l, err := descubre.MDNS{}.Buscar(ctx)
	propia := n.id.Huella.String()
	l = slices.DeleteFunc(l, func(a descubre.Anuncio) bool { return a.Huella == propia })
	n.mu.Lock()
	n.encontrados = l
	n.mu.Unlock()
	return l, err
}

// Disparar le pasa un gesto al trabajo del controlador sin bloquear.
func (n *Nucleo) Disparar(d Disparo) {
	select {
	case n.gestos <- d:
	default:
		n.op.Reg.Anotar(Linea{Tipo: TipoAccion, Sesion: d.Sesion, Gesto: nombreGesto(d.Evento.Gesto), Activacion: d.Activacion,
			Resultado: "descartada", Error: "cola_llena"})
	}
}

func (n *Nucleo) decir(f string, a ...any) {
	n.mu.Lock()
	n.accion, n.accionT = fmt.Sprintf(f, a...), time.Now()
	n.mu.Unlock()
}

// trabajar es la única gorrutina que toca el Enrutador y la conexión con el
// destino: los gestos van en orden y, entre uno y otro, vigila la conexión.
func (n *Nucleo) trabajar(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	defer n.soltarConexion()
	for {
		select {
		case <-ctx.Done():
			return
		case d := <-n.gestos:
			n.hacer(ctx, d)
		case <-tick.C:
			n.vigilar(ctx)
		}
	}
}

func (n *Nucleo) soltarConexion() {
	n.mu.Lock()
	r := n.remoto
	n.remoto, n.remotoDe, n.conectado = nil, "", false
	n.mu.Unlock()
	if r != nil {
		_ = r.Con.Cerrar()
	}
	n.enr.PorDefecto = nil
}

// conexion devuelve el Remoto del destino elegido, conectándose si hace falta.
func (n *Nucleo) conexion(ctx context.Context) (*core.Remoto, error) {
	n.mu.Lock()
	elegido := n.elegido
	var dest Destino
	for _, d := range n.destinos {
		if d.Huella == elegido {
			dest = d
		}
	}
	r, de := n.remoto, n.remotoDe
	n.mu.Unlock()
	if dest.Huella == "" {
		return nil, errSinDestino
	}
	if r != nil && de == elegido {
		return r, nil
	}
	if r != nil {
		n.soltarConexion()
	}
	h, err := core.ParsearHuella(dest.Huella)
	if err != nil {
		return nil, err
	}
	ctxC, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	con, err := core.Conectar(ctxC, dest.Dir, n.id, h)
	if err != nil {
		n.mu.Lock()
		n.conectado, n.errDest = false, err.Error()
		n.mu.Unlock()
		return nil, err
	}
	r = &core.Remoto{Nombre: dest.Nombre, Con: con, HTTP: core.ClienteHTTP(n.id, h), Base: "https://" + dest.Dir}
	if _, err := r.Pedir(ctxC, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Ping{Ping: &lazulv1.Ping{}}}); err != nil {
		_ = con.Cerrar()
		n.mu.Lock()
		n.conectado, n.errDest = false, err.Error()
		n.mu.Unlock()
		return nil, err
	}
	n.mu.Lock()
	n.remoto, n.remotoDe, n.conectado, n.errDest = r, elegido, true, ""
	n.mu.Unlock()
	n.enr.PorDefecto = r
	return r, nil
}

func (n *Nucleo) vigilar(ctx context.Context) {
	r, err := n.conexion(ctx)
	if err != nil || r == nil {
		return
	}
	ctxP, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := r.Pedir(ctxP, &lazulv1.Sobre{Contenido: &lazulv1.Sobre_Ping{Ping: &lazulv1.Ping{}}}); err != nil {
		n.soltarConexion()
		n.mu.Lock()
		n.errDest = "se perdió la conexión: " + err.Error()
		n.mu.Unlock()
	}
}

var errSinDestino = errors.New("no hay ningún equipo emparejado al que mandar (empareja uno)")

// ClaseError reduce un error a una palabra para el registro (el texto puede
// llevar rutas o direcciones: ese solo se muestra en la ventana).
func ClaseError(err error) string {
	var ne net.Error
	switch {
	case err == nil:
		return ""
	case errors.Is(err, core.ErrNadaAgarrado):
		return "nada_agarrado"
	case errors.Is(err, errSinDestino):
		return "sin_destino"
	case errors.Is(err, platform.ErrNoImplementado):
		return "no_implementado"
	case errors.Is(err, platform.ErrSinVentana):
		return "sin_ventana"
	case errors.Is(err, context.DeadlineExceeded):
		return "tiempo"
	case errors.Is(err, core.ErrHuellaCambiada), errors.Is(err, core.ErrNoEmparejado):
		return "confianza"
	case errors.As(err, &ne):
		return "red"
	}
	return "otro"
}

func (n *Nucleo) hacer(ctx context.Context, d Disparo) {
	g := nombreGesto(d.Evento.Gesto)
	l := Linea{Tipo: TipoAccion, Sesion: d.Sesion, Gesto: g, Direccion: nombreDir(d.Evento.Direccion), Activacion: d.Activacion, Prueba: d.Prueba}
	if d.Prueba {
		l.Resultado = "no_mandada_prueba"
		n.op.Reg.Anotar(l)
		n.decir("%s reconocido en la prueba de trabajo normal: no se mandó nada (cuenta como activación falsa)", g)
		return
	}
	ev := &lazulv1.EventoGesto{Gesto: d.Evento.Gesto, Confianza: d.Evento.Confianza, Direccion: d.Evento.Direccion}
	var h core.Hecho
	var err error
	for intento := range 2 {
		if _, err = n.conexion(ctx); err != nil && d.Evento.Gesto != lazulv1.Gesto_GESTO_AGARRAR {
			break // agarrar no necesita al otro (solo para adelantar la subida)
		}
		ctxG, cancel := context.WithTimeout(ctx, 15*time.Second)
		h, err = n.enr.Gesto(ctxG, ev)
		cancel()
		if err == nil || intento == 1 || errors.Is(err, core.ErrNadaAgarrado) || errors.Is(err, platform.ErrNoImplementado) {
			break
		}
		// La conexión pudo caerse (el otro se reinició): otra vez, conectando de nuevo.
		n.soltarConexion()
	}
	l.Que, l.Destino, l.Ms = h.Que, h.Destino, h.Total.Milliseconds()
	if err != nil {
		l.Resultado, l.Error = "falla", ClaseError(err)
		n.op.Reg.Anotar(l)
		n.decir("%s: no se pudo: %v", g, err)
		return
	}
	l.Resultado = "ok"
	n.op.Reg.Anotar(l)
	switch d.Evento.Gesto {
	case lazulv1.Gesto_GESTO_AGARRAR:
		n.decir("Agarrada una captura de esta pantalla (%d ms); suéltala abriendo la mano hacia un lado", h.Total.Milliseconds())
	case lazulv1.Gesto_GESTO_SOLTAR:
		n.decir("Captura soltada en «%s» en %d ms", h.Destino, h.Total.Milliseconds())
	default:
		n.decir("Ventana activa mandada a «%s» como %s en %d ms", h.Destino, h.Que, h.Total.Milliseconds())
	}
}

// alRecibir: el receptor atendió una Orden. Se anota el tipo, nunca la URL
// ni el nombre del archivo.
func (n *Nucleo) alRecibir(o *lazulv1.Orden, resp *lazulv1.Respuesta, d time.Duration) {
	que := "tomar"
	if obj := o.GetEntregar().GetObjeto(); obj != nil {
		switch {
		case obj.GetUrl() != "":
			que = "url"
		case obj.GetApp() != nil:
			que = "app"
		case obj.GetArchivo() != nil:
			que = "archivo"
		default:
			que = "otro"
		}
	}
	l := Linea{Tipo: TipoRecibido, Que: que, Ms: d.Milliseconds(), Resultado: "ok"}
	texto := "Llegó " + que + " y se abrió aquí"
	if !resp.GetOk() {
		l.Resultado, l.Error = "falla", strings.ToLower(strings.TrimPrefix(resp.GetFalla().GetCodigo().String(), "CODIGO_"))
		texto = "Llegó " + que + " pero no se pudo abrir: " + resp.GetFalla().GetDetalle()
	}
	n.op.Reg.Anotar(l)
	n.mu.Lock()
	n.recibido, n.recibidoT = texto, time.Now()
	n.mu.Unlock()
}

// Estado para la ventana.
func (n *Nucleo) Estado() EstadoRed {
	n.mu.Lock()
	defer n.mu.Unlock()
	e := EstadoRed{Nombre: n.op.Nombre, Huella: n.id.Huella.Corta(), Escuchando: n.ln != nil, Error: n.errRed,
		Conectado: n.conectado, ErrorDest: n.errDest, Accion: n.accion, Recibido: n.recibido}
	if n.ln != nil {
		e.Direcciones = direcciones(n.ln.Addr())
	}
	if n.codigo != "" && time.Now().Before(n.codigoVence) {
		e.Codigo, e.CodigoVence = n.codigo, n.codigoVence.Format("15:04:05")
	}
	if !n.accionT.IsZero() {
		e.AccionHora = n.accionT.Format("15:04:05")
	}
	if !n.recibidoT.IsZero() {
		e.RecibidoH = n.recibidoT.Format("15:04:05")
	}
	emparejados := map[string]bool{}
	for _, d := range n.destinos {
		h, _ := core.ParsearHuella(d.Huella)
		eq := Equipo{Nombre: d.Nombre, Huella: h.Corta(), Dir: d.Dir, Rol: "le mando", Elegido: d.Huella == n.elegido}
		eq.Conectado = eq.Elegido && n.conectado
		if eq.Elegido {
			e.Destino = d.Nombre
		}
		e.Equipos = append(e.Equipos, eq)
		emparejados[d.Huella] = true
	}
	for _, p := range n.pares.Lista() {
		e.Equipos = append(e.Equipos, Equipo{Nombre: p.Nombre, Huella: p.Huella.Corta(), Rol: "me manda"})
	}
	for _, a := range n.encontrados {
		if emparejados[a.Huella] {
			continue
		}
		h, err := core.ParsearHuella(a.Huella)
		if err != nil {
			continue
		}
		e.Equipos = append(e.Equipos, Equipo{Nombre: a.Nombre, Huella: h.Corta(), Dir: a.Dir, Rol: "en la red", Encontrado: true})
	}
	return e
}

// Encontrado devuelve la huella completa de un equipo encontrado por mDNS
// con esa dirección (para pasarla a Emparejar).
func (n *Nucleo) Encontrado(dir string) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, a := range n.encontrados {
		if a.Dir == dir {
			return a.Huella
		}
	}
	return ""
}

// direcciones: las IPv4 de la red local con el puerto, para escribirlas en
// el otro equipo.
func direcciones(a net.Addr) []string {
	_, puerto, _ := net.SplitHostPort(a.String())
	var l []string
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, ad := range addrs {
			if ipn, ok := ad.(*net.IPNet); ok && ipn.IP.To4() != nil && ipn.IP.IsPrivate() {
				l = append(l, net.JoinHostPort(ipn.IP.String(), puerto))
			}
		}
	}
	slices.Sort(l)
	return l
}

func (n *Nucleo) rutaDestinos() string { return filepath.Join(n.op.Dir, "destinos.json") }

func (n *Nucleo) leerDestinos() error {
	b, err := os.ReadFile(n.rutaDestinos())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var ds []Destino
	if err := json.Unmarshal(b, &ds); err != nil {
		return fmt.Errorf("destinos.json: %w", err)
	}
	for _, d := range ds {
		if _, err := core.ParsearHuella(d.Huella); err != nil {
			return fmt.Errorf("destinos.json: %w", err)
		}
	}
	n.destinos = ds
	return nil
}

func (n *Nucleo) guardarDestinos() error {
	b, err := json.MarshalIndent(n.destinos, "", "  ")
	if err != nil {
		return err
	}
	tmp := n.rutaDestinos() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, n.rutaDestinos())
}
