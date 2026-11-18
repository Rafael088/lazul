#!/usr/bin/env bash
# Baja MediaPipe tasks-vision y los modelos de mano y Pose a frontend/vendor (no se
# versionan; van dentro del ejecutable). Si ya están en prototipos/camara, los copia.
set -euo pipefail
cd "$(dirname "$0")/frontend"
VER=1.2.2
mkdir -p vendor && cd vendor
proto=../../../../prototipos/camara/frontend/vendor
if [ -f "$proto/vision_bundle.mjs" ] && [ -d "$proto/wasm" ]; then
  [ -f vision_bundle.mjs ] || cp "$proto/vision_bundle.mjs" .
  [ -d wasm ] || cp -r "$proto/wasm" .
  if [ ! -f hand_landmarker.task ] && [ -f "$proto/hand_landmarker.task" ]; then cp "$proto/hand_landmarker.task" .; fi
  if [ ! -f pose_landmarker_lite.task ] && [ -f "$proto/pose_landmarker_lite.task" ]; then cp "$proto/pose_landmarker_lite.task" .; fi
fi
[ -f vision_bundle.mjs ] || { curl -sSfL "https://registry.npmjs.org/@mediapipe/tasks-vision/-/tasks-vision-$VER.tgz" | tar xz --strip-components=1 package/vision_bundle.mjs package/wasm; }
[ -f hand_landmarker.task ] || curl -sSfLo hand_landmarker.task https://storage.googleapis.com/mediapipe-models/hand_landmarker/hand_landmarker/float16/1/hand_landmarker.task
[ -f pose_landmarker_lite.task ] || curl -sSfLo pose_landmarker_lite.task https://storage.googleapis.com/mediapipe-models/pose_landmarker/pose_landmarker_lite/float16/1/pose_landmarker_lite.task
