---
id: arq-mgew
temas: [arquitectura]
actualizado: 2026-10-26
---
# Arquitectura inicial

Piezas: (1) núcleo Go: descubrimiento mDNS, emparejamiento con código de 6 dígitos y TLS con huella fijada, permisos por dispositivo, enrutador de comandos; no llama al sistema. (2) platform/<so>/: una interfaz por capacidad (Screenshot, Clipboard, InjectKey, Media, ActiveApp, LaunchApp), un archivo por sistema. (3) desktop/ (Wails): webview con cámara y MediaPipe Hand Landmarker; el clasificador (puntos de la mano → gesto) es una función pura en gestures/, probada con grabaciones en fixtures/gestos/. (4) proto/: esquema único de mensajes; se generan los tipos de Go y TS. Cómo se hablan: la webview emite eventos de gesto al núcleo Go; el controlador manda a cada receptor comandos de pocos bytes por WebSocket; los archivos viajan directo de origen a destino con el protocolo de LocalSend. ./dev sim levanta un controlador y N receptores falsos en local.
