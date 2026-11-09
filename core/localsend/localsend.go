// Package localsend implementa la parte del protocolo LocalSend v2 que usa
// Lazul para mover archivos (capturas, documentos) de un equipo al otro:
// info, prepare-upload, upload y cancel. Los Sobres solo llevan los
// metadatos (Archivo con sha256 y transferencia_id); los bytes van por aquí.
//
// Protocolo: https://github.com/localsend/protocol (v2). No hace el
// descubrimiento por multicast de LocalSend: Lazul ya sabe la dirección y la
// huella del otro equipo por mDNS y el emparejamiento.
//
// Seguridad: en Lazul el Servidor se monta dentro de core.Servir, en el
// mismo puerto TLS 1.3 que las Órdenes, y solo atiende a equipos
// emparejados (certificado de cliente con huella fijada). El Cliente habla
// con un *http.Client que fija la huella del receptor (core.ClienteHTTP).
// Por eso la app oficial de LocalSend, que no manda certificado de cliente,
// no puede subirle archivos a un receptor de Lazul; el Cliente sí sigue el
// protocolo para hablar con ella (sin probar todavía contra la app real).
package localsend

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Ruta base de la API v2.
const Ruta = "/api/localsend/v2/"

// Límites del Servidor.
const (
	MaxArchivo  = 512 << 20 // bytes por archivo
	MaxArchivos = 64        // por sesión
	MaxSesiones = 8         // abiertas a la vez
	DuraSesion  = 10 * time.Minute
)

// Dispositivo es el "info" de LocalSend.
type Dispositivo struct {
	Alias       string `json:"alias"`
	Version     string `json:"version"`
	DeviceModel string `json:"deviceModel,omitempty"`
	DeviceType  string `json:"deviceType,omitempty"`
	Fingerprint string `json:"fingerprint"`
	Port        int    `json:"port,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Download    bool   `json:"download"`
}

// Meta describe un archivo en prepare-upload.
type Meta struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	FileType string `json:"fileType"`
	SHA256   string `json:"sha256,omitempty"`
}

type pedidoPreparar struct {
	Info  Dispositivo     `json:"info"`
	Files map[string]Meta `json:"files"`
}

type respuestaPreparar struct {
	SessionID string            `json:"sessionId"`
	Files     map[string]string `json:"files"`
}

// Recibido es un archivo que ya llegó entero y con su sha256 comprobado.
type Recibido struct {
	Sesion string
	Nombre string // el que mandó el otro, ya limpio
	Ruta   string // dónde quedó en la bandeja
	Bytes  int64
	SHA256 [32]byte
}

type archivoSesion struct {
	meta     Meta
	token    string
	subiendo bool
	recibido *Recibido
}

type sesion struct {
	id       string
	ip       string
	creada   time.Time
	archivos map[string]*archivoSesion
}

// Servidor recibe archivos en Bandeja. Permitir decide quién puede subir
// (en Lazul: solo equipos emparejados); nil no deja subir a nadie.
type Servidor struct {
	Bandeja  string
	Info     Dispositivo
	Permitir func(*http.Request) bool
	// Reloj se puede cambiar en pruebas.
	Reloj func() time.Time

	mu       sync.Mutex
	sesiones map[string]*sesion
}

func (s *Servidor) ahora() time.Time {
	if s.Reloj != nil {
		return s.Reloj()
	}
	return time.Now()
}

// ServeHTTP atiende las rutas bajo Ruta.
func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimPrefix(r.URL.Path, Ruta) {
	case "info":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Info)
		return
	}
	if s.Permitir == nil || !s.Permitir(r) {
		http.Error(w, "no permitido", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "solo POST", http.StatusMethodNotAllowed)
		return
	}
	switch strings.TrimPrefix(r.URL.Path, Ruta) {
	case "prepare-upload":
		s.preparar(w, r)
	case "upload":
		s.subir(w, r)
	case "cancel":
		s.mu.Lock()
		delete(s.sesiones, r.URL.Query().Get("sessionId"))
		s.mu.Unlock()
	default:
		http.NotFound(w, r)
	}
}

func ipDe(r *http.Request) string {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func (s *Servidor) preparar(w http.ResponseWriter, r *http.Request) {
	var p pedidoPreparar
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&p); err != nil {
		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
		return
	}
	if len(p.Files) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(p.Files) > MaxArchivos {
		http.Error(w, "demasiados archivos", http.StatusBadRequest)
		return
	}
	ses := &sesion{id: aleatorio(), ip: ipDe(r), creada: s.ahora(), archivos: map[string]*archivoSesion{}}
	resp := respuestaPreparar{SessionID: ses.id, Files: map[string]string{}}
	for id, m := range p.Files {
		if m.Size < 0 || m.Size > MaxArchivo || nombreLimpio(m.FileName) == "" {
			http.Error(w, "archivo inválido", http.StatusBadRequest)
			return
		}
		if m.SHA256 != "" {
			if b, err := hex.DecodeString(m.SHA256); err != nil || len(b) != sha256.Size {
				http.Error(w, "sha256 inválido", http.StatusBadRequest)
				return
			}
		}
		m.ID = id
		a := &archivoSesion{meta: m, token: aleatorio()}
		ses.archivos[id] = a
		resp.Files[id] = a.token
	}
	s.mu.Lock()
	s.limpiar()
	if len(s.sesiones) >= MaxSesiones {
		s.mu.Unlock()
		http.Error(w, "ocupado", http.StatusConflict)
		return
	}
	if s.sesiones == nil {
		s.sesiones = map[string]*sesion{}
	}
	s.sesiones[ses.id] = ses
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// limpiar quita las sesiones vencidas (con s.mu tomado).
func (s *Servidor) limpiar() {
	for id, ses := range s.sesiones {
		if s.ahora().Sub(ses.creada) > DuraSesion {
			delete(s.sesiones, id)
		}
	}
}

func (s *Servidor) subir(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	s.limpiar()
	ses := s.sesiones[q.Get("sessionId")]
	var a *archivoSesion
	if ses != nil {
		a = ses.archivos[q.Get("fileId")]
	}
	ok := a != nil && a.recibido == nil && !a.subiendo && a.token == q.Get("token") && ses.ip == ipDe(r)
	if ok {
		a.subiendo = true
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "token, sesión o IP inválidos", http.StatusForbidden)
		return
	}
	rec, err := s.guardar(ses.id, a.meta, r.Body)
	s.mu.Lock()
	a.subiendo, a.recibido = false, rec
	s.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// guardar escribe el archivo en la bandeja: primero a un temporal, comprueba
// largo y sha256, y lo renombra a un nombre que no pise otro.
func (s *Servidor) guardar(sesion string, m Meta, cuerpo io.Reader) (*Recibido, error) {
	if err := os.MkdirAll(s.Bandeja, 0o700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(s.Bandeja, ".llegando-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(cuerpo, m.Size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	if n != m.Size {
		return nil, fmt.Errorf("llegaron %d bytes de %d", n, m.Size)
	}
	var suma [32]byte
	copy(suma[:], h.Sum(nil))
	if m.SHA256 != "" && !strings.EqualFold(m.SHA256, hex.EncodeToString(suma[:])) {
		return nil, errors.New("el sha256 no coincide")
	}
	nombre := nombreLimpio(m.FileName)
	ruta, err := libre(s.Bandeja, nombre)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(f.Name(), ruta); err != nil {
		return nil, err
	}
	return &Recibido{Sesion: sesion, Nombre: nombre, Ruta: ruta, Bytes: n, SHA256: suma}, nil
}

// ErrNoRecibido: no llegó (todavía) un archivo con esa sesión y ese sha256.
var ErrNoRecibido = errors.New("localsend: el archivo no llegó")

// Buscar devuelve el archivo de la sesión con ese sha256, si ya llegó entero,
// y lo da por entregado: otra búsqueda igual ya no lo encuentra, y la sesión
// se cierra cuando no le quedan archivos.
func (s *Servidor) Buscar(sesionID string, suma []byte) (Recibido, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ses := s.sesiones[sesionID]; ses != nil {
		for id, a := range ses.archivos {
			if a.recibido != nil && bytes.Equal(a.recibido.SHA256[:], suma) {
				delete(ses.archivos, id)
				if len(ses.archivos) == 0 {
					delete(s.sesiones, sesionID)
				}
				return *a.recibido, nil
			}
		}
	}
	return Recibido{}, ErrNoRecibido
}

// nombreLimpio deja solo el último tramo del nombre, sin caracteres de
// control ni separadores, y con un largo razonable. "" si no queda nada.
func nombreLimpio(n string) string {
	n = n[strings.LastIndexAny(n, `/\`)+1:]
	n = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, n)
	n = strings.Trim(n, ". ")
	if r := []rune(n); len(r) > 120 {
		n = string(r[len(r)-120:])
	}
	return n
}

func libre(dir, nombre string) (string, error) {
	ext := filepath.Ext(nombre)
	base := strings.TrimSuffix(nombre, ext)
	for i := 0; i < 1000; i++ {
		n := nombre
		if i > 0 {
			n = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		ruta := filepath.Join(dir, n)
		if _, err := os.Lstat(ruta); errors.Is(err, os.ErrNotExist) {
			return ruta, nil
		}
	}
	return "", errors.New("demasiados archivos con el mismo nombre")
}

func aleatorio() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Archivo es lo que manda el Cliente.
type Archivo struct {
	Nombre string
	Mime   string
	Datos  []byte
}

// SHA256 de los datos.
func (a Archivo) SHA256() [32]byte { return sha256.Sum256(a.Datos) }

// Enviar sube a por LocalSend al equipo en base ("https://host:puerto") y
// devuelve el id de la sesión, que va en Archivo.transferencia_id.
func Enviar(ctx context.Context, c *http.Client, base string, yo Dispositivo, a Archivo) (string, error) {
	suma := a.SHA256()
	p := pedidoPreparar{Info: yo, Files: map[string]Meta{"1": {
		ID: "1", FileName: a.Nombre, Size: int64(len(a.Datos)), FileType: a.Mime, SHA256: hex.EncodeToString(suma[:]),
	}}}
	cuerpo, _ := json.Marshal(p)
	resp, err := pedir(ctx, c, base+Ruta+"prepare-upload", "application/json", cuerpo)
	if err != nil {
		return "", err
	}
	var rp respuestaPreparar
	if err := json.Unmarshal(resp, &rp); err != nil || rp.SessionID == "" || rp.Files["1"] == "" {
		return "", fmt.Errorf("prepare-upload: respuesta inesperada %q", recortar(resp))
	}
	q := url.Values{"sessionId": {rp.SessionID}, "fileId": {"1"}, "token": {rp.Files["1"]}}
	if _, err := pedir(ctx, c, base+Ruta+"upload?"+q.Encode(), "application/octet-stream", a.Datos); err != nil {
		return "", err
	}
	return rp.SessionID, nil
}

func pedir(ctx context.Context, c *http.Client, u, tipo string, cuerpo []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(cuerpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", tipo)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s %s", strings.TrimPrefix(u[strings.Index(u, Ruta):], Ruta), resp.Status, recortar(b))
	}
	return b, nil
}

func recortar(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}
