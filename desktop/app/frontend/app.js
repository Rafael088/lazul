// Ventana de Lazul: la cámara y MediaPipe aquí; todo lo demás (clasificar,
// emparejar, mandar) en Go. Los cuadros no salen de esta página: a Go solo
// le llegan los 21 puntos de la mano.
const $ = (id) => document.getElementById(id);
const go = () => window.go.main.App;
const log = (s) => { try { go().Log(String(s)); } catch {} };

const CONEXIONES = [[0,1],[1,2],[2,3],[3,4],[0,5],[5,6],[6,7],[7,8],[5,9],[9,10],[10,11],[11,12],[9,13],[13,14],[14,15],[15,16],[13,17],[0,17],[17,18],[18,19],[19,20]];

async function esperarWails() {
  for (let i = 0; i < 400 && !(window.go && window.go.main); i++) await new Promise((r) => setTimeout(r, 25));
}

// ---------- cámara + MediaPipe ----------
let cfg, lm = null, encendida = false, generacion = 0, t0 = performance.now(), ultimoTs = -1, cola = Promise.resolve(), pendientes = 0;
const puntos = () => $("puntos").getContext("2d");

function pintar(manos, w, h) {
  const c = puntos();
  c.clearRect(0, 0, 640, 480);
  for (const m of manos) {
    c.strokeStyle = "#7fa8ff"; c.lineWidth = 2;
    for (const [a, b] of CONEXIONES) { c.beginPath(); c.moveTo(m.p[a][0] * 640, m.p[a][1] * 480); c.lineTo(m.p[b][0] * 640, m.p[b][1] * 480); c.stroke(); }
    c.fillStyle = "#ffcc4d";
    for (const [x, y] of m.p) { c.beginPath(); c.arc(x * 640, y * 480, 3.5, 0, 7); c.fill(); }
  }
}

function mostrarVista(v) {
  if (!v) return;
  $("pose").textContent = v.pose || "-";
  $("modo").textContent = v.modo || "-";
  $("modo").className = "grande modo-" + (v.modo || "reposo");
  $("ultimo").textContent = v.ultimo || "-";
  $("ultimohora").textContent = v.ultimo_hora ? "a las " + v.ultimo_hora : "";
}

function procesar(fuente, w, h) {
  let ts = performance.now(); if (ts <= ultimoTs) ts = ultimoTs + 0.01; ultimoTs = ts;
  const r = lm.detectForVideo(fuente, ts);
  const manos = r.landmarks.map((pts, i) => {
    const hd = (r.handedness || r.handednesses || [])[i];
    return { derecha: !!(hd && hd[0] && hd[0].categoryName === "Right"), p: pts.map((p) => [+p.x.toFixed(5), +p.y.toFixed(5), +p.z.toFixed(5)]) };
  });
  pintar(manos, w, h);
  const cuadro = { t: Math.round(ts - t0), manos, ancho: w, alto: h };
  // En orden y sin amontonar: si Go se atrasa, se salta el cuadro.
  if (pendientes > 4) return;
  pendientes++;
  cola = cola.then(() => go().Cuadro(cuadro)).then(mostrarVista).catch((e) => log("cuadro: " + e)).finally(() => pendientes--);
}

async function cargarMediaPipe() {
  if (lm) return;
  $("sincamara").textContent = "cargando MediaPipe…";
  const { FilesetResolver, HandLandmarker } = await import("./vendor/vision_bundle.mjs");
  const fileset = await FilesetResolver.forVisionTasks("./vendor/wasm");
  let offscreen = false;
  try { offscreen = typeof OffscreenCanvas !== "undefined" && !!new OffscreenCanvas(8, 8).getContext("webgl2"); } catch {}
  // WebKitGTK no da WebGL2 en OffscreenCanvas (lo que MediaPipe usa por defecto): se le pasa un canvas normal.
  const crear = (delegate) => HandLandmarker.createFromOptions(fileset, {
    baseOptions: { modelAssetPath: "./vendor/hand_landmarker.task", delegate }, runningMode: "VIDEO", numHands: 1,
    canvas: offscreen ? undefined : document.createElement("canvas") });
  let delegado = "GPU";
  try { lm = await crear("GPU"); } catch (e) { log("GPU falló, uso CPU: " + e); delegado = "CPU"; lm = await crear("CPU"); }
  log("MediaPipe listo (" + delegado + ")");
}

async function encender() {
  const gen = ++generacion;
  encendida = true;
  $("camara").textContent = "Apagar cámara";
  try {
    await cargarMediaPipe();
    const err = await go().CamaraEncender();
    if (err) throw new Error(err);
    $("sincamara").textContent = "abriendo la cámara…";
    if (cfg.camara === "proceso") await bucleProceso(gen);
    else await bucleWebview(gen);
  } catch (e) {
    const msg = (e && e.message) || String(e);
    $("sincamara").textContent = "Cámara: " + msg;
    $("sincamara").style.display = "flex";
    go().CamaraError(msg);
  }
}

async function apagar() {
  generacion++;
  encendida = false;
  $("camara").textContent = "Encender cámara";
  const v = $("video");
  if (v.srcObject) { v.srcObject.getTracks().forEach((t) => t.stop()); v.srcObject = null; }
  $("imagen").getContext("2d").clearRect(0, 0, 640, 480);
  puntos().clearRect(0, 0, 640, 480);
  $("sincamara").textContent = "cámara apagada";
  $("sincamara").style.display = "flex";
  await go().CamaraApagar();
}

// Linux: los cuadros los da ffmpeg (otro proceso) como JPEG por el AssetServer.
async function bucleProceso(gen) {
  let seq = 0;
  const lienzo = $("imagen"), ctx = lienzo.getContext("2d");
  $("video").style.display = "none";
  while (gen === generacion) {
    const r = await fetch(`/camara/cuadro?despues=${seq}`, { cache: "no-store" });
    if (gen !== generacion) return;
    if (!r.ok) throw new Error(await r.text());
    seq = +r.headers.get("X-Seq");
    const bmp = await createImageBitmap(await r.blob());
    $("sincamara").style.display = "none";
    ctx.drawImage(bmp, 0, 0, 640, 480);
    procesar(bmp, bmp.width, bmp.height);
    bmp.close();
  }
}

// Windows: la webview abre la cámara.
async function bucleWebview(gen) {
  const video = $("video");
  $("imagen").style.display = "none";
  const st = await navigator.mediaDevices.getUserMedia({ video: { width: 640, height: 480, frameRate: { ideal: 30 } }, audio: false });
  if (gen !== generacion) { st.getTracks().forEach((t) => t.stop()); return; }
  video.srcObject = st;
  await video.play();
  $("sincamara").style.display = "none";
  const cb = () => {
    if (gen !== generacion) return;
    procesar(video, video.videoWidth || 640, video.videoHeight || 480);
    video.requestVideoFrameCallback(cb);
  };
  if ("requestVideoFrameCallback" in HTMLVideoElement.prototype) video.requestVideoFrameCallback(cb);
  else {
    let tv = -1;
    const bucle = () => { if (gen !== generacion) return; if (video.currentTime !== tv) { tv = video.currentTime; procesar(video, video.videoWidth || 640, video.videoHeight || 480); } requestAnimationFrame(bucle); };
    requestAnimationFrame(bucle);
  }
}

// ---------- el resto de la ventana ----------
function texto(id, t) { $(id).textContent = t; }

function pintarEstado(e) {
  const r = e.red;
  texto("nombre", r.nombre); texto("huella", r.huella);
  texto("dirs", (r.direcciones || []).join("  ·  ") || (r.escuchando ? "(sin red local)" : "-"));
  texto("errred", r.error || "");
  texto("codigo", r.codigo || ""); texto("vence", r.codigo ? "vence a las " + r.codigo_vence : "");
  texto("destino", r.destino || "nadie (empareja uno)");
  $("conexion").innerHTML = r.destino ? (r.conectado ? '<span class="bien">conectado</span>' : '<span class="mal">sin conexión</span> ' + (r.error_destino || "")) : "";
  texto("accion", r.accion ? `${r.accion_hora} · ${r.accion}` : "-");
  texto("recibido", r.recibido ? `${r.recibido_hora} · ${r.recibido}` : "-");
  const filas = (r.equipos || []).map((q) => {
    let extra = "";
    if (q.rol === "le mando") extra = q.elegido ? (q.conectado ? '<span class="bien">elegido · conectado</span>' : "elegido") : `<button class="sec" data-elegir="${q.huella}">Mandar aquí</button>`;
    if (q.encontrado) extra = `<button class="sec" data-usar="${q.dir}">Usar esta dirección</button>`;
    return `<tr><td><b>${esc(q.nombre)}</b></td><td class="tenue">${q.rol}</td><td class="mono">${q.huella}</td><td class="mono tenue">${q.dir || ""}</td><td>${extra}</td></tr>`;
  });
  $("equipos").innerHTML = filas.join("") || '<tr><td class="tenue">ninguno todavía</td></tr>';
  const p = e.permiso;
  $("permiso").className = p ? "ver" : "";
  if (p) { texto("permnombre", "«" + p.nombre + "»"); texto("permhuella", p.huella); }
  $("prueba").textContent = e.prueba ? "Terminar prueba de trabajo normal" : "Empezar prueba de trabajo normal";
  $("prueba").dataset.on = e.prueba ? "1" : "";
  if (cfg.reproducir) mostrarVista(e.vista);
  const fps = e.fps ? `${e.fps.toFixed(1)} cuadros/s · mano en ${(100 * e.con_mano).toFixed(0)} %` : "";
  texto("fps", (e.camara ? "Cámara: " + e.camara + " " : "") + fps);
}

function esc(s) { return String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]); }

async function main() {
  await esperarWails();
  cfg = await go().Config();
  texto("registro", cfg.registro);
  $("abrircodigo").onclick = () => go().AbrirCodigo();
  $("emparejar").onclick = async () => {
    $("emparejar").disabled = true;
    texto("emparejarres", "esperando a que la persona del otro equipo permita…");
    const err = await go().Emparejar($("dir").value, $("cod").value);
    $("emparejar").disabled = false;
    $("emparejarres").className = err ? "mal" : "bien";
    texto("emparejarres", err ? "No se pudo: " + err : "Emparejado. Los gestos de esta cámara van a ese equipo.");
    if (!err) $("cod").value = "";
  };
  $("buscar").onclick = async () => { texto("emparejarres", "buscando…"); texto("emparejarres", await go().Buscar()); };
  $("equipos").onclick = async (ev) => {
    const b = ev.target.closest("button"); if (!b) return;
    if (b.dataset.elegir) await go().Elegir(b.dataset.elegir);
    if (b.dataset.usar) $("dir").value = b.dataset.usar;
  };
  $("permitir").onclick = () => go().ResponderPermiso(true);
  $("rechazar").onclick = () => go().ResponderPermiso(false);
  $("prueba").onclick = () => go().Prueba(!$("prueba").dataset.on);
  $("falsa").onclick = async () => texto("resumentexto", await go().Marcar(true));
  $("buscada").onclick = async () => texto("resumentexto", await go().Marcar(false));
  $("resumen").onclick = async () => texto("resumentexto", await go().Resumen());
  $("camara").onclick = () => (encendida ? apagar() : encender());
  const tick = async () => { try { pintarEstado(await go().Estado()); } catch (e) { log("estado: " + e); } };
  setInterval(tick, 500); tick();

  if (cfg.reproducir) { texto("sincamara", "reproduciendo grabaciones (sin cámara)"); $("camara").disabled = true; return; }
  if (cfg.camara === "no") { texto("sincamara", "sin cámara (-camara no)"); $("camara").disabled = true; return; }
  encender();
}

main().catch((e) => { const m = (e && e.message) || String(e); texto("sincamara", "ERROR " + m); log("ERROR " + m); });
