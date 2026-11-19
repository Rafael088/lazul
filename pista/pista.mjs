const $ = (id) => document.getElementById(id);
const { calcularResumen } = globalThis.LazulResumen;
const ICONO = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2 15 9l7 .7-5.3 4.7 1.6 7.1L12 17.8l-6.3 3.7 1.6-7.1L2 9.7 9 9z"/></svg>';
const blancos = [
  { id: "enlace-28", tipo: "enlace", tamanoPx: 28, posicion: "centro arriba" },
  { id: "boton-44", tipo: "botón", tamanoPx: 44, posicion: "centro derecha" },
  { id: "icono-32", tipo: "ícono", tamanoPx: 32, posicion: "centro abajo" },
  { id: "enlace-40", tipo: "enlace", tamanoPx: 40, posicion: "centro izquierda" },
  { id: "icono-arriba", tipo: "ícono", tamanoPx: 36, posicion: "borde arriba" },
  { id: "boton-derecha", tipo: "botón", tamanoPx: 48, posicion: "borde derecha" },
  { id: "enlace-abajo", tipo: "enlace", tamanoPx: 34, posicion: "borde abajo" },
  { id: "icono-izquierda", tipo: "ícono", tamanoPx: 28, posicion: "borde izquierda" },
  { id: "esquina-si", tipo: "botón", tamanoPx: 44, posicion: "esquina superior izquierda" },
  { id: "esquina-sd", tipo: "ícono", tamanoPx: 32, posicion: "esquina superior derecha" },
  { id: "esquina-id", tipo: "enlace", tamanoPx: 36, posicion: "esquina inferior derecha" },
  { id: "esquina-ii", tipo: "botón", tamanoPx: 52, posicion: "esquina inferior izquierda" },
  { id: "boton-36", tipo: "botón", tamanoPx: 36, posicion: "centro abajo" },
  { id: "icono-24", tipo: "ícono", tamanoPx: 24, posicion: "centro derecha" },
];

let indice = 0;
let inicioSesion = null;
let inicioBlanco = 0;
let fueraActual = 0;
let resultados = [];
let bloqueado = false;
let reloj = 0;
let duracionReposo = 600000;
let inicioReposo = 0;
let toquesFalsos = 0;
let resumenFinal = null;

function mostrar(id) {
  for (const pantalla of document.querySelectorAll("main.pantalla")) pantalla.hidden = pantalla.id !== id;
}

function slug(texto) {
  return texto.normalize("NFD").replace(/[\u0300-\u036f]/g, "").replaceAll(" ", "-");
}

function crearBlanco(datos) {
  const elemento = datos.tipo === "enlace" ? document.createElement("a") : document.createElement("button");
  if (datos.tipo === "enlace") elemento.href = "#blanco";
  else elemento.type = "button";
  elemento.className = `blanco tipo-${slug(datos.tipo)} pos-${slug(datos.posicion)}`;
  elemento.style.setProperty("--tam", `${datos.tamanoPx}px`);
  elemento.dataset.blanco = datos.id;
  elemento.setAttribute("aria-label", `Blanco ${datos.tipo} de ${datos.tamanoPx} píxeles`);
  if (datos.tipo === "ícono") elemento.innerHTML = ICONO;
  else elemento.textContent = datos.tipo === "enlace" ? "Abrir" : "Tocar";
  elemento.addEventListener("click", acertar);
  return elemento;
}

function pintarBlanco() {
  bloqueado = false;
  fueraActual = 0;
  const datos = blancos[indice];
  $("superficie").replaceChildren(crearBlanco(datos));
  $("paso").textContent = `Blanco ${indice + 1} de ${blancos.length}`;
  $("instruccion").textContent = `${datos.tipo} · ${datos.tamanoPx} px · ${datos.posicion}`;
  $("cuenta-fuera").textContent = "Sin toques fuera";
  $("barra").style.width = `${indice * 100 / blancos.length}%`;
  $("hud").className = "hud";
  inicioBlanco = performance.now();
}

function empezar() {
  indice = 0;
  resultados = [];
  inicioSesion = new Date();
  duracionReposo = Number($("duracion").value);
  mostrar("prueba");
  pintarBlanco();
  clearInterval(reloj);
  reloj = setInterval(() => {
    $("cronometro").textContent = `${((performance.now() - inicioBlanco) / 1000).toFixed(1).replace(".", ",")} s`;
  }, 100);
}

function registrarFuera(evento) {
  if ($("prueba").hidden || bloqueado || !evento.target.closest("#prueba") || evento.target.closest("[data-blanco]")) return;
  fueraActual++;
  $("cuenta-fuera").textContent = `${fueraActual} ${fueraActual === 1 ? "toque fuera" : "toques fuera"}`;
  $("hud").className = "hud fallo";
  $("instruccion").textContent = "Fuera del blanco · vuelve a intentarlo";
  setTimeout(() => { if (!bloqueado) $("hud").className = "hud"; }, 220);
}

function acertar(evento) {
  evento.preventDefault();
  if (bloqueado) return;
  bloqueado = true;
  const datos = blancos[indice];
  resultados.push({ ...datos, tiempoMs: Math.round(performance.now() - inicioBlanco), toquesFuera: fueraActual });
  $("hud").className = "hud acierto";
  $("instruccion").textContent = fueraActual ? "Acierto" : "A la primera";
  $("barra").style.width = `${(indice + 1) * 100 / blancos.length}%`;
  $("superficie").replaceChildren();
  setTimeout(() => {
    indice++;
    if (indice < blancos.length) pintarBlanco();
    else empezarReposo();
  }, 320);
}

function formatoRestante(ms) {
  const segundos = Math.max(0, Math.ceil(ms / 1000));
  return `${String(Math.floor(segundos / 60)).padStart(2, "0")}:${String(segundos % 60).padStart(2, "0")}`;
}

function empezarReposo() {
  clearInterval(reloj);
  mostrar("reposo");
  inicioReposo = performance.now();
  toquesFalsos = 0;
  $("restante").textContent = formatoRestante(duracionReposo);
  $("falsos").textContent = "0 toques falsos";
  reloj = setInterval(() => {
    const restante = duracionReposo - (performance.now() - inicioReposo);
    $("restante").textContent = formatoRestante(restante);
    if (restante <= 0) terminar();
  }, 250);
}

function registrarFalso(evento) {
  if ($("reposo").hidden || evento.target.closest("button")) return;
  toquesFalsos++;
  $("falsos").textContent = `${toquesFalsos} ${toquesFalsos === 1 ? "toque falso" : "toques falsos"}`;
  $("zona-reposo").classList.add("tocado");
  setTimeout(() => $("zona-reposo").classList.remove("tocado"), 220);
}

function nombreArchivo() {
  return `lazul-pista-${new Date().toISOString().replaceAll(":", "-")}.json`;
}

function descargar(resumen) {
  const blob = new Blob([`${JSON.stringify(resumen, null, 2)}\n`], { type: "application/json" });
  const enlace = document.createElement("a");
  enlace.href = URL.createObjectURL(blob);
  enlace.download = nombreArchivo();
  enlace.click();
  setTimeout(() => URL.revokeObjectURL(enlace.href), 1000);
}

function terminar() {
  if ($("reposo").hidden) return;
  clearInterval(reloj);
  const fin = new Date();
  resumenFinal = calcularResumen({
    inicio: inicioSesion.toISOString(),
    fin: fin.toISOString(),
    blancos: resultados,
    reposo: {
      duracionPlaneadaMs: duracionReposo,
      duracionRealMs: Math.round(performance.now() - inicioReposo),
      toquesFalsos,
    },
  });
  try { localStorage.setItem("lazul.pista.ultima", JSON.stringify(resumenFinal)); } catch {}
  mostrarResumen(resumenFinal);
  descargar(resumenFinal);
}

function mostrarResumen(resumen) {
  mostrar("final");
  $("precision").textContent = `${resumen.aciertos_primera}/${resumen.blancos_total}`;
  $("fallos").textContent = resumen.fallos;
  $("fuera").textContent = resumen.toques_fuera_blanco;
  $("tiempo").textContent = `${(resumen.tiempo_promedio_ms / 1000).toFixed(1).replace(".", ",")} s`;
  $("falsos-final").textContent = resumen.reposo.toques_falsos;
  $("detalle").replaceChildren(...resumen.blancos.map((blanco) => {
    const fila = document.createElement("tr");
    fila.innerHTML = `<td>${blanco.tipo} · ${blanco.tamano_px} px</td><td>${blanco.posicion}</td><td class="${blanco.acierto_primera ? "si" : "no"}">${blanco.acierto_primera ? "Sí" : `No · ${blanco.toques_fuera} fuera`}</td><td>${(blanco.tiempo_ms / 1000).toFixed(1).replace(".", ",")} s</td>`;
    return fila;
  }));
  $("guardado").textContent = resumen.reposo.completo
    ? "El JSON se descargó automáticamente. El reposo completó el tiempo previsto."
    : "El JSON se descargó automáticamente. El reposo terminó antes del tiempo previsto y así consta en el archivo.";
}

$("empezar").addEventListener("click", empezar);
$("terminar-reposo").addEventListener("click", terminar);
$("descargar").addEventListener("click", () => descargar(resumenFinal));
$("repetir").addEventListener("click", () => location.reload());
document.addEventListener("click", registrarFuera);
$("reposo").addEventListener("click", registrarFalso);
