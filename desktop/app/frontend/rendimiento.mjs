export const INTERVALO_CUADRO_MS = 1000 / 30;

// Pose delimita el torso, que cambia mucho más despacio que la mano. Cuando
// Hand ya consume casi todo un cuadro, espaciar Pose libera la misma GPU sin
// quitar el seguimiento de dos manos que necesita el zoom.
export function cuadrosEntrePose(manoMediaMs) {
  return manoMediaMs >= 24 ? 6 : 3;
}

export function mediaSuave(anterior, muestra) {
  return anterior > 0 ? anterior * 0.8 + muestra * 0.2 : muestra;
}

// Devuelve cuánto falta para el siguiente cuadro de 30 Hz. Si la inferencia
// ya consumió ese plazo, continúa enseguida pero empieza un plazo nuevo: no
// suma siempre 33 ms al costo de MediaPipe ni intenta una ráfaga atrasada.
export function planearSiguienteCuadro(ahora, objetivoAnterior) {
  const candidato = objetivoAnterior + INTERVALO_CUADRO_MS;
  const objetivo = candidato < ahora ? ahora : candidato;
  return { objetivo, espera: Math.max(0, objetivo - ahora) };
}
