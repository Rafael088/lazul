#!/usr/bin/env bash
# Compara la carga que tenía la app (Pose cada 3 cuadros) con el modo de
# protección (cada 6) manteniendo dos manos. Requiere el escritorio/GPU real.
# Uso: ./medir-lag.sh [segundos] [nombre]
set -euo pipefail
cd "$(dirname "$0")"
seg=${1:-30}
nombre=${2:-lag}

go build -tags desktop,production,webkit2_41 -o camara .
./medir-linux.sh imagen "$seg" GPU "$nombre-antes" LAZUL_NUM_HANDS=2 LAZUL_POSE_CADA=3
./medir-linux.sh imagen "$seg" GPU "$nombre-despues" LAZUL_NUM_HANDS=2 LAZUL_POSE_CADA=6

python3 - "$nombre" <<'PY'
import json, pathlib, sys
nombre = sys.argv[1]
print("medida       fps  mano p50  Pose p50  CPU %núcleo")
for etapa in ("antes", "despues"):
    d = json.loads((pathlib.Path("resultados") / f"{nombre}-{etapa}.json").read_text())
    w = d["web"]
    print(f"{etapa:10} {w['fps']:5.1f} {w['inferencia_ms']['p50']:9.1f} "
          f"{(w.get('pose_ms') or {}).get('p50', 0):9.1f} {d['cpu_pct_nucleo']:12.1f}")
PY
