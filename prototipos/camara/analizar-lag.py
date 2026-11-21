#!/usr/bin/env python3
"""Resume un episodio de lag del registro y estima el presupuesto liberado.

No presenta la estimación como una medida de GPU: calcula el límite conservador
de espaciar Pose de 1/3 a 1/6, sin atribuir a Hand ninguna mejora adicional.
Uso: analizar-lag.py REGISTRO SESION
"""

import json
import math
import pathlib
import statistics
import sys


def correlacion(xs, ys):
    mx, my = statistics.mean(xs), statistics.mean(ys)
    arriba = sum((x - mx) * (y - my) for x, y in zip(xs, ys))
    abajo = math.sqrt(sum((x - mx) ** 2 for x in xs) * sum((y - my) ** 2 for y in ys))
    return arriba / abajo if abajo else 0


def media(xs, clave):
    return round(statistics.mean(x[clave] for x in xs), 2)


if len(sys.argv) != 3:
    raise SystemExit("uso: analizar-lag.py REGISTRO SESION")

ruta, sesion = pathlib.Path(sys.argv[1]), sys.argv[2]
muestras = []
for linea in ruta.read_text().splitlines():
    try:
        dato = json.loads(linea)
    except json.JSONDecodeError:
        continue
    if dato.get("tipo") == "rendimiento" and dato.get("sesion") == sesion:
        muestras.append(dato)

if not muestras:
    raise SystemExit(f"sin muestras de rendimiento para {sesion}")

lentas = [m for m in muestras if m.get("mano_ms", 0) >= 24]
if not lentas:
    raise SystemExit(f"la sesión {sesion} no tiene Hand >= 24 ms")

periodos = [1000 / m["fps"] for m in muestras]
estimados = []
for m in lentas:
    # pose_ms ya está promediado sobre todos los cuadros. Pasar de 1/3 a
    # 1/6 libera la mitad, aun suponiendo que Hand no se acelera nada.
    periodo = 1000 / m["fps"]
    estimados.append(min(30, 1000 / max(1, periodo - m["pose_ms"] / 2)))

salida = {
    "sesion": sesion,
    "muestras": len(muestras),
    "lentas_hand_ge_24ms": len(lentas),
    "antes": {
        "fps": media(lentas, "fps"),
        "mano_ms": media(lentas, "mano_ms"),
        "pose_ms": media(lentas, "pose_ms"),
        "cpu_core": media(lentas, "cpu_core"),
    },
    "correlacion_periodo": {
        "mano_ms": round(correlacion(periodos, [m["mano_ms"] for m in muestras]), 3),
        "pose_ms": round(correlacion(periodos, [m["pose_ms"] for m in muestras]), 3),
        "cpu_core": round(correlacion(periodos, [m["cpu_core"] for m in muestras]), 3),
    },
    "modelo_pose_1_de_6": {
        "fps_sin_mejora_adicional_de_hand": round(statistics.mean(estimados), 2),
        "ms_liberados_por_cuadro": round(media(lentas, "pose_ms") / 2, 2),
    },
}
print(json.dumps(salida, ensure_ascii=False, indent=2))
