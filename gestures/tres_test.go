package gestures

import "testing"

// formaTres: índice, medio y anular estirados; el meñique recogido y el
// pulgar cruzado sobre la palma.
func formaTres() forma {
	return conPulgar(dedos([4]float64{0, 0, 0, 1}), v3{-0.10, 0.62, -0.15})
}

// formaTresPulgarPegado: el pulgar no cruza, queda pegado al costado del
// índice (otra manera natural de recogerlo).
func formaTresPulgarPegado() forma {
	return conPulgar(dedos([4]float64{0, 0, 0, 0.9}), v3{0.30, 0.70, -0.10})
}

// formaCuatroPulgarRecogido: como la de tres pero con el meñique estirado.
func formaCuatroPulgarRecogido() forma {
	return conPulgar(dedos([4]float64{0, 0, 0, 0}), v3{-0.10, 0.62, -0.15})
}

// cada15 deja un cuadro de cada dos: la cámara a ~15 fps.
func cada15(cs []Cuadro) []Cuadro {
	var out []Cuadro
	for i, c := range cs {
		if i%2 == 0 {
			out = append(out, c)
		}
	}
	return out
}

func pasarTres(d *DetectorTresDedos, cs []Cuadro) (lados []LadoPantalla, intentos []IntentoTres) {
	for _, c := range cs {
		r := d.Cuadro(c)
		if r.Lado != PantallaNinguna {
			lados = append(lados, r.Lado)
		}
		if r.Intento != nil {
			intentos = append(intentos, *r.Intento)
		}
	}
	if in := d.Reiniciar(MotivoCorto); in != nil {
		intentos = append(intentos, *in)
	}
	return lados, intentos
}

func TestTresDedosPose(t *testing.T) {
	col := colocacionN(0)
	casos := []struct {
		nombre string
		f      forma
		quiere bool
	}{
		{"tres, pulgar cruzado", formaTres(), true},
		{"tres, pulgar pegado", formaTresPulgarPegado(), true},
		{"abierta", formaAbierta(), false},
		{"cuatro con el pulgar recogido", formaCuatroPulgarRecogido(), false},
		{"dos dedos", formaDosDedos(), false},
		{"señalar", formaSenalar(), false},
		{"L", formaL(), false},
		{"puño", formaPuno(), false},
		{"pinza ok", formaPinza(true), false},
		{"agarre", formaAgarre(), false},
		{"relajada", formaRelajada(), false},
	}
	for _, c := range casos {
		for _, izq := range []bool{false, true} {
			col.izquierda = izq
			m := col.mano(c.f.puntos(), v3{})
			if got := TresDedos(m, float32(col.aspecto)); got != c.quiere {
				r := medir(m, float32(col.aspecto))
				t.Errorf("%s (izquierda=%v): TresDedos = %v, quiere %v (ext %.2f pulgar %.2f)", c.nombre, izq, got, c.quiere, r.ext, r.pulgar)
			}
		}
	}
}

// tomaTres arma una toma sintética del gesto: mano relajada, la pose,
// deslizarla mov (en altos de imagen) en ms y volver a relajarla.
func tomaTres(i int, f forma, mov v3, ms int64) []Cuadro {
	col := colocacionN(i)
	_, rd := luzN(i, uint64(900+i))
	return sintetizar(col, rd, []tramo{
		{f: formaRelajada(), ms: 300},
		{f: f, ms: 250, trans: 150},
		{f: f, ms: ms, mov: mov},
		{f: formaRelajada(), ms: 400, trans: 150},
	})
}

func TestTresDedosDispara(t *testing.T) {
	for i := range 12 {
		col := colocacionN(i)
		palma := col.escala
		lado := []float64{-1, 1}[i%2]
		quiere := PantallaDerecha // hacia la X menor: la derecha de la persona
		if lado > 0 {
			quiere = PantallaIzquierda
		}
		f := formaTres()
		if i%3 == 2 {
			f = formaTresPulgarPegado()
		}
		casos := []struct {
			nombre string
			ms     int64
			fps15  bool
		}{
			{"30 fps", 600, false},
			{"15 fps", 600, true},
			{"despacio, 2 s", 2000, true},
		}
		for _, c := range casos {
			cs := tomaTres(i, f, v3{1.4 * palma * lado, 0.1 * palma, 0}, c.ms)
			if c.fps15 {
				cs = cada15(cs)
			}
			lados, intentos := pasarTres(NuevoDetectorTresDedos(), cs)
			if len(lados) != 1 || lados[0] != quiere {
				t.Errorf("toma %d %s: lados %v, quiere [%v]; intentos %+v", i, c.nombre, lados, quiere, intentos)
				continue
			}
			for _, in := range intentos {
				if in.Motivo != MotivoDisparo {
					t.Errorf("toma %d %s: intento de más %+v", i, c.nombre, in)
				}
			}
		}
	}
}

func TestTresDedosNoDispara(t *testing.T) {
	col := colocacionN(0)
	p := col.escala
	casos := []struct {
		nombre string
		cs     []Cuadro
		motivo string // el del intento, si hay pose
	}{
		{"quieta", tomaTres(0, formaTres(), v3{0.2 * p, 0, 0}, 800), MotivoCorto},
		{"corta", tomaTres(0, formaTres(), v3{0.7 * p, 0, 0}, 600), MotivoCorto},
		{"vertical", tomaTres(0, formaTres(), v3{0.1 * p, 1.5 * p, 0}, 600), MotivoVertical},
		{"abierta a un lado", tomaTres(0, formaAbierta(), v3{2 * p, 0, 0}, 600), ""},
		{"cuatro dedos a un lado", tomaTres(0, formaCuatroPulgarRecogido(), v3{2 * p, 0, 0}, 600), ""},
		{"dos dedos a un lado", tomaTres(0, formaDosDedos(), v3{2 * p, 0, 0}, 600), ""},
		{"señalar a un lado", tomaTres(0, formaSenalar(), v3{2 * p, 0, 0}, 600), ""},
		{"pinza arrastrada", tomaTres(0, formaPinza(true), v3{2 * p, 0, 0}, 600), ""},
	}
	for _, c := range casos {
		lados, intentos := pasarTres(NuevoDetectorTresDedos(), c.cs)
		if len(lados) != 0 {
			t.Errorf("%s: disparó %v", c.nombre, lados)
		}
		if c.motivo == "" {
			for _, in := range intentos {
				t.Errorf("%s: intento %+v sin la pose", c.nombre, in)
			}
			continue
		}
		if len(intentos) != 1 || intentos[0].Motivo != c.motivo {
			t.Errorf("%s: intentos %+v, quiere uno %q", c.nombre, intentos, c.motivo)
		}
	}

	t.Run("dos manos en la pose", func(t *testing.T) {
		a := tomaTres(0, formaTres(), v3{1.5 * p, 0, 0}, 600)
		b := tomaTres(0, formaTres(), v3{-1.5 * p, 0, 0}, 600)
		for i := range a {
			if i < len(b) && len(b[i].Manos) == 1 && len(a[i].Manos) == 1 {
				m := b[i].Manos[0]
				for k := range m.Puntos {
					m.Puntos[k].Y -= 0.3
				}
				a[i].Manos = append(a[i].Manos, m)
			}
		}
		if lados, _ := pasarTres(NuevoDetectorTresDedos(), a); len(lados) != 0 {
			t.Fatalf("disparó %v", lados)
		}
	})
	t.Run("salto del seguimiento", func(t *testing.T) {
		m := col.mano(formaTres().puntos(), v3{})
		var cs []Cuadro
		for i := range 6 {
			cs = append(cs, Cuadro{TMs: int64(i) * 66, Aspecto: float32(col.aspecto), Manos: []Mano{m}})
		}
		lejos := col.mano(formaTres().puntos(), v3{2.5 * p, 0, 0})
		for i := range 6 {
			cs = append(cs, Cuadro{TMs: int64(6+i) * 66, Aspecto: float32(col.aspecto), Manos: []Mano{lejos}})
		}
		lados, intentos := pasarTres(NuevoDetectorTresDedos(), cs)
		if len(lados) != 0 {
			t.Fatalf("disparó %v", lados)
		}
		if len(intentos) == 0 || intentos[0].Motivo != MotivoSalto {
			t.Fatalf("intentos %+v, quiere primero %q", intentos, MotivoSalto)
		}
	})
	t.Run("30 s de manos sueltas", func(t *testing.T) {
		for s := range uint64(4) {
			cs := sintetizar(colocacionN(int(s)), luzMala, tramosNormales(200+s, 30000))
			if lados, _ := pasarTres(NuevoDetectorTresDedos(), cs); len(lados) != 0 {
				t.Errorf("semilla %d: disparó %v", s, lados)
			}
		}
	})
}

// Un disparo por pose: seguir deslizando o volver no dispara otra vez; soltar
// la pose y repetir, sí.
func TestTresDedosUnoPorPose(t *testing.T) {
	col := colocacionN(0)
	p := col.escala
	_, rd := luzN(0, 77)
	cs := sintetizar(col, rd, []tramo{
		{f: formaRelajada(), ms: 300},
		{f: formaTres(), ms: 250, trans: 150},
		{f: formaTres(), ms: 600, mov: v3{1.4 * p, 0, 0}},
		{f: formaTres(), ms: 600, mov: v3{1.4 * p, 0, 0}},
		{f: formaTres(), ms: 800, mov: v3{-2.8 * p, 0, 0}},
		{f: formaRelajada(), ms: 500, trans: 150},
		{f: formaTres(), ms: 250, trans: 150},
		{f: formaTres(), ms: 600, mov: v3{1.4 * p, 0, 0}},
		{f: formaRelajada(), ms: 400, trans: 150},
	})
	lados, _ := pasarTres(NuevoDetectorTresDedos(), cs)
	if len(lados) != 2 || lados[0] != PantallaIzquierda || lados[1] != PantallaIzquierda {
		t.Fatalf("lados %v, quiere [izquierda izquierda]", lados)
	}
}

// TestTresDedosSinFalsos: fuera de las tomas tres-derecha/izquierda,
// ninguna grabación real (gestos 0.1 ni tomas del pellizco) tiene el gesto,
// así que nada dispara; las que sí lo tienen disparan una vez a su lado.
// Medido el 21-nov: la pose sale en 9 de ~21 000 manos, siempre en un
// cuadro suelto.
func TestTresDedosSinFalsos(t *testing.T) {
	var gs []Grabacion
	for _, dir := range []string{"../fixtures/gestos", "../fixtures/pellizco"} {
		g, err := LeerDirectorio(dir)
		if err != nil {
			t.Fatal(err)
		}
		gs = append(gs, g...)
	}
	if len(gs) < 100 {
		t.Fatalf("solo %d grabaciones", len(gs))
	}
	for _, m := range MedirTresDedos(gs) {
		if !m.Acierto() {
			t.Errorf("%s: quiere %v, disparos %+v", m.Archivo, m.Quiere, m.Disparos)
		}
	}
}
