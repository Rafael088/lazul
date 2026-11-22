// Variante B (cotejo-cb8m9r): seguir una mano y buscar la segunda de vez en
// cuando, en lugar de correr la detección de palma en cada cuadro.
//
// MediaPipe en VIDEO solo se ahorra la palma cuando ya sigue numHands manos, y
// setOptions rehace el grafo (setGraph), o sea que pierde el seguimiento. Por
// eso hay tres detectores fijos:
//   una   VIDEO, 1 mano: el de casi todos los cuadros.
//   dos   VIDEO, 2 manos: mientras hay dos a la vista.
//   busca IMAGE, 2 manos: la búsqueda. No usa `dos` porque, al correr salteado,
//         su recorte anterior está viejo y ocupa uno de los dos cupos.
// Al cambiar, el que entra corre junto al que sale hasta que ve lo mismo; hasta
// entonces manda el que sale.
//
// Todo lo de este archivo es puro: el estado entra y sale, y los detectores se
// inyectan.

export const SIGUE_DOS = "dos"; // como antes: numHands 2 en cada cuadro
export const SIGUE_BUSCA = "busca";

// 433 ms y no 500: la mano puede aparecer justo después de una búsqueda y el
// cambio tarda uno o dos cuadros más. El total queda en 500 ms entre 20 y 30 fps.
export const BUSCA_MS = 433;
// Una mano recién vista busca enseguida (dos manos que suben juntas), pero un
// parpadeo no puede convertirse en una búsqueda por cuadro.
export const BUSCA_MIN_MS = 130;
// Histéresis: tiempo seguido sin la segunda mano para volver a seguir una.
export const SALIDA_MS = 1000;
// Si el detector que entra no ve lo mismo en este plazo, no se cambia.
export const RELEVO_MS = 200;

export function estadoInicial() {
  return { fase: "una", desde: 0, sinSegunda: null, habia: false, ultimaBusca: -Infinity };
}

// Qué detectores de vídeo corren en este cuadro.
export function detectoresDe(estado) {
  return { una: estado.fase !== "dos", dos: estado.fase !== "una" };
}

export function debeBuscar(estado, t, manosUna) {
  if (estado.fase !== "una" || manosUna < 1) return false;
  const espera = estado.habia ? BUSCA_MS : BUSCA_MIN_MS;
  return t - estado.ultimaBusca >= espera;
}

// vio trae cuántas manos dio cada detector que corrió (null si no corrió) y,
// al salir de dos, si `una` sigue la misma mano que `dos`. Devuelve el estado
// siguiente y de qué detector son las manos que van a Go.
export function avanzar(estado, t, { una = null, dos = null, busca = null, misma = false }) {
  const e = { ...estado };
  let usar = "una";
  if (busca !== null) e.ultimaBusca = t;

  if (e.fase === "una") {
    if (busca !== null && busca >= 2) Object.assign(e, { fase: "entrando", desde: t });
  } else if (e.fase === "entrando") {
    if (dos >= 2) {
      Object.assign(e, { fase: "dos", sinSegunda: null });
      usar = "dos";
    } else {
      if (una === 0 && dos > 0) usar = "dos"; // la primera se perdió en el relevo
      if (t - e.desde >= RELEVO_MS) e.fase = "una";
    }
  } else {
    usar = "dos";
    if (dos >= 2) {
      Object.assign(e, { fase: "dos", sinSegunda: null });
    } else if (e.fase === "dos") {
      if (e.sinSegunda === null) e.sinSegunda = t;
      if (t - e.sinSegunda >= SALIDA_MS) {
        if (dos === 0) e.fase = "una"; // no hay seguimiento que perder
        else Object.assign(e, { fase: "saliendo", desde: t });
      }
    } else if (dos === 0 || (una === 1 && misma)) {
      e.fase = "una";
      if (una === 1) usar = "una";
      // La búsqueda que sigue cuenta desde el cambio: `dos` acaba de mirar.
      e.ultimaBusca = t;
    } else if (t - e.desde >= RELEVO_MS) {
      Object.assign(e, { fase: "dos", sinSegunda: t });
    }
  }
  e.habia = (usar === "dos" ? dos : una) > 0;
  return { estado: e, usar };
}

const medir = (m, aspecto) => {
  const [a, b] = [m.p[0], m.p[9]];
  const tam = Math.hypot((b[0] - a[0]) * aspecto, b[1] - a[1]);
  return { x: (a[0] + b[0]) / 2 * aspecto, y: (a[1] + b[1]) / 2, tam: Math.max(tam, 0.05) };
};
const separadas = (a, b) => Math.hypot(a.x - b.x, a.y - b.y) / Math.max(a.tam, b.tam);

export function mismaMano(a, b, aspecto) {
  return separadas(medir(a, aspecto), medir(b, aspecto)) < 1;
}

// Un cuadro completo. correr = { una, dos, busca }: cada uno devuelve sus manos.
export function seguirCuadro(estado, t, aspecto, correr) {
  const toca = detectoresDe(estado);
  const una = toca.una ? correr.una() : null;
  const dos = toca.dos ? correr.dos() : null;
  const busca = una && debeBuscar(estado, t, una.length) ? correr.busca() : null;
  const misma = estado.fase === "saliendo" && una.length === 1 && dos.length === 1 && mismaMano(una[0], dos[0], aspecto);
  const r = avanzar(estado, t, {
    una: una && una.length, dos: dos && dos.length, busca: busca && busca.length, misma,
  });
  return { estado: r.estado, manos: r.usar === "dos" ? dos : una, buscada: busca !== null };
}

// ---------- identidad entre detectores ----------
// Go reconoce cada mano por (lado, orden entre las de ese lado) y de ahí salen
// la histéresis del pellizco y la ranura del dedo. Dos detectores pueden dar
// otro lado u otro orden para la misma mano, así que el lado se fija al nacer
// la mano, nunca se repite en un cuadro y se conserva por cercanía.

// Lo mismo que el embrague aguanta una mano perdida antes de soltar.
export const MEMORIA_MS = 250;
const CERCA_PALMAS = 2.5;

// previas es lo que devolvió la llamada anterior en `conocidas` ([] al empezar).
// La primera mano es la que lee la L y la que mueve el puntero: una que llega
// va siempre detrás, aunque la que estaba esté quieta, porque quieta es como
// se apunta. Si la primera está fuera del panel la quita su filtro, en Go.
export function identificar(previas, manos, t, aspecto) {
  const vivas = previas.filter((c) => t - c.t <= MEMORIA_MS);
  const vistas = manos.map((m) => ({ m, ...medir(m, aspecto) }));
  const pares = [];
  for (const v of vistas) {
    for (const c of vivas) {
      const d = separadas(v, c);
      // El lado solo desempata cuando las manos se cruzan.
      if (d <= CERCA_PALMAS) pares.push({ v, c, costo: d + (v.m.derecha === c.derecha ? 0 : 0.5) });
    }
  }
  pares.sort((a, b) => a.costo - b.costo);

  const usadas = new Set();
  const presentes = [];
  for (const { v, c } of pares) {
    if (v.id || usadas.has(c)) continue;
    usadas.add(c);
    v.id = { derecha: c.derecha, rango: c.rango };
    presentes.push(v);
  }
  let siguiente = Math.max(-1, ...vivas.map((c) => c.rango)) + 1;
  for (const v of vistas) {
    if (v.id) continue;
    const derecha = presentes.some((o) => o.id.derecha === v.m.derecha) ? !v.m.derecha : v.m.derecha;
    v.id = { derecha, rango: siguiente++ };
    presentes.push(v);
  }
  presentes.sort((a, b) => a.id.rango - b.id.rango);

  const conocidas = presentes.map((v) => ({ ...v.id, x: v.x, y: v.y, tam: v.tam, t }));
  for (const c of vivas) {
    if (!conocidas.some((o) => o.derecha === c.derecha)) conocidas.push(c);
  }
  return { conocidas, manos: presentes.map((v) => ({ ...v.m, derecha: v.id.derecha })) };
}
