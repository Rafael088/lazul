import test from "node:test";
import assert from "node:assert/strict";
import {
  INTERVALO_CUADRO_MS,
  cuadrosEntrePose,
  mediaSuave,
  planearSiguienteCuadro,
} from "./rendimiento.mjs";

test("Pose conserva su frecuencia normal y baja carga cuando Hand se alarga", () => {
  assert.equal(cuadrosEntrePose(17), 3);
  assert.equal(cuadrosEntrePose(23.99), 3);
  assert.equal(cuadrosEntrePose(24), 6);
  assert.equal(cuadrosEntrePose(41), 6);
  assert.equal(mediaSuave(20, 30), 22);
});

test("el reloj de Windows no suma 33 ms al tiempo de inferencia", () => {
  const aTiempo = planearSiguienteCuadro(12, 0);
  assert.equal(aTiempo.objetivo, INTERVALO_CUADRO_MS);
  assert.ok(Math.abs(aTiempo.espera - (INTERVALO_CUADRO_MS - 12)) < 1e-9);

  const tarde = planearSiguienteCuadro(45, 0);
  assert.deepEqual(tarde, { objetivo: 45, espera: 0 });
  const recuperado = planearSiguienteCuadro(46, tarde.objetivo);
  assert.ok(Math.abs(recuperado.espera - (INTERVALO_CUADRO_MS - 1)) < 1e-9);
});
