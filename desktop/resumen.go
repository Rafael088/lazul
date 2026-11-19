package desktop

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// VentanaResumen: la meta de dec-q1p7 es menos de 1 activación falsa en 30
// min de trabajo normal.
const VentanaResumen = 30 * time.Minute

// Resumen de las activaciones en los últimos 30 min de uso (con la cámara
// dando cuadros) y en la última prueba de trabajo normal.
//
// Cuenta como falsa: toda activación dentro de la prueba de trabajo normal;
// fuera de ella, la que la persona marcó «sin querer» y la que venció sin
// ningún gesto (salvo que la persona la marcara «a propósito»). Una que
// terminó en captura o gesto 4 cuenta como buscada, salvo marca «sin querer».
type Resumen struct {
	Desde             time.Time     // empieza la ventana (la hora en que se completan 30 min de uso hacia atrás)
	Uso               time.Duration // uso dentro de la ventana (hasta 30 min)
	UsoTotal          time.Duration // uso en todo el registro
	Activaciones      int
	PorResultado      map[string]int
	Falsas            int
	Buscadas          int
	Evitadas          int
	EvitadasPorMotivo map[string]int
	// EvitadasPanelPorMotivo: de las manos que quedaron fuera del panel, por
	// qué (fuera_borde, cerca, lejos, sin_palma o sin_panel).
	EvitadasPanelPorMotivo map[string]int
	MuestrasRendimiento    int
	FPSPromedio            float64
	CPUCorePromedio        float64
	CPUTotalPromedio       float64
	ManoMsPromedio         float64
	PoseMsPromedio         float64
	// La última prueba de trabajo normal (cero si no hubo).
	PruebaDesde        time.Time
	PruebaUso          time.Duration
	PruebaActivaciones int
	PruebaEvitadas     int
	PruebaAbierta      bool
}

type tramo struct{ desde, hasta time.Time }

// tramosDeUso: entre dos líneas seguidas de la misma sesión hay uso si el
// hueco no pasa de HuecoMax.
func tramosDeUso(ls []Linea) []tramo {
	var ts []tramo
	ult := map[string]time.Time{}
	for _, l := range ls {
		if l.Sesion == "" || l.Tipo == TipoRecibido {
			continue
		}
		if p, ok := ult[l.Sesion]; ok && l.T.After(p) && l.T.Sub(p) <= HuecoMax {
			ts = append(ts, tramo{p, l.T})
		}
		if l.Tipo == TipoFin {
			delete(ult, l.Sesion)
		} else {
			ult[l.Sesion] = l.T
		}
	}
	return ts
}

// usoEntre: cuánto uso hay entre a y b (b cero = sin fin).
func usoEntre(ts []tramo, a, b time.Time) time.Duration {
	var d time.Duration
	for _, t := range ts {
		ini, fin := t.desde, t.hasta
		if ini.Before(a) {
			ini = a
		}
		if !b.IsZero() && fin.After(b) {
			fin = b
		}
		if fin.After(ini) {
			d += fin.Sub(ini)
		}
	}
	return d
}

type claveAct struct {
	sesion string
	n      int
}

// Resumir hace el Resumen de las líneas de un registro.
func Resumir(ls []Linea) Resumen {
	ls = slices.Clone(ls)
	slices.SortStableFunc(ls, func(a, b Linea) int { return a.T.Compare(b.T) })
	ts := tramosDeUso(ls)
	r := Resumen{PorResultado: map[string]int{}, EvitadasPorMotivo: map[string]int{}, EvitadasPanelPorMotivo: map[string]int{}}
	for _, t := range ts {
		r.UsoTotal += t.hasta.Sub(t.desde)
	}
	// La ventana: hacia atrás desde el último tramo hasta juntar 30 min.
	var falta = VentanaResumen
	for i := len(ts) - 1; i >= 0; i-- {
		d := ts[i].hasta.Sub(ts[i].desde)
		if d >= falta {
			r.Desde = ts[i].hasta.Add(-falta)
			falta = 0
			break
		}
		falta -= d
		r.Desde = ts[i].desde
	}
	r.Uso = VentanaResumen - falta

	marcas := map[claveAct]bool{}
	for _, l := range ls {
		if l.Tipo == TipoMarca && l.Falsa != nil {
			marcas[claveAct{l.Sesion, l.Activacion}] = *l.Falsa
		}
		if l.Tipo == TipoPrueba {
			if l.Resultado == "empieza" {
				r.PruebaDesde, r.PruebaAbierta, r.PruebaActivaciones = l.T, true, 0
			} else if r.PruebaAbierta {
				r.PruebaAbierta = false
				r.PruebaUso = usoEntre(ts, r.PruebaDesde, l.T)
			}
		}
	}
	if r.PruebaAbierta {
		r.PruebaUso = usoEntre(ts, r.PruebaDesde, time.Time{})
	}
	for _, l := range ls {
		if l.Tipo == TipoRendimiento && len(ts) > 0 && !l.T.Before(r.Desde) {
			r.MuestrasRendimiento++
			r.FPSPromedio += l.FPS
			r.CPUCorePromedio += l.CPUCore
			r.CPUTotalPromedio += l.CPUTotal
			r.ManoMsPromedio += l.ManoMs
			r.PoseMsPromedio += l.PoseMs
			continue
		}
		if l.Tipo == TipoEvitada {
			if !r.PruebaDesde.IsZero() && l.Prueba && !l.T.Before(r.PruebaDesde) {
				r.PruebaEvitadas++
			}
			if len(ts) > 0 && !l.T.Before(r.Desde) {
				r.Evitadas++
				r.EvitadasPorMotivo[l.Resultado]++
				if l.Resultado == "fuera_panel" && l.Motivo != "" {
					r.EvitadasPanelPorMotivo[l.Motivo]++
				}
			}
			continue
		}
		if l.Tipo != TipoActivacion {
			continue
		}
		if !r.PruebaDesde.IsZero() && l.Prueba && !l.T.Before(r.PruebaDesde) {
			r.PruebaActivaciones++
		}
		if len(ts) == 0 || l.T.Before(r.Desde) {
			continue
		}
		r.Activaciones++
		r.PorResultado[l.Resultado]++
		marca, marcada := marcas[claveAct{l.Sesion, l.Activacion}]
		switch {
		case l.Prueba:
			r.Falsas++
		case marcada:
			if marca {
				r.Falsas++
			} else {
				r.Buscadas++
			}
		case l.Resultado == ActSinGesto || l.Resultado == ActInterrumpe:
			r.Falsas++
		default:
			r.Buscadas++
		}
	}
	if r.MuestrasRendimiento > 0 {
		n := float64(r.MuestrasRendimiento)
		r.FPSPromedio /= n
		r.CPUCorePromedio /= n
		r.CPUTotalPromedio /= n
		r.ManoMsPromedio /= n
		r.PoseMsPromedio /= n
	}
	return r
}

func minutos(d time.Duration) string { return fmt.Sprintf("%.1f min", d.Minutes()) }

// Veredicto contra «menos de 1 activación falsa en 30 min». Usa la prueba de
// trabajo normal si ya tiene 30 min de uso; si no, la ventana.
func (r Resumen) Veredicto() string {
	if r.PruebaUso >= VentanaResumen {
		if r.PruebaActivaciones == 0 {
			return fmt.Sprintf("PASA: 0 activaciones en la prueba de trabajo normal (%s de uso)", minutos(r.PruebaUso))
		}
		por30 := float64(r.PruebaActivaciones) * VentanaResumen.Minutes() / r.PruebaUso.Minutes()
		if por30 < 1 {
			return fmt.Sprintf("PASA: %d activaciones en %s de prueba (%.2f por 30 min)", r.PruebaActivaciones, minutos(r.PruebaUso), por30)
		}
		return fmt.Sprintf("NO PASA: %d activaciones en %s de prueba de trabajo normal (%.2f por 30 min)", r.PruebaActivaciones, minutos(r.PruebaUso), por30)
	}
	if r.Uso < VentanaResumen {
		return fmt.Sprintf("FALTA USO: hay %s de los 30 min (con la cámara encendida)", minutos(r.Uso))
	}
	if r.Falsas == 0 {
		return "PASA: 0 activaciones falsas en los últimos 30 min de uso"
	}
	return fmt.Sprintf("NO PASA: %d activaciones falsas en los últimos 30 min de uso", r.Falsas)
}

// Texto: el resumen para la persona, unas pocas líneas.
func (r Resumen) Texto() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", r.Veredicto())
	if r.Uso > 0 {
		fmt.Fprintf(&b, "Últimos %s de uso (desde las %s): %d activaciones, %d falsas, %d buscadas\n",
			minutos(r.Uso), r.Desde.Local().Format("15:04 del 02/01"), r.Activaciones, r.Falsas, r.Buscadas)
	}
	if len(r.PorResultado) > 0 {
		fmt.Fprintf(&b, "En qué terminaron: %s\n", conteo(r.PorResultado))
	}
	if r.Evitadas > 0 {
		fmt.Fprintf(&b, "Activaciones evitadas por la zona: %d (%s)\n", r.Evitadas, conteo(r.EvitadasPorMotivo))
	}
	if len(r.EvitadasPanelPorMotivo) > 0 {
		fmt.Fprintf(&b, "Salidas del panel: %s\n", conteo(r.EvitadasPanelPorMotivo))
	}
	if r.MuestrasRendimiento > 0 {
		fmt.Fprintf(&b, "Rendimiento (%d muestras): %.1f cuadros/s, CPU %.1f%% de un núcleo (%.1f%% total), mano %.1f ms + Pose %.1f ms\n",
			r.MuestrasRendimiento, r.FPSPromedio, r.CPUCorePromedio, r.CPUTotalPromedio, r.ManoMsPromedio, r.PoseMsPromedio)
	}
	if !r.PruebaDesde.IsZero() {
		estado := "terminada"
		if r.PruebaAbierta {
			estado = "en curso"
		}
		fmt.Fprintf(&b, "Prueba de trabajo normal (%s, desde las %s): %s de uso, %d activaciones (todas falsas), %d evitadas por la zona\n",
			estado, r.PruebaDesde.Local().Format("15:04 del 02/01"), minutos(r.PruebaUso), r.PruebaActivaciones, r.PruebaEvitadas)
	}
	fmt.Fprintf(&b, "Uso total registrado: %s", minutos(r.UsoTotal))
	return b.String()
}

// conteo ordena un mapa de recuentos como «clave n, clave n».
func conteo(m map[string]int) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	partes := make([]string, 0, len(ks))
	for _, k := range ks {
		partes = append(partes, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(partes, ", ")
}
