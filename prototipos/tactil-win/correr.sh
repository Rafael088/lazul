#!/usr/bin/env bash
# Corre un guion del táctil en el escritorio del PC Windows (ssh win).
# Uso: prototipos/tactil-win/correr.sh guion.txt [archivos que traer de vuelta...]
set -euo pipefail
cd "$(dirname "$0")/../.."
guion=$1; shift
d=.dev/tactil-win; mkdir -p $d
GOOS=windows GOARCH=amd64 go build -o $d/tactil-win.exe ./prototipos/tactil-win
ssh -o LogLevel=ERROR win 'New-Item -Force -ItemType Directory "$HOME\lazul-tactil" | Out-Null' >/dev/null
scp -q -o LogLevel=ERROR $d/tactil-win.exe prototipos/tactil-win/pagina.html prototipos/tactil-win/prueba.pdf herramientas/win-escritorio.ps1 win:lazul-tactil/
scp -q -o LogLevel=ERROR "$guion" win:lazul-tactil/guion.txt
ssh -o LogLevel=ERROR win '& "$HOME\lazul-tactil\win-escritorio.ps1" -exe "$HOME\lazul-tactil\tactil-win.exe" -argumentos "guion guion.txt" -segundos 300' | tr -d '\r'
for f in "$@"; do scp -q -o LogLevel=ERROR "win:lazul-tactil/$f" $d/ && echo "traído $d/$f"; done
