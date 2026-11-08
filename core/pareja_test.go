package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Rafael088/lazul/platform/falso"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// banco: un receptor escuchando en 127.0.0.1 con TLS y un controlador.
type banco struct {
	puerta *Puerta
	dir    string
	ctrl   *Identidad
	hola   *lazulv1.Hola

	mu    sync.Mutex
	ahora time.Time
	vio   []Solicitud
}

func (b *banco) reloj() time.Time { b.mu.Lock(); defer b.mu.Unlock(); return b.ahora }
func (b *banco) avanzar(d time.Duration) {
	b.mu.Lock()
	b.ahora = b.ahora.Add(d)
	b.mu.Unlock()
}

func nuevoBanco(t *testing.T, acepta bool) *banco {
	t.Helper()
	b := &banco{ahora: time.Now(), hola: &lazulv1.Hola{EquipoId: "ctrl", Nombre: "Controlador"}}
	idR, err := NuevaIdentidad("receptor")
	if err != nil {
		t.Fatal(err)
	}
	b.ctrl, _ = NuevaIdentidad("controlador")
	b.puerta = &Puerta{Identidad: idR, Pares: NuevosPares(), Reloj: b.reloj,
		Permiso: func(_ context.Context, s Solicitud) bool {
			b.mu.Lock()
			b.vio = append(b.vio, s)
			b.mu.Unlock()
			return acepta
		}}
	b.dir = servir(t, b.puerta)
	return b
}

func servir(t *testing.T, p *Puerta) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &Receptor{Sistema: falso.New("r").Sistema(), Hola: &lazulv1.Hola{EquipoId: "rec", Nombre: "Receptor"}}
	go Servir(ctx, ln, r, p)
	return ln.Addr().String()
}

func (b *banco) emparejar(t *testing.T, codigo string) (Par, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return Emparejar(ctx, b.dir, b.ctrl, b.hola, codigo, Huella{})
}

func ping(t *testing.T, c *Conexion) *lazulv1.Sobre {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Enviar(ctx, &lazulv1.Sobre{Version: 1, Id: "p", Contenido: &lazulv1.Sobre_Ping{Ping: &lazulv1.Ping{}}}); err != nil {
		t.Fatal(err)
	}
	s, err := c.Recibir(ctx)
	if err != nil {
		return nil
	}
	return s
}

func conectar(dir string, yo *Identidad, h Huella) (*Conexion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return Conectar(ctx, dir, yo, h)
}

func TestEmparejarYConectar(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, err := b.puerta.AbrirEmparejamiento()
	if err != nil {
		t.Fatal(err)
	}
	par, err := b.emparejar(t, codigo)
	if err != nil {
		t.Fatal(err)
	}
	if par.Huella != b.puerta.Identidad.Huella || par.Nombre != "Receptor" {
		t.Fatalf("el controlador fijó %v", par)
	}
	if got, ok := b.puerta.Pares.Buscar(b.ctrl.Huella); !ok || got.Nombre != "Controlador" {
		t.Fatalf("el receptor fijó %v %v", got, ok)
	}
	if len(b.vio) != 1 || b.vio[0].Huella != b.ctrl.Huella || b.vio[0].Nombre != "Controlador" {
		t.Fatalf("la persona vio %v", b.vio)
	}
	c, err := conectar(b.dir, b.ctrl, par.Huella)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Cerrar()
	if s := ping(t, c); s.GetPong() == nil {
		t.Fatalf("sin Pong: %v", s)
	}
}

func TestNoEmparejadoEsRechazado(t *testing.T) {
	b := nuevoBanco(t, true)
	_, err := conectar(b.dir, b.ctrl, b.puerta.Identidad.Huella)
	if !errors.Is(err, ErrNoEmparejado) {
		t.Fatalf("esperaba ErrNoEmparejado, fue %v", err)
	}
}

func TestSinCertificadoNiTLSEsRechazado(t *testing.T) {
	b := nuevoBanco(t, true)
	if _, err := b.puerta.AbrirEmparejamiento(); err != nil {
		t.Fatal(err)
	}
	// TLS sin certificado de cliente: el apretón de manos falla.
	conn, err := tls.Dial("tcp", b.dir, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13})
	if err == nil {
		_, err = conn.Read(make([]byte, 1))
		conn.Close()
	}
	if err == nil {
		t.Fatal("aceptó TLS sin certificado de cliente")
	}
	// Sin TLS.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := websocket.Dial(ctx, "ws://"+b.dir+"/lazul", nil); err == nil {
		t.Fatal("aceptó WebSocket sin TLS")
	}
}

func TestCodigoErroneoNoEmpareja(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	malo := otroCodigo(codigo)
	for i := IntentosCodigo - 1; i >= 0; i-- {
		_, err := b.emparejar(t, malo)
		var fp *FallaPareja
		if !errors.Is(err, ErrCodigoErroneo) || !errors.As(err, &fp) || fp.Restantes != i {
			t.Fatalf("con %d restantes: %v", i, err)
		}
	}
	// Agotados los intentos, el bueno tampoco entra: hay que abrir otro.
	if _, err := b.emparejar(t, codigo); !errors.Is(err, ErrCaducado) {
		t.Fatalf("tras agotar: %v", err)
	}
	if len(b.puerta.Pares.Lista()) != 0 || len(b.vio) != 0 {
		t.Fatalf("no debía fijar ni preguntar: %v %v", b.puerta.Pares.Lista(), b.vio)
	}
}

func TestCodigoCaducadoNoEmpareja(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	b.avanzar(DuracionCodigo + time.Second)
	if _, err := b.emparejar(t, codigo); !errors.Is(err, ErrCaducado) {
		t.Fatalf("esperaba ErrCaducado: %v", err)
	}
	if len(b.puerta.Pares.Lista()) != 0 {
		t.Fatal("fijó con un código vencido")
	}
}

func TestSinEmparejamientoAbierto(t *testing.T) {
	b := nuevoBanco(t, true)
	if _, err := b.emparejar(t, "123456"); !errors.Is(err, ErrCerrado) {
		t.Fatalf("esperaba ErrCerrado: %v", err)
	}
}

func TestCodigoSirveUnaVezEnLaRed(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	if _, err := b.emparejar(t, codigo); err != nil {
		t.Fatal(err)
	}
	otro, _ := NuevaIdentidad("intruso")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := Emparejar(ctx, b.dir, otro, &lazulv1.Hola{Nombre: "Intruso"}, codigo, Huella{}); !errors.Is(err, ErrCerrado) {
		t.Fatalf("reusó el código: %v", err)
	}
}

func TestPersonaRechazaEnElReceptor(t *testing.T) {
	b := nuevoBanco(t, false)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	if _, err := b.emparejar(t, codigo); !errors.Is(err, ErrRechazado) {
		t.Fatalf("esperaba ErrRechazado: %v", err)
	}
	if len(b.vio) != 1 {
		t.Fatalf("no se le preguntó a la persona: %v", b.vio)
	}
	if _, err := conectar(b.dir, b.ctrl, b.puerta.Identidad.Huella); !errors.Is(err, ErrNoEmparejado) {
		t.Fatalf("rechazado y conecta: %v", err)
	}
}

func TestSinPermisoConfiguradoRechaza(t *testing.T) {
	b := nuevoBanco(t, true)
	b.puerta.Permiso = nil
	codigo, _ := b.puerta.AbrirEmparejamiento()
	if _, err := b.emparejar(t, codigo); !errors.Is(err, ErrRechazado) {
		t.Fatalf("sin quien pregunte debe rechazar: %v", err)
	}
}

func TestHuellaDelReceptorCambiada(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	par, err := b.emparejar(t, codigo)
	if err != nil {
		t.Fatal(err)
	}
	// Otro equipo (otra clave) contesta donde estaba el receptor y hasta
	// conoce al controlador: igual se rechaza por la huella.
	impostor, _ := NuevaIdentidad("impostor")
	p2 := &Puerta{Identidad: impostor, Pares: NuevosPares()}
	_ = p2.Pares.Fijar(Par{Huella: b.ctrl.Huella, Nombre: "Controlador"})
	dir2 := servir(t, p2)
	if _, err := conectar(dir2, b.ctrl, par.Huella); !errors.Is(err, ErrHuellaCambiada) {
		t.Fatalf("esperaba ErrHuellaCambiada: %v", err)
	}
}

func TestHuellaDelControladorCambiada(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	par, err := b.emparejar(t, codigo)
	if err != nil {
		t.Fatal(err)
	}
	nueva, _ := NuevaIdentidad("controlador")
	if _, err := conectar(b.dir, nueva, par.Huella); !errors.Is(err, ErrNoEmparejado) {
		t.Fatalf("controlador con otra clave: %v", err)
	}
}

func TestHuellaAnunciadaNoCoincide(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	otra, _ := NuevaIdentidad("otra")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := Emparejar(ctx, b.dir, b.ctrl, b.hola, codigo, otra.Huella); !errors.Is(err, ErrHuellaCambiada) {
		t.Fatalf("esperaba ErrHuellaCambiada: %v", err)
	}
	// No gastó el código: se cortó antes de mandar la prueba.
	if _, err := b.emparejar(t, codigo); err != nil {
		t.Fatalf("el código debía seguir vivo: %v", err)
	}
}

func TestOlvidarCortaLaConexion(t *testing.T) {
	b := nuevoBanco(t, true)
	codigo, _ := b.puerta.AbrirEmparejamiento()
	par, _ := b.emparejar(t, codigo)
	c, err := conectar(b.dir, b.ctrl, par.Huella)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Cerrar()
	_ = b.puerta.Pares.Olvidar(b.ctrl.Huella)
	s := ping(t, c)
	if s.GetRespuesta().GetFalla().GetCodigo() != lazulv1.Falla_CODIGO_SIN_PERMISO {
		t.Fatalf("tras olvidar: %v", s)
	}
}

// La prueba depende de la sesión TLS: la misma en los dos extremos y distinta
// en otra sesión, así que no se puede reenviar.
func TestPruebaAtadaALaSesion(t *testing.T) {
	a, _ := NuevaIdentidad("a")
	r, _ := NuevaIdentidad("r")
	sesion := func() (cli, srv tls.ConnectionState) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()
		tc := tls.Client(c1, configCliente(a, func(Huella) error { return nil }))
		ts := tls.Server(c2, configServidor(r))
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); _ = ts.Handshake() }()
		if err := tc.Handshake(); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		return tc.ConnectionState(), ts.ConnectionState()
	}
	c1, s1 := sesion()
	c2, _ := sesion()
	p := func(cs tls.ConnectionState, rol string) []byte {
		b, err := prueba("123456", rol, &cs, a.Huella, r.Huella)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if !bytes.Equal(p(c1, rolControlador), p(s1, rolControlador)) {
		t.Fatal("los dos extremos de la misma sesión no coinciden")
	}
	if bytes.Equal(p(c1, rolControlador), p(c2, rolControlador)) {
		t.Fatal("la prueba es igual en otra sesión: se podría reenviar")
	}
	if bytes.Equal(p(c1, rolControlador), p(c1, rolReceptor)) {
		t.Fatal("controlador y receptor mandan la misma prueba: se podría reflejar")
	}
}

func otroCodigo(c string) string {
	b := []byte(c)
	b[0] = '0' + (b[0]-'0'+1)%10
	return string(b)
}
