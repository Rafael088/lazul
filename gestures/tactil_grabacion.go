package gestures

import (
	"fmt"
	"strings"
)

// ResultadoTactil conserva también el estado esperado al alternar el modo.
type ResultadoTactil struct {
	Grabacion     string
	Esperado      AccionTactil
	Direccion     DireccionTactil
	Activo        bool
	ComprobarModo bool
	ReconocerL    bool // las tomas antiguas pueden contener varios intentos de la L
	Emitidos      []EventoTactil
}

func (r ResultadoTactil) Acierto() bool {
	if r.Esperado == TactilNinguno {
		return len(r.Emitidos) == 0
	}
	if r.ReconocerL {
		return len(r.Emitidos) > 0 && r.Emitidos[0].Accion == TactilModo && r.Emitidos[0].Activo
	}
	if len(r.Emitidos) != 1 || r.Emitidos[0].Accion != r.Esperado {
		return false
	}
	if r.Direccion != TactilSinDireccion && r.Emitidos[0].Direccion != r.Direccion {
		return false
	}
	return !r.ComprobarModo || r.Emitidos[0].Activo == r.Activo
}

// MedicionTactil resume positivos y falsos positivos como Medicion, pero sin
// mezclar las acciones táctiles con los cuatro gestos de transferencia.
type MedicionTactil struct {
	PorAccion       map[AccionTactil][2]int
	Positivas       int
	AciertosPos     int
	Negativas       int
	FalsosPositivos int
}

func (m MedicionTactil) Precision() float64 {
	if m.Positivas == 0 {
		return 0
	}
	return float64(m.AciertosPos) / float64(m.Positivas)
}

func accionTactilDeNombre(s string) (AccionTactil, error) {
	switch strings.ToUpper(s) {
	case "", "NINGUNO":
		return TactilNinguno, nil
	case "MODO":
		return TactilModo, nil
	case "SCROLL":
		return TactilScroll, nil
	case "ZOOM":
		return TactilZoom, nil
	case "BARRIDO":
		return TactilBarrido, nil
	default:
		return 0, fmt.Errorf("acción táctil desconocida %q", s)
	}
}

func direccionTactilDeNombre(s string) (DireccionTactil, error) {
	switch strings.ToUpper(s) {
	case "", "NINGUNA":
		return TactilSinDireccion, nil
	case "IZQUIERDA":
		return TactilIzquierda, nil
	case "DERECHA":
		return TactilDerecha, nil
	case "ARRIBA":
		return TactilArriba, nil
	case "ABAJO":
		return TactilAbajo, nil
	case "ABRIR":
		return TactilAbrir, nil
	case "CERRAR":
		return TactilCerrar, nil
	default:
		return 0, fmt.Errorf("dirección táctil desconocida %q", s)
	}
}

func validarCabeceraTactil(c Cabecera) error {
	if _, err := accionTactilDeNombre(c.Tactil); err != nil {
		return err
	}
	if _, err := direccionTactilDeNombre(c.DireccionTactil); err != nil {
		return err
	}
	for nombre, valor := range map[string]string{"inicio_tactil": c.InicioTactil, "estado_tactil": c.EstadoTactil} {
		if valor != "" && !strings.EqualFold(valor, "activo") && !strings.EqualFold(valor, "inactivo") {
			return fmt.Errorf("%s desconocido %q", nombre, valor)
		}
	}
	return nil
}

func esperadoTactil(g Grabacion) (ResultadoTactil, bool, error) {
	r := ResultadoTactil{Grabacion: g.Archivo}
	inicioActivo := strings.EqualFold(g.Cabecera.InicioTactil, "activo")
	if g.Cabecera.Tactil == "" {
		// Las tomas reales de ACTIVAR son también la prueba real de la L del
		// modo táctil. El resto queda como negativo con el modo apagado.
		if strings.EqualFold(g.Cabecera.Gesto, "ACTIVAR") {
			r.Esperado, r.Activo, r.ComprobarModo, r.ReconocerL = TactilModo, true, true, true
		}
		return r, inicioActivo, nil
	}
	var err error
	if r.Esperado, err = accionTactilDeNombre(g.Cabecera.Tactil); err != nil {
		return r, false, err
	}
	if r.Direccion, err = direccionTactilDeNombre(g.Cabecera.DireccionTactil); err != nil {
		return r, false, err
	}
	if r.Esperado == TactilModo {
		r.ComprobarModo = true
		r.Activo = strings.EqualFold(g.Cabecera.EstadoTactil, "activo")
	}
	return r, inicioActivo, nil
}

func reproducirTactil(g Grabacion, activo bool, aj AjustesTactiles) ResultadoTactil {
	r, _, err := esperadoTactil(g)
	if err != nil {
		return ResultadoTactil{Grabacion: g.Archivo}
	}
	e := NuevoEstadoTactil(activo, aj)
	for _, c := range g.Cuadros {
		var ev EventoTactil
		e, ev = ClasificarTactil(e, c)
		if ev.Accion != TactilNinguno {
			r.Emitidos = append(r.Emitidos, ev)
		}
	}
	return r
}

func ReproducirTactil(g Grabacion) ResultadoTactil {
	_, activo, _ := esperadoTactil(g)
	return reproducirTactil(g, activo, AjustesTactilesPorDefecto)
}

func ReproducirTactilDesde(g Grabacion, activo bool) ResultadoTactil {
	return reproducirTactil(g, activo, AjustesTactilesPorDefecto)
}

func MedirTactil(gs []Grabacion) (MedicionTactil, []ResultadoTactil) {
	m := MedicionTactil{PorAccion: map[AccionTactil][2]int{}}
	rs := make([]ResultadoTactil, 0, len(gs))
	for _, g := range gs {
		r := ReproducirTactil(g)
		rs = append(rs, r)
		if r.Esperado == TactilNinguno {
			m.Negativas++
			m.FalsosPositivos += len(r.Emitidos)
			continue
		}
		m.Positivas++
		v := m.PorAccion[r.Esperado]
		v[1]++
		if r.Acierto() {
			v[0]++
			m.AciertosPos++
		}
		m.PorAccion[r.Esperado] = v
	}
	return m, rs
}
