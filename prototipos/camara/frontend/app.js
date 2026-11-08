// Corre Hand Landmarker sobre la cámara (o una foto) y mide:
//  fps            resultados por segundo
//  inferencia     lo que tarda detectForVideo
//  cuadro→puntos  desde que el cuadro llega a la página (requestVideoFrameCallback) hasta tener los puntos
//  captura→puntos desde metadata.captureTime, si el navegador lo da
import { FilesetResolver, HandLandmarker } from "./vendor/vision_bundle.mjs";

const $ = (id) => document.getElementById(id);
const estado = (t) => { $("estado").textContent = t; };
const go = () => window.go.main.App;
const log = (s) => { try { go().Log(String(s)); } catch {} };

function resumen(xs) {
  if (!xs.length) return null;
  const s = [...xs].sort((a, b) => a - b);
  const q = (p) => s[Math.min(s.length - 1, Math.floor(p * s.length))];
  return { n: s.length, media: +(s.reduce((a, b) => a + b, 0) / s.length).toFixed(2),
           p50: +q(0.5).toFixed(2), p95: +q(0.95).toFixed(2), max: +s[s.length - 1].toFixed(2) };
}

async function esperarWails() {
  for (let i = 0; i < 200 && !(window.go && window.go.main); i++) await new Promise((r) => setTimeout(r, 25));
}

async function main() {
  await esperarWails();
  const cfg = await go().Config();
  const info = { ua: navigator.userAgent, modo: cfg.modo, rvfc: "requestVideoFrameCallback" in HTMLVideoElement.prototype,
                 mediaDevices: !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia) };
  const probarGL = (c, t) => { try { return !!c.getContext(t); } catch (e) { return "excepción " + e; } };
  info.webgl = { canvas_webgl: probarGL(document.createElement("canvas"), "webgl"),
                 canvas_webgl2: probarGL(document.createElement("canvas"), "webgl2"),
                 offscreen_webgl2: typeof OffscreenCanvas !== "undefined" ? probarGL(new OffscreenCanvas(8, 8), "webgl2") : "sin OffscreenCanvas" };
  info.apis = { MediaStreamTrackProcessor: typeof MediaStreamTrackProcessor, VideoFrame: typeof VideoFrame, ImageCapture: typeof ImageCapture };
  log("config " + JSON.stringify(cfg) + " info " + JSON.stringify(info));

  estado("cargando MediaPipe…");
  const t0 = performance.now();
  const fileset = await FilesetResolver.forVisionTasks("./vendor/wasm");
  let lm, delegado = cfg.delegado;
  const crear = (d) => HandLandmarker.createFromOptions(fileset, {
    baseOptions: { modelAssetPath: "./vendor/hand_landmarker.task", delegate: d },
    runningMode: "VIDEO", numHands: 1,
    // WebKitGTK no da WebGL2 en OffscreenCanvas (lo que MediaPipe usa por defecto): se le pasa un canvas normal.
    canvas: info.webgl.offscreen_webgl2 === true ? undefined : document.createElement("canvas") });
  try { lm = await crear(delegado); }
  catch (e) { log("delegado " + delegado + " falló: " + e); delegado = "CPU"; lm = await crear("CPU"); }
  info.delegado_usado = delegado;
  info.carga_ms = Math.round(performance.now() - t0);

  // Fuente de cuadros
  const video = $("video"), lienzo = $("lienzo"), ctx2d = lienzo.getContext("2d");
  let foto = null;
  if (cfg.modo !== "camara" && cfg.modo !== "proceso") { foto = new Image(); foto.src = "./vendor/mano.jpg"; await foto.decode(); }
  const conMano = cfg.modo.endsWith("+mano"), porProceso = cfg.modo.startsWith("proceso");
  if (cfg.modo.startsWith("camara")) {
    if (!info.mediaDevices) throw new Error("navigator.mediaDevices.getUserMedia no existe en esta webview");
    log("dispositivos " + JSON.stringify((await navigator.mediaDevices.enumerateDevices()).map((d) => [d.kind, d.label])));
    log("pidiendo la cámara…");
    const st = await navigator.mediaDevices.getUserMedia({ video: { width: 640, height: 480, frameRate: { ideal: 30 } }, audio: false });
    log("cámara concedida: " + st.getVideoTracks()[0].label);
    video.srcObject = st;
    const vigia = setInterval(() => log(`video readyState=${video.readyState} t=${video.currentTime.toFixed(2)} ${video.videoWidth}x${video.videoHeight} paused=${video.paused}`), 1000);
    await Promise.race([video.play(), new Promise((_, no) => setTimeout(() => no(new Error("video.play() no arrancó en 10 s")), 10000))])
      .finally(() => clearInterval(vigia));
    log("video corriendo");
    const s = st.getVideoTracks()[0].getSettings();
    info.camara = { ancho: s.width, alto: s.height, fps_pedidos: s.frameRate, etiqueta: st.getVideoTracks()[0].label };
  }

  const inf = [], c2p = [], cap2p = [];
  let resultados = 0, cuadrosMano = 0, cuadrosCamara = 0, midiendo = false, tMedida = 0, ultimoTs = -1;
  const puntos = $("puntos").getContext("2d");

  function pintar(r) {
    puntos.clearRect(0, 0, 640, 480);
    if (!r.landmarks.length) return;
    puntos.fillStyle = "#7fa8ff";
    for (const p of r.landmarks[0]) { puntos.beginPath(); puntos.arc(p.x * 640, p.y * 480, 4, 0, 7); puntos.fill(); }
  }

  let errores = 0, errorBucle = null;
  function procesar(fuente, tCuadro, tCaptura, tGo) {
    try { procesar1(fuente, tCuadro, tCaptura ?? tGo ?? null); }
    catch (e) { if (errores++ < 3) log("error en el cuadro: " + (e && e.stack || e)); }
  }
  function procesar1(fuente, tCuadro, tCaptura) {
    let ts = performance.now(); if (ts <= ultimoTs) ts = ultimoTs + 0.01; ultimoTs = ts;
    const a = performance.now();
    const r = lm.detectForVideo(fuente, ts);
    const b = performance.now();
    pintar(r);
    if (!midiendo) return;
    resultados++; if (r.landmarks.length) cuadrosMano++;
    inf.push(b - a); c2p.push(b - tCuadro);
    if (tCaptura != null) cap2p.push(b - tCaptura);
  }

  function dibujarCompuesto(fondo) {
    ctx2d.clearRect(0, 0, 640, 480);
    if (fondo) ctx2d.drawImage(fondo, 0, 0, 640, 480);
    else { ctx2d.fillStyle = "#808080"; ctx2d.fillRect(0, 0, 640, 480); }
    const h = 360, w = foto.width * h / foto.height;   // la mano ocupa buena parte del cuadro
    ctx2d.drawImage(foto, (640 - w) / 2, 60, w, h);
  }

  if (porProceso) {
    // Plan B: los cuadros los captura ffmpeg (otro proceso) y llegan como JPEG por el AssetServer.
    // La latencia se mide en reloj de pared (Date.now) desde que Go recibió el cuadro.
    let seq = 0, primero = true;
    const bucle = async () => {
      while (true) {
        const r = await fetch(`/camara/cuadro?despues=${seq}`, { cache: "no-store" });
        if (!r.ok) throw new Error("captura por proceso: " + (await r.text()));
        const nuevo = +r.headers.get("X-Seq"), tGo = +r.headers.get("X-T");
        if (midiendo && seq) cuadrosCamara += nuevo - seq;   // cuadros que dio la cámara, también los saltados
        seq = nuevo;
        const bmp = await createImageBitmap(await r.blob());
        if (primero) { primero = false; info.camara = { ancho: bmp.width, alto: bmp.height, por: "ffmpeg v4l2", tiene_X_T: !!tGo }; }
        const tPagina = performance.now();
        const fuente = conMano ? (dibujarCompuesto(bmp), lienzo) : bmp;
        procesar(fuente, tPagina, null, tGo ? performance.now() - (Date.now() - tGo) : null);
        bmp.close();
      }
    };
    bucle().catch((e) => { log("ERROR " + e); errorBucle = String(e); });
  } else if (cfg.modo === "imagen") {
    const bucle = () => { const t = performance.now(); dibujarCompuesto(null); procesar(lienzo, t, null); requestAnimationFrame(bucle); };
    requestAnimationFrame(bucle);
  } else if (info.rvfc) {
    const cb = (now, md) => {
      if (midiendo) cuadrosCamara++;
      // captureTime está en el mismo reloj que performance.now() cuando existe
      const tCap = md && typeof md.captureTime === "number" ? md.captureTime : null;
      if (conMano) { dibujarCompuesto(video); procesar(lienzo, now, tCap); }
      else procesar(video, now, tCap);
      video.requestVideoFrameCallback(cb);
    };
    video.requestVideoFrameCallback(cb);
  } else {
    // Sin rVFC: rAF y solo cuando cambia currentTime
    let tv = -1;
    const bucle = () => {
      if (video.currentTime !== tv) { tv = video.currentTime; const t = performance.now(); if (midiendo) cuadrosCamara++;
        if (conMano) { dibujarCompuesto(video); procesar(lienzo, t, null); } else procesar(video, t, null); }
      requestAnimationFrame(bucle);
    };
    requestAnimationFrame(bucle);
  }

  estado(`calentando ${cfg.calentar} s…`);
  await new Promise((r) => setTimeout(r, cfg.calentar * 1000));
  await go().MedirInicio();
  midiendo = true; tMedida = performance.now();
  const tick = setInterval(() => {
    const s = (performance.now() - tMedida) / 1000;
    estado(`midiendo ${s.toFixed(0)}/${cfg.segundos} s · ${(resultados / s).toFixed(1)} fps · mano en ${cuadrosMano}/${resultados}\n` +
           `delegado ${delegado} · inferencia p50 ${resumen(inf)?.p50 ?? "-"} ms`);
  }, 500);
  await new Promise((r) => setTimeout(r, cfg.segundos * 1000));
  midiendo = false; clearInterval(tick);
  const seg = (performance.now() - tMedida) / 1000;
  const web = { ...info, segundos: +seg.toFixed(2), resultados, fps: +(resultados / seg).toFixed(2),
    error_bucle: errorBucle,
    fps_camara: cfg.modo === "imagen" ? null : +(cuadrosCamara / seg).toFixed(2),
    cuadros_con_mano: cuadrosMano, errores_cuadro: errores, inferencia_ms: resumen(inf), cuadro_a_puntos_ms: resumen(c2p), captura_a_puntos_ms: resumen(cap2p) };
  estado(JSON.stringify(web, null, 1));
  await go().Reportar(web);
}

main().catch(async (e) => {
  const msg = (e && (e.name + ": " + e.message)) || String(e);
  estado("ERROR " + msg); log("ERROR " + msg);
  try { await go().Reportar({ error: msg, ua: navigator.userAgent }); } catch {}
});
