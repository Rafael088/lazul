package main

// Plan B (modo «proceso»): la cámara no la abre la webview sino un proceso
// aparte (ffmpeg, V4L2 directo). Go recibe los JPEG por una tubería y la
// página los pide por el AssetServer; MediaPipe sigue en la webview.

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type captura struct {
	mu    sync.Mutex
	cond  *sync.Cond
	jpeg  []byte
	seq   int
	t     float64 // ms Unix en que Go terminó de recibir el cuadro
	cmd   *exec.Cmd
	error string
}

func nuevaCaptura(dispositivo string) *captura {
	c := &captura{}
	c.cond = sync.NewCond(&c.mu)
	c.cmd = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "v4l2", "-input_format", "mjpeg", "-video_size", "640x480", "-framerate", "30",
		"-i", dispositivo, "-c:v", "copy", "-f", "mjpeg", "-")
	out, err := c.cmd.StdoutPipe()
	if err == nil {
		err = c.cmd.Start()
	}
	if err != nil {
		c.error = err.Error()
		return c
	}
	go c.leer(out)
	return c
}

// leer parte el flujo MJPEG por los marcadores de inicio (FFD8) y fin (FFD9) de cada JPEG.
func (c *captura) leer(r io.Reader) {
	br := bufio.NewReaderSize(r, 1<<20)
	var buf bytes.Buffer
	var prev byte
	dentro := false
	for {
		b, err := br.ReadByte()
		if err != nil {
			c.mu.Lock()
			c.error = "ffmpeg terminó: " + err.Error()
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
				t := float64(time.Now().UnixNano()) / 1e6
				c.mu.Lock()
				c.jpeg = append(c.jpeg[:0], buf.Bytes()...)
				c.seq++
				c.t = t
				c.cond.Broadcast()
				c.mu.Unlock()
			}
		}
		prev = b
	}
}

func (c *captura) cerrar() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

// ServeHTTP: GET /camara/cuadro?despues=N espera al primer cuadro con seq > N.
func (c *captura) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/camara/cuadro" {
		http.NotFound(w, r)
		return
	}
	despues, _ := strconv.Atoi(r.URL.Query().Get("despues"))
	c.mu.Lock()
	for c.seq <= despues && c.error == "" {
		c.cond.Wait()
	}
	if c.error != "" && c.seq <= despues {
		e := c.error
		c.mu.Unlock()
		http.Error(w, e, http.StatusServiceUnavailable)
		return
	}
	jpeg := append([]byte(nil), c.jpeg...)
	seq, t := c.seq, c.t
	c.mu.Unlock()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Seq", strconv.Itoa(seq))
	w.Header().Set("X-T", fmt.Sprintf("%.3f", t))
	_, _ = w.Write(jpeg)
}
