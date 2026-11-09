package main

// En Linux la cámara no la abre la webview (WebKitGTK cuelga con
// video.srcObject, dec-7tgj) sino ffmpeg en un proceso aparte (V4L2 directo).
// Go recibe los JPEG por una tubería y la página los pide por el
// AssetServer; MediaPipe sigue en la webview. Los cuadros solo viven en
// memoria: nunca se guardan.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Rafael088/lazul/platform"
)

type captura struct {
	dispositivo string

	mu    sync.Mutex
	cond  *sync.Cond
	jpeg  []byte
	seq   int
	cmd   *exec.Cmd
	error string
	gen   int // cambia en cada encender: los cuadros viejos no valen
}

func nuevaCaptura(dispositivo string) *captura {
	c := &captura{dispositivo: dispositivo}
	c.cond = sync.NewCond(&c.mu)
	return c
}

func (c *captura) encender() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil {
		return nil
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		c.error = "falta ffmpeg (instálalo con el gestor de paquetes)"
		return errors.New(c.error)
	}
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "v4l2", "-input_format", "mjpeg", "-video_size", "640x480", "-framerate", "30",
		"-i", c.dispositivo, "-c:v", "copy", "-f", "mjpeg", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		c.error = err.Error()
		return err
	}
	c.cmd, c.error, c.jpeg = cmd, "", nil
	c.gen++
	go c.leer(out, cmd, &stderr, c.gen)
	return nil
}

func (c *captura) apagar() {
	c.mu.Lock()
	cmd := c.cmd
	c.cmd, c.jpeg = nil, nil
	c.error = "cámara apagada"
	c.cond.Broadcast()
	c.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// leer parte el flujo MJPEG por los marcadores de inicio (FFD8) y fin (FFD9) de cada JPEG.
func (c *captura) leer(r io.Reader, cmd *exec.Cmd, stderr *bytes.Buffer, gen int) {
	br := bufio.NewReaderSize(r, 1<<20)
	var buf bytes.Buffer
	var prev byte
	dentro := false
	for {
		b, err := br.ReadByte()
		if err != nil {
			c.mu.Lock()
			if c.gen == gen && c.cmd == cmd {
				c.error = "la cámara se cerró (¿la usa otra app?): " + string(bytes.TrimSpace(stderr.Bytes()))
				c.cmd = nil
			}
			c.cond.Broadcast()
			c.mu.Unlock()
			return
		}
		if !dentro {
			if prev == 0xFF && b == 0xD8 {
				dentro = true
				buf.Reset()
				buf.Write([]byte{0xFF, 0xD8})
			}
		} else {
			buf.WriteByte(b)
			if prev == 0xFF && b == 0xD9 {
				dentro = false
				c.mu.Lock()
				if c.gen == gen {
					c.jpeg = append(c.jpeg[:0], buf.Bytes()...)
					c.seq++
					c.cond.Broadcast()
				}
				c.mu.Unlock()
			}
		}
		prev = b
	}
}

// ServeHTTP: GET /camara/cuadro?despues=N espera al primer cuadro con seq > N.
func (c *captura) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/camara/cuadro" {
		http.NotFound(w, r)
		return
	}
	despues, _ := strconv.Atoi(r.URL.Query().Get("despues"))
	// Que una espera sin cuadros no quede colgada para siempre.
	listo := make(chan struct{})
	defer close(listo)
	go func() {
		select {
		case <-listo:
		case <-time.After(5 * time.Second):
			c.mu.Lock()
			c.cond.Broadcast()
			c.mu.Unlock()
		}
	}()
	ini := time.Now()
	c.mu.Lock()
	for c.seq <= despues && c.cmd != nil && time.Since(ini) < 5*time.Second {
		c.cond.Wait()
	}
	if c.seq <= despues || c.jpeg == nil {
		e := c.error
		if e == "" {
			e = "la cámara no da cuadros"
		}
		c.mu.Unlock()
		http.Error(w, e, http.StatusServiceUnavailable)
		return
	}
	jpeg := append([]byte(nil), c.jpeg...)
	seq := c.seq
	c.mu.Unlock()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Seq", strconv.Itoa(seq))
	_, _ = w.Write(jpeg)
}

func manejador(c *captura) http.Handler {
	if c == nil {
		return nil
	}
	return c
}

// anotador cumple LaunchApp sin abrir nada (con -abrir=false): deja una línea
// en la salida con el tipo, nunca la URL ni la ruta.
type anotador struct{}

func (anotador) Launch(ctx context.Context, ref platform.AppRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch {
	case ref.URL != "":
		decir("abriría una URL")
	case ref.Archivo != "":
		info, err := os.Stat(ref.Archivo)
		if err != nil {
			return fmt.Errorf("archivo: %w", platform.ErrNoEncontrada)
		}
		decir("abriría el archivo recibido (%s, %d bytes)", filepath.Ext(ref.Archivo), info.Size())
	case ref.ID != "":
		decir("abriría la app %s", ref.ID)
	default:
		return platform.ErrRefInvalida
	}
	return nil
}
