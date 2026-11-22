// Graba tomas de gestos: cámara → MediaPipe Hand Landmarker → cuadros en el
// formato de gestures/grabacion.go. Los puntos se guardan tal como los da
// MediaPipe (imagen sin espejar); el video solo se muestra espejado.
//
// Siempre se piden las dos manos (numHands: 2). Las tomas del modelo nuevo
// (toma:*) guardan los puntos crudos y la etiqueta; el servidor calcula y
// guarda el estado del pellizco de cada mano en cada cuadro. La cabecera
// deja "panel" vacío: lo llenará el panel calibrado cuando exista.
import { FilesetResolver, HandLandmarker } from "./vendor/vision_bundle.mjs";

const $ = (id) => document.getElementById(id);
const GUIAS = {
  ACTIVAR: "Empieza con la mano quieta o fuera de cuadro. Haz la L (índice arriba, pulgar al lado, los demás recogidos) de frente a la cámara, sostenla un segundo y bájala. Una L por toma.",
  AGARRAR: "La toma arranca como si ya hubieras hecho la L. Con la mano abierta o suelta, cierra el puño y mantenlo medio segundo.",
  SOLTAR: "La toma arranca como si ya tuvieras algo agarrado. Empieza con el puño cerrado y ábrelo moviendo la mano hacia un lado (alterna izquierda y derecha entre tomas).",
  MANDAR_APP: "La toma arranca como si ya hubieras hecho la L. Junta las puntas del pulgar y el índice y arrastra hacia un lado o hacia arriba; luego suelta.",
  "NINGUNO:pinza": "Negativo de la L: pinzas (cerradas, a medio cerrar, con los otros dedos estirados o recogidos) sin hacer la L. No debe activar nada.",
  "NINGUNO:senalar": "Negativo de la L: señala con el índice hacia arriba con el pulgar pegado, a medio salir o escondido. No debe activar nada.",
  "NINGUNO:normal": "Unos 2 minutos de trabajo normal frente a la cámara: escribir, tomar café, hablar con las manos. Sin gestos a propósito. No debe activar nada.",
  "toma:arrastrar-arriba": "Pon la mano dentro del panel imaginario (un rectángulo delante de ti, del tamaño de la pantalla). Pellizca con pulgar e índice y arrastra hacia arriba un palmo; suelta al final. Un arrastre por toma.",
  "toma:arrastrar-abajo": "Dentro del panel imaginario, pellizca con pulgar e índice y arrastra hacia abajo un palmo; suelta al final. Un arrastre por toma.",
  "toma:arrastrar-izquierda": "Dentro del panel imaginario, pellizca con pulgar e índice y arrastra hacia tu izquierda un palmo; suelta al final. Un arrastre por toma.",
  "toma:arrastrar-derecha": "Dentro del panel imaginario, pellizca con pulgar e índice y arrastra hacia tu derecha un palmo; suelta al final. Un arrastre por toma.",
  "toma:entrar-panel": "Empieza con la mano fuera del panel imaginario y entra en él sin pellizcar; puedes cruzarlo hasta el otro lado. Sin pellizco: solo entrar.",
  "toma:salir-panel": "Empieza con la mano dentro del panel imaginario, sin pellizcar, y sácala hacia fuera. Sin pellizco: solo salir.",
  "toma:zoom-dos-manos": "Con las dos manos a la vista, pellizca con las dos y sepáralas para acercar; vuelve a juntarlas para alejar. Dos o tres ciclos por toma, sin soltar el pellizco.",
  "toma:tres-derecha": "Estira índice, medio y anular con el pulgar y el meñique recogidos y desliza la mano hacia tu derecha más o menos un palmo, sin prisa; luego baja la mano. Un deslizamiento por toma.",
  "toma:tres-izquierda": "Estira índice, medio y anular con el pulgar y el meñique recogidos y desliza la mano hacia tu izquierda más o menos un palmo, sin prisa; luego baja la mano. Un deslizamiento por toma.",
  "toma:normal": "Dos minutos de trabajo normal frente a la cámara, con una o las dos manos a la vista: escribir, tomar café, hablar con las manos. Sin pellizcos a propósito.",
};
const NOTA_PANEL = "Sin calibración del panel: la toma guarda los puntos crudos de las dos manos y el estado del pellizco; «panel» queda vacío en la cabecera.";

let grabando = false, toma = null, ultima = null, t0Vivo = performance.now(), cola = Promise.resolve();

function seleccion() {
  const [clase, detalle] = $("gesto").value.split(":");
  if (clase === "toma") return { gesto: "NINGUNO", negativo: "", toma: detalle, luz: $("luz").value, nota: $("nota").value.trim() };
  return { gesto: clase, negativo: detalle || "", toma: "", luz: $("luz").value, nota: $("nota").value.trim() };
}

async function reiniciarVivo() {
  t0Vivo = performance.now();
  cola = cola.then(() => fetch("/reiniciar?gesto=" + encodeURIComponent(seleccion().gesto), { method: "POST" })).catch(() => {});
  await cola;
}

async function actualizarCuenta() {
  try {
    const n = await (await fetch("/cuenta")).json();
    const lineas = [];
    for (const [carpeta, titulo] of [["gestos", "gestos viejos"], ["pellizco", "pellizco"]]) {
      const claves = Object.keys(n[carpeta] || {}).sort();
      if (claves.length) lineas.push(titulo + ":", ...claves.map((k) => `  ${k.padEnd(22)} ${String(n[carpeta][k]).padStart(3)} / 20`));
    }
    $("cuenta").textContent = lineas.length ? "En la carpeta:\n" + lineas.join("\n") : "Aún no hay tomas.";
  } catch {}
}

function mostrarGuia() {
  const s = seleccion();
  $("guia").textContent = GUIAS[$("gesto").value] || "";
  $("notaPanel").textContent = s.toma ? NOTA_PANEL : "";
  reiniciarVivo();
}

async function alternar() {
  if (!grabando) {
    const s = seleccion();
    toma = { cabecera: { gesto: s.gesto, luz: s.luz, ancho: dims.ancho, alto: dims.alto }, cuadros: [], t0: performance.now() };
    if (s.negativo) toma.cabecera.negativo = s.negativo;
    if (s.toma) toma.cabecera.toma = s.toma;
    if (s.nota) toma.cabecera.nota = s.nota;
    await reiniciarVivo();
    toma.t0 = performance.now();
    grabando = true;
    $("escena").classList.add("grabando");
    $("grabar").textContent = "Parar y guardar";
    return;
  }
  grabando = false;
  $("escena").classList.remove("grabando");
  $("grabar").textContent = "Grabar";
  const lineas = [JSON.stringify(toma.cabecera), ...toma.cuadros.map((c) => JSON.stringify(c))].join("\n") + "\n";
  const r = await fetch("/guardar", { method: "POST", body: lineas });
  if (!r.ok) { $("ultima").innerHTML = `<span class="mal">no se guardó: ${await r.text()}</span>`; return; }
  const res = await r.json();
  ultima = res.archivo;
  $("descartar").disabled = false;
  if (res.toma) {
    const p = res.pellizco || {};
    const con = p.cierres ? `${p.cierres} cierre${p.cierres === 1 ? "" : "s"} de pellizco` : "sin pellizco";
    $("ultima").innerHTML = `Guardada <b>${res.archivo}</b> · ${res.cuadros} cuadros · ${(res.ms / 1000).toFixed(1)} s<br>` +
      `<span class="${p.cierres ? "bien" : "mal"}">${con}, ${Math.round(100 * (p.fraccion || 0))} % de los cuadros con al menos una mano pellizcando</span>`;
  } else {
    const salio = res.emitidos.length ? res.emitidos.join(", ") : "nada";
    $("ultima").innerHTML = `Guardada <b>${res.archivo}</b> · ${res.cuadros} cuadros · ${(res.ms / 1000).toFixed(1)} s<br>` +
      (res.acierto ? `<span class="bien">el clasificador la reconoce (salió: ${salio})</span>`
                   : `<span class="mal">el clasificador no la reconoce (salió: ${salio}); guárdala igual si el gesto estuvo bien: eso es lo que hay que medir</span>`);
  }
  actualizarCuenta();
  reiniciarVivo();
}

async function descartar() {
  if (!ultima) return;
  const r = await fetch("/descartar?archivo=" + encodeURIComponent(ultima), { method: "POST" });
  $("ultima").textContent = r.ok ? `Descartada ${ultima}.` : "no se pudo descartar: " + (await r.text());
  ultima = null;
  $("descartar").disabled = true;
  actualizarCuenta();
}

const dims = { ancho: 640, alto: 480 };

function resumenPellizco(pellizco) {
  if (!pellizco || !pellizco.length) return "sin manos";
  return pellizco.map((p) => `${p.derecha ? "D" : "I"} ${p.cerrado ? "cerrado" : "abierto"} ${Number(p.apertura).toFixed(2)}`).join(" · ");
}

async function main() {
  $("gesto").addEventListener("change", mostrarGuia);
  $("grabar").addEventListener("click", alternar);
  $("descartar").addEventListener("click", descartar);
  document.addEventListener("keydown", (e) => {
    if (e.code === "Space" && !(e.target instanceof HTMLInputElement) && !$("grabar").disabled) { e.preventDefault(); alternar(); }
  });
  mostrarGuia();
  actualizarCuenta();

  $("vivo").textContent = "cargando MediaPipe…";
  const fileset = await FilesetResolver.forVisionTasks("./vendor/wasm");
  const crear = (delegate) => HandLandmarker.createFromOptions(fileset, {
    baseOptions: { modelAssetPath: "./vendor/hand_landmarker.task", delegate }, runningMode: "VIDEO", numHands: 2 });
  let lm;
  try { lm = await crear("GPU"); } catch { lm = await crear("CPU"); }

  $("vivo").textContent = "pidiendo la cámara…";
  const st = await navigator.mediaDevices.getUserMedia({ video: { width: 640, height: 480, frameRate: { ideal: 30 } }, audio: false });
  const video = $("video");
  video.srcObject = st;
  await video.play();
  dims.ancho = video.videoWidth || 640; dims.alto = video.videoHeight || 480;
  const lienzo = $("puntos"), ctx = lienzo.getContext("2d");
  lienzo.width = dims.ancho; lienzo.height = dims.alto;
  $("grabar").disabled = false;

  let ultimoTs = -1, evento = "", eventoHasta = 0, vivo = { pose: "-", modo: "-", pellizco: [] };
  const procesar = (ahora) => {
    let ts = ahora; if (ts <= ultimoTs) ts = ultimoTs + 0.01; ultimoTs = ts;
    const r = lm.detectForVideo(video, ts);
    const manos = r.landmarks.map((pts, i) => {
      const h = (r.handedness || r.handednesses || [])[i];
      return { derecha: !!(h && h[0] && h[0].categoryName === "Right"), p: pts.map((p) => [+p.x.toFixed(5), +p.y.toFixed(5), +p.z.toFixed(5)]) };
    });
    ctx.clearRect(0, 0, lienzo.width, lienzo.height);
    ctx.fillStyle = "#7fa8ff";
    for (const m of manos) for (const [x, y] of m.p) { ctx.beginPath(); ctx.arc(x * lienzo.width, y * lienzo.height, 4, 0, 7); ctx.fill(); }

    const t = Math.round(ahora - (grabando ? toma.t0 : t0Vivo));
    if (grabando) toma.cuadros.push({ t: Math.max(0, t), manos });
    const cuerpo = JSON.stringify({ t, manos, ancho: dims.ancho, alto: dims.alto });
    cola = cola.then(async () => {
      const res = await (await fetch("/cuadro", { method: "POST", body: cuerpo })).json();
      vivo = res;
      if (res.gesto) { evento = res.gesto + (res.direccion ? " → " + res.direccion : ""); eventoHasta = performance.now() + 1500; }
    }).catch(() => {});
    const seg = grabando ? ` · grabando ${((ahora - toma.t0) / 1000).toFixed(1)} s` : "";
    $("vivo").innerHTML = `pose ${vivo.pose} · modo ${vivo.modo} · pellizco ${resumenPellizco(vivo.pellizco)}${seg} <span id="evento">${performance.now() < eventoHasta ? evento : ""}</span>`;
  };
  if ("requestVideoFrameCallback" in HTMLVideoElement.prototype) {
    const cb = (ahora) => { procesar(ahora); video.requestVideoFrameCallback(cb); };
    video.requestVideoFrameCallback(cb);
  } else {
    let tv = -1;
    const bucle = () => { if (video.currentTime !== tv) { tv = video.currentTime; procesar(performance.now()); } requestAnimationFrame(bucle); };
    requestAnimationFrame(bucle);
  }
}

main().catch((e) => { $("vivo").innerHTML = `<span class="mal">ERROR ${e && e.message || e}</span>`; });
