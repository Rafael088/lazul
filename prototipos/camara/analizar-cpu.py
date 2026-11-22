#!/usr/bin/env python3
"""Mide el criterio 5 de dec-uaex (CPU y fps) en el registro de la app real.

Cuenta las muestras de 5 s tomadas con el panel encendido y una media de al
menos 0,5 manos: el filtro que da la referencia del 21-nov (34 muestras,
126 %). No distingue si la mano estaba dentro del rectángulo del panel: el
registro no lo guarda. Hace falta una sesión con 5 min en esa condición.

Uso: analizar-cpu.py REGISTRO [SESION]   (sin SESION: la última con muestras)
Salida: 0 cumple, 1 no cumple, 2 faltan muestras, 3 no se pudo leer.
"""

import json
import pathlib
import statistics
import sys

MUESTRAS_5_MIN = 60  # la app anota una muestra cada 5 s


def leer(ruta):
    """Muestras por sesión, cada una con el estado del panel al anotarla."""
    sesiones = {}
    panel = False
    for linea in ruta.read_text().splitlines():
        try:
            d = json.loads(linea)
        except json.JSONDecodeError:
            continue
        tipo, resultado = d.get("tipo"), d.get("resultado")
        if tipo in ("inicio", "fin"):
            panel = False
        elif tipo == "panel" and resultado in ("enciende", "apaga"):
            panel = resultado == "enciende"
        elif tipo == "rendimiento" and d.get("cpu_core") and d.get("fps"):
            d["panel"] = panel
            sesiones.setdefault(d.get("sesion", ""), []).append(d)
    return sesiones


def mediana(xs, clave):
    return statistics.median(x.get(clave, 0) for x in xs)


def describir(nombre, xs):
    if not xs:
        print(f"{nombre:25}   0")
        return
    print(f"{nombre:25} {len(xs):3} {mediana(xs, 'cpu_core'):7.1f}"
          f" {sum(x['cpu_core'] > 100 for x in xs):5} {mediana(xs, 'fps'):6.1f}"
          f" {sum(x['fps'] < 25 for x in xs):4} {mediana(xs, 'mano_ms'):7.1f}"
          f" {mediana(xs, 'pose_ms'):7.1f}")


def main():
    if len(sys.argv) not in (2, 3):
        print("uso: analizar-cpu.py REGISTRO [SESION]", file=sys.stderr)
        return 3
    try:
        sesiones = leer(pathlib.Path(sys.argv[1]))
    except OSError as e:
        print(e, file=sys.stderr)
        return 3
    sesion = sys.argv[2] if len(sys.argv) == 3 else next(reversed(sesiones), None)
    muestras = sesiones.get(sesion)
    if not muestras:
        print(f"sin muestras de rendimiento para {sesion}", file=sys.stderr)
        return 3

    if not any("pose_cada" in x or "manos_media" in x for x in muestras):
        print(f"{sesion} es anterior a manos_media: no dice si había mano", file=sys.stderr)
        return 3

    print(f"sesión {sesion}")
    print(f"{'condición':25}   n  CPU %  >100    fps  <25 Hand ms Pose ms")
    grupos = {}
    for panel in (True, False):
        for mano in (True, False):
            xs = [x for x in muestras
                  if x["panel"] == panel and (x.get("manos_media", 0) >= 0.5) == mano]
            grupos[panel, mano] = xs
            describir(f"panel {'sí' if panel else 'no'}, mano {'sí' if mano else 'no'}", xs)

    # Cuánto del tiempo de pared pasa la webview dentro de la inferencia:
    # no es CPU, pero si la sigue, el peso está ahí o en esperar a la GPU.
    inferencia = [(x.get("mano_ms", 0) + x.get("pose_ms", 0)) * x["fps"] / 10 for x in muestras]
    cpu = [x["cpu_core"] for x in muestras]
    if len(set(inferencia)) > 1 and len(set(cpu)) > 1:
        print(f"correlación CPU / tiempo en inferencia: {statistics.correlation(inferencia, cpu):.3f}")

    medidas = grupos[True, True]
    if len(medidas) < MUESTRAS_5_MIN:
        print(f"FALTAN MUESTRAS: {len(medidas)} de {MUESTRAS_5_MIN} con panel y mano")
        return 2
    cumple = mediana(medidas, "cpu_core") < 100 and mediana(medidas, "fps") >= 25
    print("CUMPLE" if cumple else "NO CUMPLE", "(mediana bajo 100 % y 25 fps o más)")
    return 0 if cumple else 1


if __name__ == "__main__":
    sys.exit(main())
