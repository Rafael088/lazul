#!/usr/bin/env bash
# Baja MediaPipe tasks-vision, el modelo de la mano y la foto de prueba a frontend/vendor (no se versionan).
set -euo pipefail
cd "$(dirname "$0")/frontend"
VER=1.2.2
mkdir -p vendor && cd vendor
[ -f vision_bundle.mjs ] || { curl -sSfL "https://registry.npmjs.org/@mediapipe/tasks-vision/-/tasks-vision-$VER.tgz" | tar xz --strip-components=1 package/vision_bundle.mjs package/wasm; }
[ -f hand_landmarker.task ] || curl -sSfLo hand_landmarker.task https://storage.googleapis.com/mediapipe-models/hand_landmarker/hand_landmarker/float16/1/hand_landmarker.task
[ -f mano.jpg ] || curl -sSfLo mano.jpg https://storage.googleapis.com/mediapipe-assets/pointing_up.jpg
ls -la
