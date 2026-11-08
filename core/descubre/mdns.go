package descubre

import (
	"context"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/hashicorp/mdns"
)

// MDNS anuncia y busca con multicast DNS en la red local (necesita red real;
// las pruebas usan Memoria).
type MDNS struct {
	// Espera por defecto de Buscar si ctx no tiene plazo.
	Espera time.Duration
}

var callado = log.New(io.Discard, "", 0)

func (MDNS) Anunciar(a Anuncio) (func(), error) {
	_, p, err := net.SplitHostPort(a.Dir)
	if err != nil {
		return nil, err
	}
	puerto, err := strconv.Atoi(p)
	if err != nil {
		return nil, err
	}
	// Nombre de host propio (no el del sistema) y las IPv4 de las interfaces.
	svc, err := mdns.NewMDNSService(a.EquipoID, Servicio, "", a.EquipoID+".lazul.local.", puerto, ipsLocales(), txt(a))
	if err != nil {
		return nil, err
	}
	srv, err := mdns.NewServer(&mdns.Config{Zone: svc, Logger: callado})
	if err != nil {
		return nil, err
	}
	return func() { _ = srv.Shutdown() }, nil
}

func (m MDNS) Buscar(ctx context.Context) ([]Anuncio, error) {
	espera := m.Espera
	if espera == 0 {
		espera = 2 * time.Second
	}
	if fin, ok := ctx.Deadline(); ok {
		espera = time.Until(fin)
	}
	ch := make(chan *mdns.ServiceEntry, 32)
	listo := make(chan struct{})
	vistos := map[string]Anuncio{}
	go func() {
		defer close(listo)
		for e := range ch {
			host := ""
			if e.AddrV4 != nil {
				host = e.AddrV4.String()
			}
			if a, err := deTxt(e.InfoFields, host, e.Port); err == nil {
				vistos[a.Huella+a.Dir] = a
			}
		}
	}()
	err := mdns.QueryContext(ctx, &mdns.QueryParam{Service: Servicio, Timeout: espera, Entries: ch, DisableIPv6: true, Logger: callado})
	close(ch)
	<-listo
	l := make([]Anuncio, 0, len(vistos))
	for _, a := range vistos {
		l = append(l, a)
	}
	return l, err
}

func ipsLocales() []net.IP {
	var ips []net.IP
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagMulticast == 0 {
			continue
		}
		dirs, _ := i.Addrs()
		for _, d := range dirs {
			if n, ok := d.(*net.IPNet); ok && n.IP.To4() != nil {
				ips = append(ips, n.IP)
			}
		}
	}
	if len(ips) == 0 {
		ips = []net.IP{net.IPv4(127, 0, 0, 1)}
	}
	return ips
}
