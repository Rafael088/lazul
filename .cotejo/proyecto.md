---
id: proyecto
version: 0.1.0
---
# lazul

## Objetivo
Lazul convierte una webcam en el control de todos los dispositivos de la red local: con gestos de la mano (y voz opcional) agarras una captura, texto, una URL o una app en un equipo y la sueltas en otro. Todo se procesa en local. La 0.1.0 es la fase 1 del MVP: 2 PCs (Windows y Linux Hyprland), gestos 1 a 4 y envío de capturas; pasa con precisión de gestos ≥ 95 % y latencia de comando < 100 ms.
## Stack
Núcleo en Go (red, emparejamiento, protocolo, permisos). Escritorio con Wails: MediaPipe Hand Landmarker en JS dentro de la webview y el clasificador de gestos como función pura. Android en Kotlin (fase 2). Web en Next.js (fase 3). Contratos en proto/ con tipos generados. Repo público con licencia FSL.
## Siguiente paso
Prueba de cámara: Wails + MediaPipe en la webview de Windows (WebView2) y de Hyprland (WebKitGTK), midiendo fps y latencia. Si falla en Linux, plan B: motor de gestos como proceso aparte.
