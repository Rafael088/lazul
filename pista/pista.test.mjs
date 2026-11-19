import test from "node:test";
import assert from "node:assert/strict";

import "./resumen.mjs";

const { calcularResumen } = globalThis.LazulResumen;

test("calcula aciertos a la primera, fallos y tiempos", () => {
  const resultado = calcularResumen({
    inicio: "2026-11-19T10:00:00.000Z",
    fin: "2026-11-19T10:01:00.000Z",
    reposo: { duracionPlaneadaMs: 600000, duracionRealMs: 60000, toquesFalsos: 1 },
    blancos: [
      { id: "enlace-28", tipo: "enlace", tamanoPx: 28, posicion: "centro", tiempoMs: 1000, toquesFuera: 0 },
      { id: "boton-44", tipo: "botón", tamanoPx: 44, posicion: "derecha", tiempoMs: 3500, toquesFuera: 2 },
      { id: "icono-32", tipo: "ícono", tamanoPx: 32, posicion: "esquina", tiempoMs: 2500, toquesFuera: 0 },
    ],
  });

  assert.equal(resultado.blancos_total, 3);
  assert.equal(resultado.aciertos_primera, 2);
  assert.equal(resultado.fallos, 1);
  assert.equal(resultado.toques_fuera_blanco, 2);
  assert.equal(resultado.tiempo_promedio_ms, 2333);
  assert.equal(resultado.tiempo_mediana_ms, 2500);
  assert.equal(resultado.tiempo_p95_ms, 3500);
  assert.equal(resultado.reposo.toques_falsos, 1);
  assert.equal(resultado.reposo.completo, false);
  assert.equal("posiciones_camara" in resultado, false);
});

test("tolera una sesión vacía sin producir NaN", () => {
  const resultado = calcularResumen({ blancos: [], reposo: {} });
  assert.equal(resultado.blancos_total, 0);
  assert.equal(resultado.precision_primera_pct, 0);
  assert.equal(resultado.tiempo_promedio_ms, 0);
  assert.equal(resultado.tiempo_mediana_ms, 0);
  assert.equal(resultado.tiempo_p95_ms, 0);
  assert.deepEqual(resultado.blancos, []);
});
