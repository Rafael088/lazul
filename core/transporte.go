package core

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// Conexion lleva Sobres en mensajes binarios de WebSocket.
//
// Todavía sin TLS ni emparejamiento: solo para ./dev sim y pruebas en
// 127.0.0.1. cotejo-1od1ta le pone TLS con huella fijada antes de abrirlo a
// la red.
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

// Conectar abre una conexión con un receptor en url (ws://host:puerto/lazul).
func Conectar(ctx context.Context, url string) (*Conexion, error) {
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(MaxSobre)
	return &Conexion{ws: ws}, nil
}

// Servir escucha en ln y atiende cada conexión con r hasta que ctx termine.
func Servir(ctx context.Context, ln net.Listener, r *Receptor) error {
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
			if resp := r.Atender(ctx, s); resp != nil {
				if err := c.Enviar(ctx, resp); err != nil {
					return
				}
			}
		}
	})}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}
