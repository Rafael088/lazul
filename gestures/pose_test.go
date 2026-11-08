package gestures

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"testing"
)

// Cada forma, en varias colocaciones (mano izquierda y derecha, inclinada,
// grande y chica), da la pose que debe.
func TestPoseDeCadaForma(t *testing.T) {
	casos := []struct {
		nombre string
		f      forma
		quiere Pose
	}{
		{"L", formaL(), PoseL},
		{"puño", formaPuno(), PosePuno},
		{"abierta", formaAbierta(), PoseAbierta},
		{"pinza", formaPinza(false), PosePinza},
		{"pinza con los otros dedos estirados", formaPinza(true), PosePinza},
		{"señalar", formaSenalar(), PoseSenalar},
		{"señalar con el pulgar a medio salir", formaSenalarMedio(), PoseSenalar},
		{"señalar con el pulgar perdido", formaSenalarSinPulgar(), PoseSenalar},
		{"C (pinza a medio cerrar)", formaC(), PoseOtra},
		{"relajada", formaRelajada(), PoseOtra},
	}
	for _, c := range casos {
		for i := 0; i < 12; i++ {
			col := colocacionN(i)
			if got := PoseDe(col.mano(c.f.puntos(), v3{}), float32(col.aspecto)); got != c.quiere {
				t.Errorf("%s, colocación %d: %v, quiere %v", c.nombre, i, got, c.quiere)
			}
		}
	}
}

// dec-rwrs: la L no se confunde con la pinza ni con señalar con el índice,
// tampoco con ruido de mala luz. Y la L con ruido se sigue viendo.
func TestLNoSeConfunde(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	ruidosa := func(f forma, i int) Mano {
		col := colocacionN(i)
		m := col.mano(f.puntos(), v3{})
		for k := range m.Puntos {
			m.Puntos[k].X += float32(rng.NormFloat64() * luzMala.sigma)
			m.Puntos[k].Y += float32(rng.NormFloat64() * luzMala.sigma)
		}
		return m
	}
	negativos := map[string]forma{
		"pinza": formaPinza(false), "pinza ok": formaPinza(true), "C": formaC(),
		"señalar": formaSenalar(), "señalar pulgar a medias": formaSenalarMedio(), "señalar sin pulgar": formaSenalarSinPulgar(),
	}
	const n = 600
	for nombre, f := range negativos {
		for i := 0; i < n; i++ {
			if PoseDe(ruidosa(f, i), 4.0/3) == PoseL {
				t.Errorf("%s tomada por L (colocación %d)", nombre, i%12)
			}
		}
	}
	aciertos := 0
	for i := 0; i < n; i++ {
		if PoseDe(ruidosa(formaL(), i), 4.0/3) == PoseL {
			aciertos++
		}
	}
	if aciertos < n*97/100 {
		t.Errorf("la L con ruido se vio en %d de %d cuadros", aciertos, n)
	}
}

// Sin corregir el aspecto, un pulgar a 90° en la imagen de 4:3 ya no está a
// 90°; con la corrección, la misma mano da lo mismo en 4:3 y en 16:9.
func TestAspecto(t *testing.T) {
	for _, asp := range []float64{4.0 / 3, 16.0 / 9, 1} {
		col := colocacion{cx: 0.7, cy: 0.8, escala: 0.2, aspecto: asp}
		if got := PoseDe(col.mano(formaL().puntos(), v3{}), float32(asp)); got != PoseL {
			t.Errorf("aspecto %.2f: %v", asp, got)
		}
	}
}

func TestManoDegenerada(t *testing.T) {
	if got := PoseDe(Mano{}, 0); got != PoseOtra {
		t.Errorf("todos los puntos en 0: %v", got)
	}
}

// Puntos reales de MediaPipe (la foto de prueba de MediaPipe, un índice
// señalando hacia arriba): es señalar, no la L.
func TestPoseConPuntosDeMediaPipe(t *testing.T) {
	b, err := os.ReadFile("testdata/mediapipe-senalar.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Ancho, Alto int
		P           [][3]float32
	}
	if err := json.Unmarshal(b, &v); err != nil || len(v.P) != NumPuntos {
		t.Fatalf("%v, %d puntos", err, len(v.P))
	}
	var m Mano
	for i, p := range v.P {
		m.Puntos[i] = Punto{p[0], p[1], p[2]}
	}
	if got := PoseDe(m, float32(v.Ancho)/float32(v.Alto)); got != PoseSenalar {
		t.Errorf("la foto de señalar dio %v", got)
	}
}
