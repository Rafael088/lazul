---
id: proyecto
version: 0.1.0
---
# lazul

## Objetivo
Lazul convierte la pantalla de un computador Windows o Linux en una pantalla táctil que se maneja en el aire, con la webcam que ya tienes y sin hardware extra. Es para cuando no tienes el ratón a mano: dar clase o presentar de pie, o el PC lejos. No promete reemplazar al ratón. La 0.2.0 pasa si cumple los 8 criterios de dec-uaex a más tardar el 2026-12-07. El táctil completo es gratis; «Un solo espacio» (agarrar en un equipo y soltar en otro) va como «próximamente» y será el servicio de pago (dec-0j4m). El video y los puntos de la mano nunca salen del equipo.
## Stack
Núcleo en Go. Escritorio con Wails: MediaPipe Hand Landmarker y Pose en JS dentro de la webview; en Linux la cámara entra por ffmpeg (dec-7tgj). Salida por pantalla táctil virtual (dec-yp1a): uinput en Linux (Hyprland) e InjectTouchInput en Windows, con imán por AT-SPI y UI Automation. Contratos en proto/ con tipos generados. Web en Next.js cuando pasen los criterios; Android en Kotlin, después. Repo público con licencia FSL.
## Siguiente paso
Hasta el 2026-11-24: medir la CPU en Linux con la versión final, bajarla solo si pasa de un núcleo y congelar la versión. Del 2026-11-25 al 2026-12-07: validar dec-uaex en los dos PC (pista, reposo de 10 min, gestos y uso real). Al pasar: video real y web. Antes de eso, ni landing ni video.
