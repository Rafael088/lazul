package lazulv1_test

import (
	"bytes"
	"flag"
	"os"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/Rafael088/lazul/proto/go/lazulv1"
)

// El mismo Sobre se arma en Go (aquí) y en TS (proto/ts/prueba.test.ts); los
// dos deben producir y leer exactamente estos bytes.
const golden = "../../testdata/sobre_v1.bin"

var actualizar = flag.Bool("actualizar", false, "reescribir proto/testdata/sobre_v1.bin")

func sobreDePrueba() *lazulv1.Sobre {
	return &lazulv1.Sobre{
		Version:   uint32(lazulv1.Version_VERSION_ACTUAL),
		Id:        "c1-42",
		EnviadoMs: 1_793_000_000_000,
		Contenido: &lazulv1.Sobre_Orden{Orden: &lazulv1.Orden{
			Accion: &lazulv1.Orden_Entregar{Entregar: &lazulv1.Entregar{
				Objeto: &lazulv1.Objeto{Tipo: &lazulv1.Objeto_App{App: &lazulv1.AppRef{
					CatalogoId: "claude",
					NativoId:   "claude.desktop",
					Nombre:     "Claude",
					Web:        "https://claude.ai",
				}}},
			}},
		}},
	}
}

func TestCompatibleConTS(t *testing.T) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(sobreDePrueba())
	if err != nil {
		t.Fatal(err)
	}
	if *actualizar {
		if err := os.WriteFile(golden, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (genéralo con go test ./proto/go/lazulv1 -actualizar)", err)
	}
	if !bytes.Equal(b, want) {
		t.Fatalf("Go produce bytes distintos a %s; si cambió el esquema, -actualizar y revisa proto/ts", golden)
	}
	var leido lazulv1.Sobre
	if err := proto.Unmarshal(want, &leido); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(&leido, sobreDePrueba()) {
		t.Fatalf("ida y vuelta distinta: %v", &leido)
	}
}
