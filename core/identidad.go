package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Huella identifica a un equipo: SHA-256 de la clave pública de su
// certificado (SubjectPublicKeyInfo). Es lo que se fija al emparejar.
type Huella [sha256.Size]byte

func HuellaDe(c *x509.Certificate) Huella { return sha256.Sum256(c.RawSubjectPublicKeyInfo) }

func (h Huella) String() string { return hex.EncodeToString(h[:]) }

// Corta es lo que ve la persona para comparar en las dos pantallas:
// los primeros 8 bytes en 4 grupos ("1A2B-3C4D-5E6F-7A8B").
func (h Huella) Corta() string {
	s := strings.ToUpper(hex.EncodeToString(h[:8]))
	return s[0:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:16]
}

func (h Huella) cero() bool { return h == Huella{} }

func ParsearHuella(s string) (Huella, error) {
	var h Huella
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(h) {
		return h, fmt.Errorf("huella inválida %q", s)
	}
	copy(h[:], b)
	return h, nil
}

// Identidad: el certificado propio de este equipo (autofirmado; la confianza
// no viene de una CA sino de la huella fijada al emparejar).
type Identidad struct {
	Cert   tls.Certificate
	Huella Huella
}

// NuevaIdentidad crea una clave ECDSA P-256 y su certificado autofirmado.
func NuevaIdentidad(nombre string) (*Identidad, error) {
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serie, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, err
	}
	plantilla := &x509.Certificate{
		SerialNumber: serie,
		Subject:      pkix.Name{CommonName: "lazul"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(20, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"lazul"},
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		return nil, err
	}
	return identidadDe(der, clave)
}

func identidadDe(der []byte, clave any) (*Identidad, error) {
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Identidad{Cert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: clave, Leaf: c}, Huella: HuellaDe(c)}, nil
}

// CargarOCrearIdentidad lee la identidad de ruta (PEM) o la crea y la guarda
// con permisos 0600. La huella no cambia mientras exista el archivo.
func CargarOCrearIdentidad(ruta, nombre string) (*Identidad, error) {
	b, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		id, err := NuevaIdentidad(nombre)
		if err != nil {
			return nil, err
		}
		return id, id.Guardar(ruta)
	}
	if err != nil {
		return nil, err
	}
	var der, claveDER []byte
	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)
		if blk == nil {
			break
		}
		switch blk.Type {
		case "CERTIFICATE":
			der = blk.Bytes
		case "PRIVATE KEY":
			claveDER = blk.Bytes
		}
	}
	if der == nil || claveDER == nil {
		return nil, fmt.Errorf("%s: falta el certificado o la clave", ruta)
	}
	clave, err := x509.ParsePKCS8PrivateKey(claveDER)
	if err != nil {
		return nil, err
	}
	id, err := identidadDe(der, clave)
	if err != nil {
		return nil, err
	}
	// Que la clave sea la del certificado: si no, TLS fallaría más tarde y peor.
	if _, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: claveDER})); err != nil {
		return nil, fmt.Errorf("%s: %w", ruta, err)
	}
	return id, nil
}

// Guardar escribe certificado y clave en un PEM con permisos 0600.
func (id *Identidad) Guardar(ruta string) error {
	claveDER, err := x509.MarshalPKCS8PrivateKey(id.Cert.PrivateKey)
	if err != nil {
		return err
	}
	b := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: id.Cert.Certificate[0]})
	b = append(b, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: claveDER})...)
	return escribirPrivado(ruta, b)
}

// escribirPrivado escribe de forma atómica (temporal y renombrar) en 0600.
func escribirPrivado(ruta string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	// CreateTemp ya crea el archivo en 0600.
	f, err := os.CreateTemp(filepath.Dir(ruta), ".lazul-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), ruta)
}
