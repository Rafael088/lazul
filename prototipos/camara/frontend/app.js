// Corre Hand Landmarker sobre la cámara (o una foto) y mide:
//  fps            resultados por segundo
//  inferencia     lo que tarda detectForVideo
//  cuadro→puntos  desde que el cuadro llega a la página (requestVideoFrameCallback) hasta tener los puntos
//  captura→puntos desde metadata.captureTime, si el navegador lo da
import { FilesetResolver, HandLandmarker, PoseLandmarker } from "./vendor/vision_bundle.mjs";

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
  let lm, pm = null, delegado = cfg.delegado;
	const crear = async (d) => {
		const canvas = () => info.webgl.offscreen_webgl2 === true ? undefined : document.createElement("canvas");
		const mano = await HandLandmarker.createFromOptions(fileset, {
			baseOptions: { modelAssetPath: "./vendor/hand_landmarker.task", delegate: d },
			runningMode: "VIDEO", numHands: cfg.manos, canvas: canvas() });
		if (!cfg.pose_cada) return [mano, null];
		try {
			const pose = await PoseLandmarker.createFromOptions(fileset, {
				baseOptions: { modelAssetPath: "./vendor/pose_landmarker_lite.task", delegate: d },
				runningMode: "VIDEO", numPoses: 1, minPoseDetectionConfidence: 0.5,
				minTrackingConfidence: 0.5, canvas: canvas() });
			return [mano, pose];
		} catch (e) { if (mano.close) mano.close(); throw e; }
	};
	try { [lm, pm] = await crear(delegado); }
	catch (e) { log("delegado " + delegado + " falló: " + e); delegado = "CPU"; [lm, pm] = await crear("CPU"); }
  info.delegado_usado = delegado;
  info.carga_ms = Math.round(performance.now() - t0);

  // Fuente de cuadros
  const video = $("video"), lienzo = $("lienzo"), ctx2d = lienzo.getContext("2d");
  const copia2d = $("copia").getContext("2d");
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

  const inf = [], infPose = [], c2p = [], cap2p = [], puente = [];
	let resultados = 0, cuadrosMano = 0, cuadrosCamara = 0, poses = 0, midiendo = false, tMedida = 0, ultimoTs = -1;
	let ultimaPose = null, cuadrosSinPose = 0, manoMediaMs = 0;
  const puntos = $("puntos").getContext("2d");

  function pintar(r) {
    puntos.clearRect(0, 0, 640, 480);
    if (!r.landmarks.length) return;
    puntos.fillStyle = "#7fa8ff";
    for (const p of r.landmarks[0]) { puntos.beginPath(); puntos.arc(p.x * 640, p.y * 480, 4, 0, 7); puntos.fill(); }
  }

  const usa = (parte) => {
    const v = cfg.variante;
    if (v === parte) return true;
    if (v === "antes") return ["respuesta", "indicador", "aura", "copia"].includes(parte);
    if (v === "despues") return ["envio", "indicador", "aura"].includes(parte);
    return false;
  };
  let ultimoAura = 0, ultimoSondeo = 0, vistaSondeada = null;
	const cuadroJSON = (r, ts, cuerpo) => ({ t: ts, manos: r.landmarks.map((ps, i) => ({
    derecha: !!(r.handednesses?.[i]?.[0]?.categoryName === "Right"),
    p: ps.map((p) => [p.x, p.y, p.z]),
	})), cuerpo });
  const pintarIndicador = (v) => {
    const el = $("indicador"); el.hidden = false;
    el.textContent = v?.indicador?.activo ? "mano en el panel" : "inactivo";
  };
  const pintarAura = (r, ahora) => {
    if (ahora - ultimoAura < 40) return;
    ultimoAura = ahora;
    const el = $("aura"), p = r.landmarks?.[0]?.[9];
    el.hidden = !p;
    if (p) el.style.transform = `translate3d(${(p.x - .5) * 400}px, ${(p.y - .5) * 240}px, 0)`;
  };
	async function partes(r, ts, cuerpo) {
    if (cfg.variante === "base" || cfg.variante === "copia" || cfg.variante === "aura") return;
		const ini = performance.now(), c = cuadroJSON(r, ts, cuerpo);
    if (usa("respuesta") || cfg.variante === "indicador") {
      const v = await go().Responder(c);
      if (usa("indicador")) pintarIndicador(v);
    } else {
      await go().Enviar(c);
      // La versión optimizada obtiene textos e indicador con el sondeo ya
      // existente, no como respuesta de cada cuadro.
      if (cfg.variante === "despues" && performance.now() - ultimoSondeo >= 500) {
        ultimoSondeo = performance.now(); vistaSondeada = await go().Estado();
      }
      if (usa("indicador") && vistaSondeada) pintarIndicador(vistaSondeada);
    }
    if (midiendo) puente.push(performance.now() - ini);
  }

  let errores = 0, errorBucle = null;
  async function procesar(fuente, tCuadro, tCaptura, tGo) {
    try { await procesar1(fuente, tCuadro, tCaptura ?? tGo ?? null); }
    catch (e) { if (errores++ < 3) log("error en el cuadro: " + (e && e.stack || e)); }
  }
  async function procesar1(fuente, tCuadro, tCaptura) {
    let ts = performance.now(); if (ts <= ultimoTs) ts = ultimoTs + 0.01; ultimoTs = ts;
    // Copia aislada entre superficies, equivalente al JPEG que la app llevaba
    // al lienzo aun cuando la vista de cámara estaba cerrada.
    if (usa("copia")) copia2d.drawImage(fuente, 0, 0, 640, 480);
    const a = performance.now();
    const r = lm.detectForVideo(fuente, ts);
    const b = performance.now();
		const manoMs = b - a;
		manoMediaMs = manoMediaMs ? manoMediaMs * .8 + manoMs * .2 : manoMs;
		let poseMs = 0, cadaPose = cfg.pose_cada;
		if (pm && cfg.pose_adaptativa && manoMediaMs >= cfg.umbral_mano_ms) cadaPose *= 2;
		if (pm && (!ultimaPose || ++cuadrosSinPose >= cadaPose)) {
			const p0 = performance.now();
			ultimaPose = pm.detectForVideo(fuente, ts);
			poseMs = performance.now() - p0;
			cuadrosSinPose = 0;
			if (midiendo) { poses++; infPose.push(poseMs); }
		}
		const puntosPose = (ultimaPose?.landmarks || [])[0];
		const cuerpo = pm
			? (puntosPose ? { p: puntosPose.map((p) => [p.x, p.y, p.z, p.visibility ?? 1]) } : null)
			: { p: Array.from({ length: 33 }, (_, i) => [.3 + i / 100, .2 + i / 200, 0, 1]) };
    if (usa("aura")) pintarAura(r, b);
		await partes(r, ts, cuerpo);
    if (!midiendo) return;
    resultados++; if (r.landmarks.length) cuadrosMano++;
		inf.push(manoMs); c2p.push(b - tCuadro);
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
        await procesar(fuente, tPagina, null, tGo ? performance.now() - (Date.now() - tGo) : null);
        bmp.close();
      }
    };
    bucle().catch((e) => { log("ERROR " + e); errorBucle = String(e); });
  } else if (cfg.modo === "imagen") {
    const bucle = async () => { const t = performance.now(); dibujarCompuesto(null); await procesar(lienzo, t, null); requestAnimationFrame(bucle); };
    requestAnimationFrame(bucle);
  } else if (info.rvfc) {
    const cb = async (now, md) => {
      if (midiendo) cuadrosCamara++;
      // captureTime está en el mismo reloj que performance.now() cuando existe
      const tCap = md && typeof md.captureTime === "number" ? md.captureTime : null;
      if (conMano) { dibujarCompuesto(video); await procesar(lienzo, now, tCap); }
      else await procesar(video, now, tCap);
      video.requestVideoFrameCallback(cb);
    };
    video.requestVideoFrameCallback(cb);
  } else {
    // Sin rVFC: rAF y solo cuando cambia currentTime
    let tv = -1;
    const bucle = async () => {
      if (video.currentTime !== tv) { tv = video.currentTime; const t = performance.now(); if (midiendo) cuadrosCamara++;
        if (conMano) { dibujarCompuesto(video); await procesar(lienzo, t, null); } else await procesar(video, t, null); }
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
		cuadros_con_mano: cuadrosMano, poses, pose_cada_real: poses ? +(resultados / poses).toFixed(2) : null,
		errores_cuadro: errores, inferencia_ms: resumen(inf), pose_ms: resumen(infPose), puente_go_ms: resumen(puente),
		cuadro_a_puntos_ms: resumen(c2p), captura_a_puntos_ms: resumen(cap2p) };
  estado(JSON.stringify(web, null, 1));
  await go().Reportar(web);
}

main().catch(async (e) => {
  const msg = (e && (e.name + ": " + e.message)) || String(e);
  estado("ERROR " + msg); log("ERROR " + msg);
  try { await go().Reportar({ error: msg, ua: navigator.userAgent }); } catch {}
});
