// Los tipos de TS leen y escriben los mismos bytes que Go
// (proto/go/lazulv1/compat_test.go arma el mismo Sobre).
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { SobreSchema, Version, Gesto, EventoGestoSchema } from "./src/lazul/v1/lazul_pb.ts";

const golden = new Uint8Array(readFileSync(new URL("../testdata/sobre_v1.bin", import.meta.url)));

test("lee el Sobre que escribió Go", () => {
  const s = fromBinary(SobreSchema, golden);
  assert.equal(s.version, Version.ACTUAL);
  assert.equal(s.id, "c1-42");
  assert.equal(s.enviadoMs, 1_793_000_000_000n);
  assert.equal(s.contenido.case, "orden");
  const accion = s.contenido.value.accion;
  assert.equal(accion.case, "entregar");
  const obj = accion.value.objeto?.tipo;
  assert.equal(obj?.case, "app");
  assert.equal(obj.value.catalogoId, "claude");
  assert.equal(obj.value.web, "https://claude.ai");
});

test("escribe los mismos bytes que Go", () => {
  const s = create(SobreSchema, {
    version: Version.ACTUAL,
    id: "c1-42",
    enviadoMs: 1_793_000_000_000n,
    contenido: {
      case: "orden",
      value: {
        accion: {
          case: "entregar",
          value: {
            objeto: {
              tipo: {
                case: "app",
                value: { catalogoId: "claude", nativoId: "claude.desktop", nombre: "Claude", web: "https://claude.ai" },
              },
            },
          },
        },
      },
    },
  });
  assert.deepEqual(toBinary(SobreSchema, s), golden);
});

test("evento de gesto de la webview", () => {
  const ev = create(EventoGestoSchema, { gesto: Gesto.ACTIVAR, confianza: 0.9, cuadroMs: 10n });
  const de = fromBinary(EventoGestoSchema, toBinary(EventoGestoSchema, ev));
  assert.equal(de.gesto, Gesto.ACTIVAR);
});
