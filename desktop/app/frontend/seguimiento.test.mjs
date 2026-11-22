import test from "node:test";
import assert from "node:assert/strict";
import {
  BUSCA_MIN_MS, SALIDA_MS, avanzar, debeBuscar, detectoresDe, estadoInicial, identificar, seguirCuadro,
} from "./seguimiento.mjs";

const ASPECTO = 4 / 3;
const mano = (x, derecha = true, y = 0.5) => ({
  derecha, p: Array.from({ length: 21 }, (_, i) => [x, i === 0 ? y + 0.06 : y - 0.06, 0]),
});

// Cámara falsa: escena(i) son las manos a la vista. `una` sigue la primera que
// vio hasta perderla, como MediaPipe con numHands 1.
function camara({ fps = 30, frio = 0, preferida = 0 } = {}) {
  let estado = estadoInicial(), conocidas = [], seguida = null, dosFrio = frio, i = 0;
  const n = { una: 0, dos: 0, busca: 0 };
  return {
    n,
    paso(escena) {
      const t = i++ * 1000 / fps;
      const fase = estado.fase;
      if (fase === "una") dosFrio = frio;
      const r = seguirCuadro(estado, t, ASPECTO, {
        una() {
          n.una++;
          const cerca = seguida === null ? null : escena.find((m) => Math.abs(m.p[0][0] - seguida) < 0.1);
          const m = cerca || escena[Math.min(preferida, escena.length - 1)];
          seguida = m ? m.p[0][0] : null;
          return m ? [m] : [];
        },
        // Orden contrario al de `una`, para que lo arregle la identidad.
        dos() { n.dos++; return dosFrio-- > 0 ? escena.slice(0, 1) : [...escena].reverse(); },
        busca() { n.busca++; return escena; },
      });
      estado = r.estado;
      const id = identificar(conocidas, r.manos, t, ASPECTO);
      conocidas = id.conocidas;
      return { t, manos: id.manos, fase: estado.fase, buscada: r.buscada };
    },
  };
}

test("cero manos: solo el detector de una y ninguna búsqueda", () => {
  const c = camara();
  for (let i = 0; i < 300; i++) assert.deepEqual(c.paso([]).manos, []);
  assert.deepEqual(c.n, { una: 300, dos: 0, busca: 0 });
});

test("una mano: no paga la palma en cada cuadro, busca ~2 veces por segundo", () => {
  const c = camara();
  for (let i = 0; i < 300; i++) assert.equal(c.paso([mano(0.4)]).manos.length, 1);
  assert.equal(c.n.una, 300);
  assert.equal(c.n.dos, 0);
  assert.ok(c.n.busca >= 18 && c.n.busca <= 25, `búsquedas ${c.n.busca}`);
});

test("la segunda mano se sigue en 500 ms o menos, aparezca cuando aparezca", () => {
  for (const fps of [20, 25, 30]) {
    for (const frio of [0, 1]) {
      for (let antes = 20; antes < 40; antes++) {
        const c = camara({ fps, frio });
        for (let i = 0; i < antes; i++) c.paso([mano(0.3)]);
        let tarda = null;
        for (let i = 0; i < fps; i++) {
          if (c.paso([mano(0.3), mano(0.7, false)]).manos.length === 2) { tarda = i * 1000 / fps; break; }
        }
        assert.ok(tarda !== null && tarda <= 500.01, `fps ${fps} frío ${frio} cuadro ${antes}: ${tarda}`);
      }
    }
  }
});

test("dos manos que suben juntas se siguen en dos o tres cuadros", () => {
  const c = camara({ frio: 1 });
  for (let i = 0; i < 10; i++) c.paso([]);
  const ns = [0, 1, 2].map(() => c.paso([mano(0.3), mano(0.7, false)]).manos.length);
  assert.deepEqual(ns, [1, 1, 2]);
});

test("con dos a la vista se siguen las dos y no se busca", () => {
  const c = camara();
  for (let i = 0; i < 5; i++) c.paso([mano(0.3), mano(0.7, false)]);
  const antes = { ...c.n };
  for (let i = 0; i < 300; i++) assert.equal(c.paso([mano(0.3), mano(0.7, false)]).manos.length, 2);
  assert.deepEqual(c.n, { ...antes, dos: antes.dos + 300 });
});

test("la segunda se va: vuelve a una tras la histéresis, sin perder la que queda", () => {
  const c = camara();
  for (let i = 0; i < 5; i++) c.paso([mano(0.3), mano(0.7, false)]);
  let vuelta = null;
  for (let i = 0; i < 60; i++) {
    const r = c.paso([mano(0.7, false)]);
    assert.deepEqual(r.manos.map((m) => m.derecha), [false]);
    if (r.fase === "una" && vuelta === null) vuelta = i * 1000 / 30;
  }
  assert.ok(vuelta >= SALIDA_MS && vuelta <= SALIDA_MS + 100, `volvió a los ${vuelta}`);
});

test("un parpadeo de la segunda no oscila entre detectores", () => {
  const c = camara();
  for (let i = 0; i < 5; i++) c.paso([mano(0.3), mano(0.7, false)]);
  for (let i = 0; i < 300; i++) {
    assert.equal(c.paso(i % 20 < 10 ? [mano(0.3)] : [mano(0.3), mano(0.7, false)]).fase, "dos");
  }
});

test("un parpadeo de la única mano no busca en cada cuadro", () => {
  const c = camara();
  for (let i = 0; i < 300; i++) c.paso(i % 2 ? [] : [mano(0.4)]);
  assert.ok(c.n.busca <= 10000 / BUSCA_MIN_MS + 1, `búsquedas ${c.n.busca}`);
  assert.equal(c.n.dos, 0);
});

test("perder la primera con dos: la otra sigue con su lado", () => {
  const c = camara();
  for (let i = 0; i < 5; i++) c.paso([mano(0.3), mano(0.7, false)]);
  for (let i = 0; i < 90; i++) assert.deepEqual(c.paso([mano(0.7, false)]).manos.map((m) => m.derecha), [false]);
});

test("perder la única que se seguía: se toma la otra, con su propio lado", () => {
  const c = camara();
  for (let i = 0; i < 20; i++) c.paso([mano(0.2)]);
  assert.deepEqual(c.paso([mano(0.8, false)]).manos.map((m) => m.derecha), [false]);
});

test("perder la primera durante el relevo: manda la que ve el detector de dos", () => {
  const e = { ...estadoInicial(), fase: "entrando", desde: 100 };
  assert.equal(avanzar(e, 133, { una: 0, dos: 1 }).usar, "dos");
});

test("se van las dos: vuelve a una sin relevo y la siguiente busca enseguida", () => {
  const c = camara();
  for (let i = 0; i < 5; i++) c.paso([mano(0.3), mano(0.7, false)]);
  let r;
  for (let i = 0; i < 40; i++) r = c.paso([]);
  assert.equal(r.fase, "una");
  assert.equal(c.paso([mano(0.5)]).buscada, true);
});

test("un relevo que no cuaja se abandona", () => {
  let e = { ...estadoInicial(), fase: "entrando", desde: 0 };
  e = avanzar(e, 200, { una: 1, dos: 1 }).estado;
  assert.equal(e.fase, "una");
  e = avanzar({ ...e, fase: "saliendo", desde: 0 }, 200, { una: 1, dos: 1, misma: false });
  assert.equal(e.estado.fase, "dos");
  assert.equal(e.usar, "dos");
});

test("al salir no se cambia a otra mano, y si vuelve la segunda se cancela", () => {
  const e = { ...estadoInicial(), fase: "saliendo", desde: 0 };
  assert.deepEqual(detectoresDe(e), { una: true, dos: true });
  assert.equal(avanzar(e, 33, { una: 1, dos: 1, misma: false }).usar, "dos");
  assert.equal(avanzar(e, 33, { una: 1, dos: 2 }).estado.fase, "dos");
  assert.equal(debeBuscar(e, 9999, 1), false);
});

test("apuntar quieto: la mano que llega se sigue pero no roba el puntero", () => {
  for (let antes = 20; antes < 40; antes++) {
    const c = camara();
    for (let i = 0; i < antes; i++) c.paso([mano(0.15, false)]);
    let tarda = null;
    for (let i = 0; i < 90; i++) {
      const r = c.paso([mano(0.15, false), mano(0.6 + i * 0.003)]);
      assert.equal(r.manos[0].derecha, false, `cuadro ${antes}+${i}`);
      if (r.manos.length === 2 && tarda === null) tarda = i * 1000 / 30;
    }
    assert.ok(tarda !== null && tarda <= 500.01, `cuadro ${antes}: ${tarda}`);
  }
});

test("una mano fantasma no mueve el puntero ni por un cuadro", () => {
  for (const dura of [1, 3, 30]) {
    let previas = [];
    for (let i = 0; i < 100; i++) {
      // Apuntar: quieta o avanzando muy despacio.
      const ms = [mano(0.3 + i * 0.0002)];
      if (i >= 40 && i < 40 + dura) ms.push(mano(0.75, false));
      const r = identificar(previas, ms, i * 33, ASPECTO);
      previas = r.conocidas;
      assert.equal(r.manos[0].derecha, true, `dura ${dura}, cuadro ${i}`);
    }
  }
});

test("la primera se va: manda la otra y no lo pierde si vuelve una mano", () => {
  const c = camara();
  for (let i = 0; i < 20; i++) c.paso([mano(0.15, false), mano(0.7)]);
  for (let i = 0; i < 15; i++) assert.equal(c.paso([mano(0.7)]).manos[0].derecha, true);
  for (let i = 0; i < 60; i++) assert.equal(c.paso([mano(0.15, false), mano(0.7)]).manos[0].derecha, true);
});

test("dos que aparecen a la vez conservan el orden de MediaPipe", () => {
  let r = identificar([], [mano(0.8, false), mano(0.2)], 0, ASPECTO);
  assert.deepEqual(r.manos.map((m) => m.derecha), [false, true]);
  r = identificar(r.conocidas, [mano(0.2), mano(0.8, false)], 33, ASPECTO);
  assert.deepEqual(r.manos.map((m) => m.derecha), [false, true]);
});

test("una mano en movimiento conserva el puntero cuando llega otra", () => {
  const c = camara();
  let r;
  for (let i = 0; i < 100; i++) {
    const a = mano(0.3 + 0.15 * Math.sin(i / 5));
    r = c.paso(i < 40 ? [a] : [a, mano(0.85, false)]);
    assert.equal(r.manos[0].derecha, true, `cuadro ${i}`);
  }
  assert.equal(r.manos.length, 2);
});

test("el cambio no corta un arrastre: misma clave en todos los cuadros", () => {
  const c = camara({ frio: 1 });
  for (let i = 0; i < 180; i++) {
    // `una` y `dos` pueden dar otro lado para la misma mano.
    const x = 0.25 + 0.1 * Math.sin(i / 10);
    const a = mano(x, i < 30 || i % 7 !== 0);
    const r = c.paso(i >= 30 && i < 90 ? [a, mano(0.8, false)] : [a]);
    const suya = r.manos.filter((m) => m.p[0][0] === x);
    assert.equal(suya.length, 1, `cuadro ${i}`);
    assert.equal(suya[0].derecha, true, `cuadro ${i}`);
  }
});

test("dos manos del mismo lado u otro orden no intercambian sus claves", () => {
  const a = mano(0.25), b = mano(0.75);
  let r = identificar([], [a], 0, ASPECTO);
  r = identificar(r.conocidas, [b, a], 33, ASPECTO);
  const clave = (x) => r.manos.find((m) => m.p[0][0] === x).derecha;
  assert.deepEqual([clave(0.25), clave(0.75)], [true, false]);
  r = identificar(r.conocidas, [a, b], 66, ASPECTO);
  assert.deepEqual([clave(0.25), clave(0.75)], [true, false]);
  r = identificar(r.conocidas, [b], 99, ASPECTO);
  assert.equal(clave(0.75), false);
  r = identificar(r.conocidas, [b, a], 165, ASPECTO);
  assert.deepEqual([clave(0.25), clave(0.75)], [true, false]);
});

test("la memoria de una mano perdida caduca", () => {
  const r = identificar([], [mano(0.4)], 0, ASPECTO);
  assert.equal(identificar(r.conocidas, [mano(0.4, false)], 200, ASPECTO).manos[0].derecha, true);
  assert.equal(identificar(r.conocidas, [mano(0.4, false)], 300, ASPECTO).manos[0].derecha, false);
});
