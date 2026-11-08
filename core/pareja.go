package core

// Emparejamiento (cotejo-1od1ta).
//
// Cada equipo tiene una Identidad: certificado autofirmado con clave ECDSA
// P-256. Todo va por TLS 1.3 con certificado en los dos lados (crypto/tls);
// la confianza no viene de una CA sino de la Huella que cada lado fija al
// emparejar.
//
//  1. El receptor abre el emparejamiento y muestra un código de 6 dígitos
//     (DuracionCodigo, IntentosCodigo, un solo uso).
//  2. El controlador se conecta a /pareja y manda PedirPareja con
//     prueba = HMAC-SHA256(código, rol ‖ exportador TLS ‖ huellaC ‖ huellaR).
//     El código no viaja; la prueba solo vale en esa sesión TLS y para ese
//     par de huellas, así que no se puede reenviar ni reflejar.
//  3. El receptor comprueba la prueba y le pregunta a la persona (Permiso),
//     mostrándole el nombre y la huella corta del controlador. Si dice que
//     sí, fija la huella y contesta con su propia prueba (rol receptor), que
//     demuestra que conocía el código; el controlador entonces fija la suya.
//  4. Desde ahí, Conectar exige la huella fijada del receptor y el receptor
//     solo atiende /lazul a huellas fijadas: un equipo no emparejado o con
//     otra clave se rechaza.
//
// Límite conocido: sin un PAKE (la librería estándar no trae ninguno), quien
// se ponga en medio durante los 2 minutos del código puede adivinarlo fuera
// de línea a partir de la prueba del controlador. Por eso la pregunta del
// receptor muestra la huella corta del controlador, que este también
// muestra: si no coinciden, la persona dice que no.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

var (
	ErrNoEmparejado   = errors.New("el receptor no tiene emparejado este equipo")
	ErrHuellaCambiada = errors.New("la huella TLS del otro equipo no es la fijada")
	ErrCodigoErroneo  = errors.New("código erróneo")
	ErrCaducado       = errors.New("el código caducó o se acabaron los intentos; abre otro en el receptor")
	ErrRechazado      = errors.New("la persona en el receptor no lo permitió")
	ErrCerrado        = errors.New("el receptor no está emparejando ahora")
	ErrPruebaReceptor = errors.New("el receptor no demostró conocer el código")
)

// FallaPareja acompaña a ErrCodigoErroneo con los intentos que quedan.
type FallaPareja struct {
	Err       error
	Restantes int
}

func (f *FallaPareja) Error() string {
	return fmt.Sprintf("%v (quedan %d intentos)", f.Err, f.Restantes)
}
func (f *FallaPareja) Unwrap() error { return f.Err }

// EsperaPermiso: cuánto espera el receptor a que la persona conteste.
const EsperaPermiso = time.Minute

const (
	rolControlador = "lazul-pareja-v1 controlador"
	rolReceptor    = "lazul-pareja-v1 receptor"
	etiquetaTLS    = "EXPORTER-lazul-pareja-v1"
)

// prueba demuestra que se conoce el código sin mandarlo, atada a la sesión
// TLS (RFC 8446 §7.5, exportador) y a las dos huellas.
func prueba(codigo, rol string, cs *tls.ConnectionState, controlador, receptor Huella) ([]byte, error) {
	ekm, err := cs.ExportKeyingMaterial(etiquetaTLS, nil, 32)
	if err != nil {
		return nil, err
	}
	m := hmac.New(sha256.New, []byte(codigo))
	m.Write([]byte(rol))
	m.Write(ekm)
	m.Write(controlador[:])
	m.Write(receptor[:])
	return m.Sum(nil), nil
}

func configServidor(id *Identidad) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{id.Cert},
		// Se exige certificado pero no se valida contra una CA: lo que vale
		// es su huella, que se mira en cada petición (Servir).
		ClientAuth: tls.RequireAnyClientCert,
	}
}

// configCliente: verificar decide con la huella del receptor.
func configCliente(id *Identidad, verificar func(Huella) error) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{id.Cert},
		ServerName:   "lazul",
		// Sin CA: la verificación la hace VerifyConnection con la huella.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("el receptor no mandó certificado")
			}
			return verificar(HuellaDe(cs.PeerCertificates[0]))
		},
	}
}

func huellaDelOtro(cs *tls.ConnectionState) (Huella, bool) {
	if cs == nil || len(cs.PeerCertificates) == 0 {
		return Huella{}, false
	}
	return HuellaDe(cs.PeerCertificates[0]), true
}

// Solicitud es lo que ve la persona en el receptor antes de permitir.
type Solicitud struct {
	Nombre   string // el que dice el controlador (no verificado)
	EquipoID string
	Huella   Huella // la del certificado del controlador; mostrar Corta()
}

// Puerta decide quién entra a un receptor.
type Puerta struct {
	Identidad *Identidad
	Pares     *Pares
	// Permiso pregunta a la persona. nil rechaza siempre.
	Permiso func(context.Context, Solicitud) bool
	// Reloj se puede cambiar en pruebas.
	Reloj func() time.Time

	mu     sync.Mutex
	codigo *codigo
}

func (p *Puerta) ahora() time.Time {
	if p.Reloj != nil {
		return p.Reloj()
	}
	return time.Now()
}

// AbrirEmparejamiento genera un código nuevo (anula el anterior) y lo
// devuelve para mostrarlo en pantalla.
func (p *Puerta) AbrirEmparejamiento() (string, error) {
	c, err := nuevoCodigo(p.ahora())
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	p.codigo = c
	p.mu.Unlock()
	return c.digitos, nil
}

func (p *Puerta) CerrarEmparejamiento() {
	p.mu.Lock()
	p.codigo = nil
	p.mu.Unlock()
}

func (p *Puerta) emparejado(cs *tls.ConnectionState) bool {
	h, ok := huellaDelOtro(cs)
	if !ok {
		return false
	}
	_, ok = p.Pares.Buscar(h)
	return ok
}

// pedido atiende un PedirPareja que llegó por la sesión cs.
func (p *Puerta) pedido(ctx context.Context, cs *tls.ConnectionState, pp *lazulv1.PedirPareja, yo *lazulv1.Hola) *lazulv1.RespuestaPareja {
	resp := func(r lazulv1.RespuestaPareja_Resultado) *lazulv1.RespuestaPareja {
		return &lazulv1.RespuestaPareja{Resultado: r}
	}
	hc, ok := huellaDelOtro(cs)
	if !ok || pp == nil {
		return resp(lazulv1.RespuestaPareja_RESULTADO_DESCONOCIDO)
	}
	hr := p.Identidad.Huella

	p.mu.Lock()
	if p.codigo == nil {
		p.mu.Unlock()
		return resp(lazulv1.RespuestaPareja_RESULTADO_CERRADO)
	}
	digitos := p.codigo.digitos
	res, quedan := p.codigo.probar(p.ahora(), func(d string) bool {
		esperada, err := prueba(d, rolControlador, cs, hc, hr)
		return err == nil && hmac.Equal(esperada, pp.GetPrueba())
	})
	if res == resBien {
		p.codigo = nil // un solo uso
	}
	p.mu.Unlock()

	switch res {
	case resErroneo:
		r := resp(lazulv1.RespuestaPareja_RESULTADO_CODIGO_ERRONEO)
		r.IntentosRestantes = uint32(quedan)
		return r
	case resCaducado:
		return resp(lazulv1.RespuestaPareja_RESULTADO_CADUCADO)
	}

	sol := Solicitud{Nombre: recortar(pp.GetHola().GetNombre()), EquipoID: recortar(pp.GetHola().GetEquipoId()), Huella: hc}
	ctxP, cancel := context.WithTimeout(ctx, EsperaPermiso)
	defer cancel()
	if p.Permiso == nil || !p.Permiso(ctxP, sol) {
		return resp(lazulv1.RespuestaPareja_RESULTADO_RECHAZADO)
	}
	suya, err := prueba(digitos, rolReceptor, cs, hc, hr)
	if err != nil {
		return resp(lazulv1.RespuestaPareja_RESULTADO_DESCONOCIDO)
	}
	if err := p.Pares.Fijar(Par{Huella: hc, EquipoID: sol.EquipoID, Nombre: sol.Nombre}); err != nil {
		return resp(lazulv1.RespuestaPareja_RESULTADO_DESCONOCIDO)
	}
	return &lazulv1.RespuestaPareja{Resultado: lazulv1.RespuestaPareja_RESULTADO_ACEPTADO, Prueba: suya, Hola: yo}
}

// Emparejar pide emparejarse al receptor en dir (host:puerto) con el código
// que muestra su pantalla. Si anunciada no es cero (la huella que vino por
// mDNS), el certificado del receptor tiene que coincidir. Devuelve el Par que
// el controlador debe fijar en sus Pares.
func Emparejar(ctx context.Context, dir string, yo *Identidad, hola *lazulv1.Hola, codigoTexto string, anunciada Huella) (Par, error) {
	cod, err := normalizarCodigo(codigoTexto)
	if err != nil {
		return Par{}, err
	}
	var hr Huella
	c, cs, err := dialTLS(ctx, "wss://"+dir+"/pareja", configCliente(yo, func(h Huella) error {
		if !anunciada.cero() && h != anunciada {
			return ErrHuellaCambiada
		}
		hr = h
		return nil
	}))
	if err != nil {
		return Par{}, err
	}
	defer c.Cerrar()

	pr, err := prueba(cod, rolControlador, cs, yo.Huella, hr)
	if err != nil {
		return Par{}, err
	}
	s := &lazulv1.Sobre{Version: uint32(lazulv1.Version_VERSION_ACTUAL), Id: "pareja", EnviadoMs: time.Now().UnixMilli(),
		Contenido: &lazulv1.Sobre_PedirPareja{PedirPareja: &lazulv1.PedirPareja{Hola: hola, Prueba: pr}}}
	if err := c.Enviar(ctx, s); err != nil {
		return Par{}, err
	}
	resp, err := c.Recibir(ctx)
	if err != nil {
		return Par{}, err
	}
	rp := resp.GetRespuestaPareja()
	switch rp.GetResultado() {
	case lazulv1.RespuestaPareja_RESULTADO_ACEPTADO:
	case lazulv1.RespuestaPareja_RESULTADO_CODIGO_ERRONEO:
		return Par{}, &FallaPareja{Err: ErrCodigoErroneo, Restantes: int(rp.GetIntentosRestantes())}
	case lazulv1.RespuestaPareja_RESULTADO_CADUCADO:
		return Par{}, ErrCaducado
	case lazulv1.RespuestaPareja_RESULTADO_RECHAZADO:
		return Par{}, ErrRechazado
	case lazulv1.RespuestaPareja_RESULTADO_CERRADO:
		return Par{}, ErrCerrado
	default:
		return Par{}, fmt.Errorf("respuesta de emparejamiento inesperada: %v", rp.GetResultado())
	}
	esperada, err := prueba(cod, rolReceptor, cs, yo.Huella, hr)
	if err != nil {
		return Par{}, err
	}
	if !hmac.Equal(esperada, rp.GetPrueba()) {
		return Par{}, ErrPruebaReceptor
	}
	return Par{Huella: hr, EquipoID: rp.GetHola().GetEquipoId(), Nombre: rp.GetHola().GetNombre(), Desde: time.Now()}, nil
}

// recortar: lo que manda el otro se muestra en pantalla; sin caracteres de
// control y con un largo razonable.
func recortar(s string) string {
	r := []rune(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s))
	if len(r) > 64 {
		r = r[:64]
	}
	return string(r)
}
