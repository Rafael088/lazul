package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/Rafael088/lazul/core/localsend"
	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Conexion lleva Sobres en mensajes binarios de WebSocket sobre TLS 1.3,
// con la huella del otro equipo fijada (ver pareja.go).
type Conexion struct{ ws *websocket.Conn }

// MaxSobre: los Sobres son de pocos bytes; los archivos van por LocalSend.
const MaxSobre = 1 << 20

func (c *Conexion) Enviar(ctx context.Context, s *lazulv1.Sobre) error {
	b, err := proto.Marshal(s)
	if err != nil {
		return err
	}
	return c.ws.Write(ctx, websocket.MessageBinary, b)
}

func (c *Conexion) Recibir(ctx context.Context) (*lazulv1.Sobre, error) {
	tipo, b, err := c.ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	if tipo != websocket.MessageBinary {
		return nil, fmt.Errorf("%w: mensaje de texto", errInvalido)
	}
	var s lazulv1.Sobre
	if err := proto.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalido, err)
	}
	return &s, nil
}

func (c *Conexion) Cerrar() error { return c.ws.Close(websocket.StatusNormalClosure, "") }

// Conectar abre una conexión con un receptor emparejado en dir (host:puerto).
// Falla con ErrHuellaCambiada si el certificado del receptor no es el de
// huella, y con ErrNoEmparejado si el receptor no conoce a este equipo.
func Conectar(ctx context.Context, dir string, yo *Identidad, huella Huella) (*Conexion, error) {
	if huella.cero() {
		return nil, errors.New("Conectar sin huella fijada: empareja primero")
	}
	c, _, err := dialTLS(ctx, "wss://"+dir+"/lazul", configCliente(yo, func(h Huella) error {
		if h != huella {
			return ErrHuellaCambiada
		}
		return nil
	}))
	return c, err
}

// ClienteHTTP es un cliente HTTPS que presenta la identidad propia y solo
// habla con el certificado de huella: es el que usa LocalSend para subirle
// archivos a un receptor emparejado.
func ClienteHTTP(yo *Identidad, huella Huella) *http.Client {
	cfg := configCliente(yo, func(h Huella) error {
		if huella.cero() || h != huella {
			return ErrHuellaCambiada
		}
		return nil
	})
	return &http.Client{Transport: &http.Transport{TLSClientConfig: cfg, ForceAttemptHTTP2: true}, Timeout: 2 * time.Minute}
}

func dialTLS(ctx context.Context, url string, cfg *tls.Config) (*Conexion, *tls.ConnectionState, error) {
	var fallo error
	verificar := cfg.VerifyConnection
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		fallo = verificar(cs)
		return fallo
	}
	tr := &http.Transport{TLSClientConfig: cfg}
	defer tr.CloseIdleConnections()
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: &http.Client{Transport: tr}})
	if err != nil {
		if fallo != nil {
			return nil, nil, fallo
		}
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			return nil, nil, ErrNoEmparejado
		}
		return nil, nil, err
	}
	if resp.TLS == nil {
		ws.CloseNow()
		return nil, nil, errors.New("conexión sin TLS")
	}
	ws.SetReadLimit(MaxSobre)
	return &Conexion{ws: ws}, resp.TLS, nil
}

// Servir escucha en ln con TLS y atiende hasta que ctx termine:
//   - /pareja: emparejamiento (solo hace algo con un código abierto);
//   - /lazul: Órdenes para r, solo de equipos emparejados en p.Pares. Se
//     vuelve a mirar en cada Sobre: olvidar un equipo corta su conexión.
func Servir(ctx context.Context, ln net.Listener, r *Receptor, p *Puerta) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/lazul", func(w http.ResponseWriter, req *http.Request) {
		if !p.emparejado(req.TLS) {
			http.Error(w, ErrNoEmparejado.Error(), http.StatusForbidden)
			return
		}
		ws, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		ws.SetReadLimit(MaxSobre)
		c := &Conexion{ws: ws}
		defer c.Cerrar()
		for {
			s, err := c.Recibir(ctx)
			if err != nil {
				return
			}
			if !p.emparejado(req.TLS) {
				_ = c.Enviar(ctx, &lazulv1.Sobre{Version: uint32(lazulv1.Version_VERSION_ACTUAL), Id: "r-" + s.GetId(), EnRespuestaA: s.GetId(),
					Contenido: respuestaFalla(lazulv1.Falla_CODIGO_SIN_PERMISO, ErrNoEmparejado.Error())})
				return
			}
			if resp := r.Atender(ctx, s); resp != nil {
				if err := c.Enviar(ctx, resp); err != nil {
					return
				}
			}
		}
	})
	if r.Archivos != nil {
		// LocalSend en el mismo puerto: solo suben los emparejados.
		r.Archivos.Permitir = func(req *http.Request) bool { return p.emparejado(req.TLS) }
		mux.Handle(localsend.Ruta, r.Archivos)
	}
	mux.HandleFunc("/pareja", func(w http.ResponseWriter, req *http.Request) {
		ws, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		ws.SetReadLimit(MaxSobre)
		c := &Conexion{ws: ws}
		defer c.Cerrar()
		ctxL, cancel := context.WithTimeout(ctx, 30*time.Second)
		s, err := c.Recibir(ctxL)
		cancel()
		if err != nil {
			return
		}
		rp := p.pedido(ctx, req.TLS, s.GetPedirPareja(), r.Hola)
		ctxE, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = c.Enviar(ctxE, &lazulv1.Sobre{Version: uint32(lazulv1.Version_VERSION_ACTUAL), Id: "r-" + s.GetId(), EnRespuestaA: s.GetId(),
			EnviadoMs: time.Now().UnixMilli(), Contenido: &lazulv1.Sobre_RespuestaPareja{RespuestaPareja: rp}})
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.Serve(tls.NewListener(ln, configServidor(p.Identidad))); err != http.ErrServerClosed {
		return err
	}
	return nil
}
