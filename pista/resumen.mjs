function redondear(valor) {
  return Number.isFinite(valor) ? Math.round(valor) : 0;
}

function percentil(valores, proporcion) {
  if (!valores.length) return 0;
  const ordenados = [...valores].sort((a, b) => a - b);
  const indice = Math.max(0, Math.ceil(ordenados.length * proporcion) - 1);
  return ordenados[indice];
}

function calcularResumen(sesion = {}) {
  const blancos = Array.isArray(sesion.blancos) ? sesion.blancos : [];
  const tiempos = blancos.map((blanco) => Number(blanco.tiempoMs) || 0);
  const aciertosPrimera = blancos.filter((blanco) => (Number(blanco.toquesFuera) || 0) === 0).length;
  const toquesFuera = blancos.reduce((total, blanco) => total + (Number(blanco.toquesFuera) || 0), 0);
  const reposo = sesion.reposo || {};
  const planeada = Number(reposo.duracionPlaneadaMs) || 0;
  const real = Number(reposo.duracionRealMs) || 0;

  return {
    esquema: 1,
    inicio: sesion.inicio || null,
    fin: sesion.fin || null,
    blancos_total: blancos.length,
    aciertos_primera: aciertosPrimera,
    precision_primera_pct: blancos.length ? redondear(aciertosPrimera * 1000 / blancos.length) / 10 : 0,
    fallos: blancos.length - aciertosPrimera,
    toques_fuera_blanco: toquesFuera,
    tiempo_promedio_ms: tiempos.length ? redondear(tiempos.reduce((a, b) => a + b, 0) / tiempos.length) : 0,
    tiempo_mediana_ms: redondear(percentil(tiempos, 0.5)),
    tiempo_p95_ms: redondear(percentil(tiempos, 0.95)),
    reposo: {
      duracion_planeada_ms: planeada,
      duracion_real_ms: real,
      completo: planeada > 0 && real >= planeada,
      toques_falsos: Number(reposo.toquesFalsos) || 0,
    },
    blancos: blancos.map((blanco) => ({
      id: blanco.id,
      tipo: blanco.tipo,
      tamano_px: blanco.tamanoPx,
      posicion: blanco.posicion,
      acierto_primera: (Number(blanco.toquesFuera) || 0) === 0,
      toques_fuera: Number(blanco.toquesFuera) || 0,
      tiempo_ms: Number(blanco.tiempoMs) || 0,
    })),
  };
}

globalThis.LazulResumen = { calcularResumen };
