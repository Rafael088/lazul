// Graba tomas de gestos: cámara → MediaPipe Hand Landmarker → cuadros en el
// formato de gestures/grabacion.go. Los puntos se guardan tal como los da
// MediaPipe (imagen sin espejar); el video solo se muestra espejado.
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
};

let grabando = false, toma = null, ultima = null, t0Vivo = performance.now(), cola = Promise.resolve();

function seleccion() {
  const [gesto, negativo] = $("gesto").value.split(":");
  return { gesto, negativo: negativo || "", luz: $("luz").value, nota: $("nota").value.trim() };
}

async function reiniciarVivo() {
  t0Vivo = performance.now();
  cola = cola.then(() => fetch("/reiniciar?gesto=" + seleccion().gesto, { method: "POST" })).catch(() => {});
  await cola;
}

async function actualizarCuenta() {
  try {
    const n = await (await fetch("/cuenta")).json();
    const filas = Object.keys(n).sort().map((k) => `${k.padEnd(22)} ${String(n[k]).padStart(3)} / 20`);
    $("cuenta").textContent = filas.length ? "En la carpeta:\n" + filas.join("\n") : "Aún no hay tomas.";
  } catch {}
}

function mostrarGuia() { $("guia").textContent = GUIAS[$("gesto").value]; reiniciarVivo(); }

async function alternar() {
  if (!grabando) {
    const s = seleccion();
    toma = { cabecera: { gesto: s.gesto, luz: s.luz, ancho: dims.ancho, alto: dims.alto }, cuadros: [], t0: performance.now() };
    if (s.negativo) toma.cabecera.negativo = s.negativo;
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
  const salio = res.emitidos.length ? res.emitidos.join(", ") : "nada";
  $("ultima").innerHTML = `Guardada <b>${res.archivo}</b> · ${res.cuadros} cuadros · ${(res.ms / 1000).toFixed(1)} s<br>` +
    (res.acierto ? `<span class="bien">el clasificador la reconoce (salió: ${salio})</span>`
                 : `<span class="mal">el clasificador no la reconoce (salió: ${salio}); guárdala igual si el gesto estuvo bien: eso es lo que hay que medir</span>`);
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
    baseOptions: { modelAssetPath: "./vendor/hand_landmarker.task", delegate }, runningMode: "VIDEO", numHands: 1 });
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

  let ultimoTs = -1, evento = "", eventoHasta = 0, vivo = { pose: "-", modo: "-" };
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
    $("vivo").innerHTML = `pose ${vivo.pose} · modo ${vivo.modo}${seg} <span id="evento">${performance.now() < eventoHasta ? evento : ""}</span>`;
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
