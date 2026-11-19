#!/usr/bin/env bash
# Repite en la NVIDIA el desglose de la ruta caliente y compara el recorrido
# anterior con el optimizado. Uso: ./medir-partes.sh [segundos] [nombre]
set -euo pipefail
cd "$(dirname "$0")"
seg=${1:-20}
nombre=${2:-nvidia}

go build -tags desktop,production,webkit2_41 -o camara .
gpu=(LAZUL_VARIANTE=base)
if [ -f /usr/share/glvnd/egl_vendor.d/10_nvidia.json ]; then
	gpu+=(__EGL_VENDOR_LIBRARY_FILENAMES=/usr/share/glvnd/egl_vendor.d/10_nvidia.json __NV_PRIME_RENDER_OFFLOAD=1)
fi

variantes=(base envio respuesta indicador aura copia antes despues)
for variante in "${variantes[@]}"; do
	envvars=("${gpu[@]}")
	envvars[0]="LAZUL_VARIANTE=$variante"
	./medir-linux.sh imagen "$seg" GPU "partes-$nombre-$variante" "${envvars[@]}"
done

python3 - "$nombre" "${variantes[@]}" <<'PY'
import json, pathlib, sys
nombre, *variantes = sys.argv[1:]
print("variante       fps   CPU %núcleo  puente p50 ms")
for v in variantes:
    p = pathlib.Path("resultados") / f"partes-{nombre}-{v}.json"
    d = json.loads(p.read_text())
    w = d["web"]
    puente = (w.get("puente_go_ms") or {}).get("p50")
    print(f"{v:12} {w.get('fps', 0):5.1f} {d.get('cpu_pct_nucleo', 0):12.1f} {str(puente if puente is not None else '-'):>14}")
PY
