#!/usr/bin/env bash
# Prueba con una mano de verdad en Linux: abre la ventana visible, 5 s de calentamiento y 30 s midiendo.
# Pon la mano abierta frente a la cámara y muévela despacio. Deja el JSON en resultados/linux-mano-real.json.
# Uso: ./probar-con-mano.sh [segundos]   (hace falta compilar antes: go build -tags desktop,production,webkit2_41 -o camara .)
set -euo pipefail
cd "$(dirname "$0")"
seg=${1:-30}
./camara -modo proceso -segundos "$seg" -calentar 5 -delegado GPU -salida "$PWD/resultados/linux-mano-real.json" 2> resultados/linux-mano-real.log
python3 - <<'PY'
import json; d=json.load(open("resultados/linux-mano-real.json")); w=d["web"]
print({"fps": w.get("fps"), "mano_en_cuadros": f'{w.get("cuadros_con_mano")}', "inferencia_ms": w.get("inferencia_ms"),
       "captura_a_puntos_ms": w.get("captura_a_puntos_ms"), "cpu_%nucleo": round(d["cpu_pct_nucleo"],1)})
PY
