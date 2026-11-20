// Indicador de Lazul: la cámara y MediaPipe aquí; todo lo demás (filtrar,
// clasificar, emparejar, mandar) queda en Go. Los cuadros no salen de esta
// página: a Go llegan puntos de manos y Pose. Por defecto solo se ve el indicador;
// la vista de cámara se abre para acomodarse y se cierra al terminar.
const $ = (id) => document.getElementById(id);
const hayGo = () => !!(window.go && window.go.main);
const go = () => window.go.main.App;
const log = (s) => { try { if (hayGo()) go().Log(String(s)); } catch {} };

const CONEXIONES = [[0,1],[1,2],[2,3],[3,4],[0,5],[5,6],[6,7],[7,8],[5,9],[9,10],[10,11],[11,12],[9,13],[13,14],[14,15],[15,16],[13,17],[0,17],[17,18],[18,19],[19,20]];

async function esperarWails() {
  for (let i = 0; i < 200 && !hayGo(); i++) await new Promise((r) => setTimeout(r, 25));
}

// ---------- indicador pequeño ----------
let vistaCamara = false;
let modoEntradaPanel = "";
let panelUltimo = {};
let delegadoMP = "";

const FLECHAS = { arriba: "↑", abajo: "↓", izquierda: "←", derecha: "→", abrir: "+", cerrar: "−" };

function etiquetaGesto(ind) {
  if (!ind || !ind.gesto) return "";
  const f = FLECHAS[ind.direccion] || "";
  if (ind.gesto === "scroll") return "scroll " + f;
  if (ind.gesto === "zoom") return "zoom " + f;
  if (ind.gesto === "barrido") return f || "barrido";
  return ind.gesto;
}

function mostrarIndicador(ind) {
  const pill = $("indicador");
  if (!pill) return;
  // La L es un interruptor: el indicador dice siempre encendido o apagado.
  // Con el panel calibrado, encendido añade lo que hace la mano.
  const p = panelUltimo;
  const sufijo = (modoEntradaPanel === "respaldo" ? " · respaldo" : "") + (delegadoMP === "CPU" ? " · CPU lenta" : "");
  if (p.calibrado && !p.calibrando) {
    const encendido = !!p.encendido;
    const tocando = encendido && p.fase === "agarrando";
    pill.classList.toggle("activo", encendido);
    let t = "apagado";
    if (encendido) t = tocando ? (p.manos_armadas === 2 ? "encendido · 2 dedos" : "encendido · tocando") : (p.dentro ? "encendido · mano en el panel" : "encendido");
    $("indicador-estado").textContent = t + sufijo;
    $("indicador-gesto").textContent = "";
    return;
  }
  const activo = !!(ind && ind.activo);
  pill.classList.toggle("activo", activo && !p.calibrando);
  $("indicador-estado").textContent = (p.calibrando ? "calibrando" : (activo ? "encendido" : "apagado")) + sufijo;
  // El gesto ya no destella aquí: el aura (halo sobre la pantalla) es la
  // señal visual. Queda la etiqueta como recordatorio en la píldora.
  $("indicador-gesto").textContent = ind && ind.destello ? etiquetaGesto(ind) : "";
}

async function alternarAjuste(on = !vistaCamara) {
  if (on === vistaCamara) return;
  vistaCamara = on;
  document.body.classList.toggle("camara", vistaCamara);
  $("ajuste").setAttribute("aria-hidden", vistaCamara ? "false" : "true");
  if (hayGo()) {
    try { await go().VistaCamara(vistaCamara); } catch (e) { log("vista de cámara: " + e); }
  }
  if (vistaCamara) $("escena").scrollIntoView({ block: "nearest" });
}

// ---------- cámara + MediaPipe ----------
let cfg, lm = null, pm = null, encendida = false, generacion = 0, t0 = performance.now(), ultimoTs = -1, cola = Promise.resolve(), pendientes = 0;
const puntos = () => $("puntos").getContext("2d");
const CADA_POSE = 3;
let ultimaPose = null, cuadrosSinPose = 0;

function pintar(manos, cuerpo) {
  const c = puntos();
  c.clearRect(0, 0, 640, 480);
	if (cuerpo && cuerpo.p.length === 33) {
		c.strokeStyle = "#4fd18b"; c.lineWidth = 3;
		const torso = [11, 12, 24, 23, 11];
		c.beginPath();
		torso.forEach((i, n) => { const p = cuerpo.p[i]; if (n) c.lineTo(p[0] * 640, p[1] * 480); else c.moveTo(p[0] * 640, p[1] * 480); });
		c.stroke();
	}
  for (const m of manos) {
    c.strokeStyle = "#7fa8ff"; c.lineWidth = 2;
    for (const [a, b] of CONEXIONES) { c.beginPath(); c.moveTo(m.p[a][0] * 640, m.p[a][1] * 480); c.lineTo(m.p[b][0] * 640, m.p[b][1] * 480); c.stroke(); }
    c.fillStyle = "#ffcc4d";
    for (const [x, y] of m.p) { c.beginPath(); c.arc(x * 640, y * 480, 3.5, 0, 7); c.fill(); }
  }
}

// Panel sobre la cámara: punteado = como se calibró; trazo fino = banda
// exterior de permanencia; sólido = panel a la escala de cada mano.
const MOTIVOS = { dentro: "dentro", fuera_borde: "fuera del borde", cerca: "muy cerca", lejos: "muy lejos" };
function pintarPanel(pc) {
  if (!pc) return;
  const c = puntos(), W = 640, H = 480;
  const [x0, y0, x1, y1] = pc.calibrado;
  c.save();
  c.setLineDash([8, 6]); c.lineWidth = 2; c.strokeStyle = "rgba(232,236,255,.75)";
  c.strokeRect(Math.min(x0, x1) * W, Math.min(y0, y1) * H, Math.abs(x1 - x0) * W, Math.abs(y1 - y0) * H);
  c.setLineDash([]);
  for (const m of pc.manos || []) {
    const color = m.Dentro ? "#4fd18b" : "#ff9f43";
		c.strokeStyle = "rgba(255,204,77,.8)"; c.lineWidth = 2; c.setLineDash([5, 5]);
		c.strokeRect(Math.min(m.BX0, m.BX1) * W, Math.min(m.BY0, m.BY1) * H, Math.abs(m.BX1 - m.BX0) * W, Math.abs(m.BY1 - m.BY0) * H);
		c.setLineDash([]);
    c.strokeStyle = color; c.lineWidth = 3;
    c.strokeRect(Math.min(m.X0, m.X1) * W, Math.min(m.Y0, m.Y1) * H, Math.abs(m.X1 - m.X0) * W, Math.abs(m.Y1 - m.Y0) * H);
    c.fillStyle = color;
    c.beginPath(); c.arc(m.ManoX * W, m.ManoY * H, 9, 0, 7); c.fill();
    const t = `${MOTIVOS[m.Motivo] || m.Motivo} · distancia ${Math.round(100 * m.Profundidad)} %`;
    c.font = "bold 15px system-ui, sans-serif";
    const tx = Math.min(W - c.measureText(t).width - 6, Math.max(6, m.ManoX * W + 14)), ty = Math.max(18, m.ManoY * H - 12);
    c.lineWidth = 4; c.strokeStyle = "rgba(16,20,40,.85)"; c.strokeText(t, tx, ty);
    c.fillText(t, tx, ty);
  }
  c.restore();
}

function mostrarVista(v) {
  if (!v) return;
  if (vistaCamara) pintarPanel(v.panel_camara);
  $("pose").textContent = v.pose || "-";
  const esTactil = cfg && cfg.modo === "tactil";
  $("modo").textContent = esTactil && v.modo === "activo" ? "ENCENDIDO" : esTactil && v.modo === "inactivo" ? "APAGADO" : (v.modo || "-").toUpperCase();
  $("modo").className = "grande modo-" + (v.modo || "reposo");
  $("ultimo").textContent = v.ultimo || "-";
  $("ultimohora").textContent = v.ultimo_hora ? "a las " + v.ultimo_hora : "";
  mostrarIndicador(v.indicador);
  if (cfg && cfg.modo === "tactil") {
    // Interruptor: la L enciende y otra L apaga. No se apaga por tiempo.
    const aviso = v.modo === "activo" ? "Haz la L para apagar" : "Haz la L para encender";
    texto("apagatactil", aviso);
    texto("errtactil", v.error ? "No se pudo ejecutar: " + v.error : "");
  }
}

function procesar(fuente, w, h) {
  let ts = performance.now(); if (ts <= ultimoTs) ts = ultimoTs + 0.01; ultimoTs = ts;
	const inicioMano = performance.now();
  const r = lm.detectForVideo(fuente, ts);
	const manoMs = performance.now() - inicioMano;
	// Pose solo confirma que hay una persona y dónde está su torso: cambia
	// despacio, así que corre 1 de cada CADA_POSE cuadros y entre medias se
	// reutiliza. Con la mano y la pose en cada cuadro, en el portátil bajaba
	// a 7 fps cuando el sistema estaba cargado.
	let rp = ultimaPose, poseMs = 0;
	if (!rp || ++cuadrosSinPose >= CADA_POSE) {
		const inicioPose = performance.now();
		rp = ultimaPose = pm.detectForVideo(fuente, ts);
		poseMs = performance.now() - inicioPose;
		cuadrosSinPose = 0;
	}
  const manos = r.landmarks.map((pts, i) => {
    const hd = (r.handedness || r.handednesses || [])[i];
    return { derecha: !!(hd && hd[0] && hd[0].categoryName === "Right"), p: pts.map((p) => [+p.x.toFixed(5), +p.y.toFixed(5), +p.z.toFixed(5)]) };
  });
	const pose = (rp.landmarks || [])[0];
	const cuerpo = pose ? { p: pose.map((p) => [+p.x.toFixed(5), +p.y.toFixed(5), +p.z.toFixed(5), +(p.visibility ?? 1).toFixed(4)]) } : null;
  if (vistaCamara) pintar(manos, cuerpo);
  const cuadro = { t: Math.round(ts - t0), manos, cuerpo, mano_ms: manoMs, pose_ms: poseMs, ancho: w, alto: h };
  // En orden y sin amontonar: si Go se atrasa, se salta el cuadro.
  if (pendientes > 4) return;
  pendientes++;
  // Cuadro solo entrega puntos. La vista y el indicador se leen con Estado
  // cada 500 ms: devolver y deserializar toda la vista aquí duplicaba trabajo
  // en cada fotograma sin que una persona pudiera ver esa frecuencia.
  cola = cola.then(() => go().Cuadro(cuadro)).catch((e) => log("cuadro: " + e)).finally(() => pendientes--);
}

async function cargarMediaPipe() {
  if (lm && pm) return;
  $("sincamara").textContent = "cargando MediaPipe…";
  const { FilesetResolver, HandLandmarker, PoseLandmarker } = await import("./vendor/vision_bundle.mjs");
  const fileset = await FilesetResolver.forVisionTasks("./vendor/wasm");
  let offscreen = false;
  try { offscreen = typeof OffscreenCanvas !== "undefined" && !!new OffscreenCanvas(8, 8).getContext("webgl2"); } catch {}
  // WebKitGTK no da WebGL2 en OffscreenCanvas (lo que MediaPipe usa por defecto): se le pasa un canvas normal.
  const crear = async (delegate) => {
		const canvas = () => offscreen ? undefined : document.createElement("canvas");
		const mano = await HandLandmarker.createFromOptions(fileset, {
			baseOptions: { modelAssetPath: "./vendor/hand_landmarker.task", delegate }, runningMode: "VIDEO", numHands: 2,
			canvas: canvas() });
		try {
			const pose = await PoseLandmarker.createFromOptions(fileset, {
				baseOptions: { modelAssetPath: "./vendor/pose_landmarker_lite.task", delegate }, runningMode: "VIDEO", numPoses: 1,
				minPoseDetectionConfidence: 0.5, minTrackingConfidence: 0.5, canvas: canvas() });
			return [mano, pose];
		} catch (e) {
			if (mano.close) mano.close();
			throw e;
		}
	};
  // En CPU va a ~7 fps: se reintenta la GPU antes de rendirse (el contexto
  // WebGL puede fallar de forma pasajera con la gráfica ocupada).
  let delegado = "GPU", fallos = 0, detalle = "";
  for (;;) {
    try { [lm, pm] = await crear("GPU"); break; }
    catch (e) {
      fallos++; detalle = String(e && e.message || e).slice(0, 200);
      log("GPU falló (" + fallos + "): " + detalle);
      if (fallos >= 3) { delegado = "CPU"; [lm, pm] = await crear("CPU"); break; }
      await new Promise((r) => setTimeout(r, 700));
    }
  }
  delegadoMP = delegado;
  log("MediaPipe Hands + Pose listo (" + delegado + ")");
  try { await go().MediaPipeListo(delegado, fallos, detalle); } catch {}
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
  if (hayGo()) await go().CamaraApagar();
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
    // MediaPipe acepta el bitmap directamente. La copia al canvas solo sirve
    // para la vista de ajuste; con el indicador cerrado era una copia entre
    // gráficas por cuadro que nadie veía.
    if (vistaCamara) ctx.drawImage(bmp, 0, 0, 640, 480);
    procesar(bmp, bmp.width, bmp.height);
    bmp.close();
  }
}

// Windows: la webview abre la cámara. Se usa un temporizador, no
// requestVideoFrameCallback, para que siga dando cuadros con el indicador
// solo (la vista de cámara oculta no debe parar los gestos).
async function bucleWebview(gen) {
  const video = $("video");
  $("imagen").style.display = "none";
  const st = await navigator.mediaDevices.getUserMedia({ video: { width: 640, height: 480, frameRate: { ideal: 30 } }, audio: false });
  if (gen !== generacion) { st.getTracks().forEach((t) => t.stop()); return; }
  video.srcObject = st;
  await video.play();
  $("sincamara").style.display = "none";
  let ultimo = -1;
  const paso = () => {
    if (gen !== generacion) return;
    if (video.currentTime !== ultimo && video.readyState >= 2) {
      ultimo = video.currentTime;
      procesar(video, video.videoWidth || 640, video.videoHeight || 480);
    }
    setTimeout(paso, 33);
  };
  paso();
}

// ---------- el resto de la ventana ----------
function texto(id, t) { const el = $(id); if (el) el.textContent = t; }

function pintarEstado(e) {
  const panel = e.panel || {};
	modoEntradaPanel = panel.modo_entrada || "";
	panelUltimo = panel;
	mostrarIndicador(e.indicador);
	texto("panelentrada", panel.modo_entrada === "tactil" ? "Entrada: pantalla táctil virtual" : "Entrada: respaldo de ratón y teclado");
	texto("panelaviso", [panel.aviso_entrada, panel.aviso_calibracion].filter(Boolean).join(" "));
  texto("panelinstruccion", panel.instruccion || "Calibra dos esquinas y tu pellizco para crear el panel delante de ti.");
  texto("panelerror", panel.error || "");
  texto("panelestado", panel.calibrando ? `Calibrando · paso ${panel.paso} de 3` :
    (panel.calibrado ? (panel.dentro ? "Mano dentro del panel" : "Panel listo") : "Sin calibrar"));
  texto("panelpos", panel.dentro ? `${Math.round(100 * panel.x)} %, ${Math.round(100 * panel.y)} %` : "");
  $("calibrarpanel").textContent = panel.calibrado ? "Recalibrar" : "Calibrar panel";
  $("capturarpanel").style.display = panel.calibrando ? "inline-block" : "none";
  $("capturarpanel").disabled = !panel.listo_capturar;
  $("capturarpanel").textContent = panel.paso === 3 ? "Guardar mi pellizco" :
    (panel.paso === 2 ? "Guardar esquina inferior derecha" : "Guardar esquina superior izquierda");
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
  if (p) {
    $("controles").open = true;
    texto("permnombre", "«" + p.nombre + "»"); texto("permhuella", p.huella);
  }
  $("prueba").textContent = e.prueba ? "Terminar prueba de trabajo normal" : "Empezar prueba de trabajo normal";
  $("prueba").dataset.on = e.prueba ? "1" : "";
  // Estado es también la única vía de vuelta para los cuadros de cámara.
  // A 2 Hz basta para textos, diagnóstico e indicador; el aura conserva su
  // ruta independiente y limitada.
  mostrarVista(e.vista);
  const evitadas = Object.values(e.evitadas || {}).reduce((a, b) => a + b, 0);
  const fps = e.fps ? `${e.fps.toFixed(1)} cuadros/s · mano ${(100 * e.con_mano).toFixed(0)} % · zona ${(100 * e.en_zona).toFixed(0)} % · CPU ${e.cpu_core.toFixed(0)} % núcleo / ${e.cpu_total.toFixed(1)} % total · mano ${e.mano_ms.toFixed(1)} ms + Pose ${e.pose_ms.toFixed(1)} ms · evitadas ${evitadas}` : "";
  texto("fps", (e.camara ? "Cámara: " + e.camara + " " : "") + fps);
}

function esc(s) { return String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]); }

function enlazarIndicador() {
  $("abrir-ajuste").onclick = (ev) => { ev.stopPropagation(); alternarAjuste(true); };
  $("cerrar-ajuste").onclick = () => alternarAjuste(false);
  $("salir").onclick = () => { if (hayGo()) go().Salir(); };
  document.addEventListener("keydown", (ev) => {
    if ((ev.ctrlKey || ev.metaKey) && ev.shiftKey && (ev.key === "c" || ev.key === "C")) {
      ev.preventDefault(); alternarAjuste();
    } else if (ev.key === "Escape" && vistaCamara) {
      ev.preventDefault(); alternarAjuste(false);
    }
  });
}

async function main() {
  enlazarIndicador();
  await esperarWails();
  if (!hayGo()) {
    // Vista previa del indicador (sin Wails, por ejemplo al abrir el HTML
    // suelto): se puede abrir y cerrar la vista de cámara para ver el diseño.
    texto("sincamara", "vista previa: aquí se vería la cámara");
    $("camara").disabled = true;
    return;
  }
  cfg = await go().Config();
  texto("registro", cfg.registro);
  const tactil = cfg.modo === "tactil";
  texto("modoetiqueta", tactil ? "Modo táctil" : "Modo");
  $("ayudatactil").style.display = tactil ? "block" : "none";
  $("ayudatransferencia").style.display = tactil ? "none" : "block";
  $("panelcal").style.display = tactil ? "block" : "none";
  $("calibrarpanel").onclick = async () => texto("panelerror", await go().CalibrarPanel());
  $("capturarpanel").onclick = async () => texto("panelerror", await go().CapturarEsquinaPanel());
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
