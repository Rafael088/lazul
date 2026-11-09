#!/usr/bin/env bash
# Baja MediaPipe tasks-vision y el modelo de la mano a frontend/vendor (no se
# versionan; van dentro del ejecutable). Si ya están en prototipos/camara, los copia.
set -euo pipefail
cd "$(dirname "$0")/frontend"
VER=1.2.2
mkdir -p vendor && cd vendor
proto=../../../../prototipos/camara/frontend/vendor
if [ ! -f vision_bundle.mjs ] && [ -f "$proto/vision_bundle.mjs" ] && [ -d "$proto/wasm" ]; then
  cp -r "$proto/vision_bundle.mjs" "$proto/wasm" .
  [ -f "$proto/hand_landmarker.task" ] && cp "$proto/hand_landmarker.task" .
fi
[ -f vision_bundle.mjs ] || { curl -sSfL "https://registry.npmjs.org/@mediapipe/tasks-vision/-/tasks-vision-$VER.tgz" | tar xz --strip-components=1 package/vision_bundle.mjs package/wasm; }
[ -f hand_landmarker.task ] || curl -sSfLo hand_landmarker.task https://storage.googleapis.com/mediapipe-models/hand_landmarker/hand_landmarker/float16/1/hand_landmarker.task
