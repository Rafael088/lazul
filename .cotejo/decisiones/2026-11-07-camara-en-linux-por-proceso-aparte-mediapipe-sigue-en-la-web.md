---
id: dec-7tgj
fecha: 2026-11-07
temas: [gestos, arquitectura, linux]
estado: propuesta
autor: agente
---
# Cámara en Linux por proceso aparte; MediaPipe sigue en la webview

Prueba cotejo-cphace (prototipos/camara). MediaPipe Hand Landmarker corre bien dentro de la webview en las dos plataformas con GPU: 14,5 ms por cuadro en WebView2 y 15 ms en WebKitGTK, unos 35 % de un núcleo. Lo que falla en Linux es la cámara dentro de WebKitGTK: Wails v2 no concede el permiso (lo parcheamos con cgo) y aun con permiso, video.srcObject cuelga la página; la sospecha es que falta gst-plugins-good en el equipo (no verificado). Propuesta: (1) Windows abre la cámara en la webview con getUserMedia (medido: captura→puntos p50 20 ms, p95 33 ms; la cámara dio 16,6 fps con poca luz). (2) Linux abre la cámara en un proceso aparte (ffmpeg con V4L2) que le pasa los JPEG a la webview por el AssetServer; MediaPipe no se mueve (medido: 30 fps, captura→puntos p50 17 ms, p95 22 ms, 35 % de un núcleo con ffmpeg incluido). (3) La fuente de cuadros va detrás de una sola interfaz en desktop/, así el clasificador no sabe de dónde viene el cuadro. (4) No hace falta el plan B completo (MediaPipe nativo fuera de la webview): la inferencia no fue el problema. (5) Wails necesita en Linux WebviewGpuPolicy (sin él no hay WebGL) y un canvas normal para MediaPipe (WebKitGTK no da WebGL2 en OffscreenCanvas). Si al instalar gst-plugins-good la cámara directa funciona en WebKitGTK, se puede volver a getUserMedia también en Linux y quitar ffmpeg. Pendiente: medir con una mano real frente a la cámara (aquí se pegó una foto de una mano sobre el cuadro real).
