#!/usr/bin/env bash
# Corre una medida en Hyprland sin robar el foco y deja el JSON en resultados/.
# Uso: ./medir-linux.sh <modo> <segundos> <delegado> <nombre> [VAR=valor ...]
# La ventana va al espacio 66, en una pantalla virtual temporal (LAZULPRUEBA) lejos de las reales:
# en un espacio oculto Hyprland no da fotogramas y la página se queda a 0 fps.
# Hace falta compilar antes: go build -tags desktop,production,webkit2_41 -o camara .
set -euo pipefail
cd "$(dirname "$0")"
modo=$1 seg=$2 del=$3 nombre=$4; shift 4
out="$PWD/resultados/$nombre.json" log="$PWD/resultados/$nombre.log"
mkdir -p resultados; rm -f "$out"
if ! hyprctl monitors | grep -q "^Monitor LAZULPRUEBA"; then
  hyprctl output create headless LAZULPRUEBA >/dev/null
  trap 'hyprctl output remove LAZULPRUEBA >/dev/null' EXIT
  sleep 1
  hyprctl eval 'hl.monitor({ output = "LAZULPRUEBA", mode = "1280x800@60", position = "-9000x-9000", scale = 1 })' >/dev/null
  hyprctl eval 'hl.workspace_rule({ workspace = "66", monitor = "LAZULPRUEBA", default = true })' >/dev/null
fi
hyprctl eval 'hl.window_rule({ match = { title = "Lazul · prueba de cámara" }, workspace = "66 silent", no_initial_focus = true, focus_on_activate = false })' >/dev/null
hyprctl eval "hl.exec_cmd(\"env $* $PWD/camara -modo $modo -segundos $seg -calentar 5 -delegado $del -salida $out > $log 2>&1\")" >/dev/null
for _ in $(seq 1 $((seg + 120))); do sleep 1; [ -s "$out" ] && break; done
[ -s "$out" ] || { echo "sin resultado"; tail -5 "$log"; exit 1; }
python3 - "$out" <<'PY'
import json,sys; d=json.load(open(sys.argv[1])); w=d["web"]
print(json.dumps({"cpu_%nucleo": round(d["cpu_pct_nucleo"],1), "cpu_%total": round(d["cpu_pct_total"],1),
  **{k: w.get(k) for k in ("error","delegado_usado","fps","fps_camara","cuadros_con_mano","inferencia_ms","cuadro_a_puntos_ms","captura_a_puntos_ms","camara")}}, ensure_ascii=False))
PY
