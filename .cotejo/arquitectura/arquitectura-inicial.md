---
id: arq-mgew
temas: [arquitectura]
actualizado: 2026-11-16
---
# Arquitectura inicial

Piezas: (1) núcleo Go: descubrimiento mDNS, emparejamiento con código de 6 dígitos y TLS con huella fijada, permisos por dispositivo, enrutador de comandos; no llama al sistema. (2) platform/<so>/: una interfaz por capacidad (Screenshot, Clipboard, InjectKey, Media, ActiveApp, LaunchApp), un archivo por sistema. (3) desktop/ (Wails): webview con cámara y MediaPipe Hand Landmarker; el clasificador (puntos de la mano → gesto) es una función pura en gestures/, probada con grabaciones en fixtures/gestos/. (4) proto/: esquema único de mensajes; se generan los tipos de Go y TS. Cómo se hablan: la webview emite eventos de gesto al núcleo Go; el controlador manda a cada receptor comandos de pocos bytes por WebSocket; los archivos viajan directo de origen a destino con el protocolo de LocalSend. ./dev sim levanta un controlador y N receptores falsos en local.

## Piezas

| Capa | Pieza | Qué es | Tecnología | Tareas |
| --- | --- | --- | --- | --- |
| ve | Cámara y gestos | La ventana que mira tu mano con la cámara | Wails + MediaPipe | cotejo-cphace |
| hace | Clasificador de gestos | Convierte la posición de la mano en uno de los 4 gestos | | cotejo-tw9i76 |
| hace | Núcleo | Revisa qué equipo puede hacer qué y reparte las órdenes | Go | cotejo-f09uwv |
| hace | Adaptadores de Linux | Hacen la captura, el portapapeles y abrir apps en Hyprland | Go | cotejo-42s7rt |
| hace | Adaptadores de Windows | Hacen la captura, el portapapeles y abrir apps en Windows | Go | cotejo-232tso |
| guarda | Catálogo de apps | Unas 30 apps comunes con su nombre en cada sistema | JSON | cotejo-qxb94p |
| guarda | Gestos grabados | Las grabaciones de cada gesto con que se prueba el clasificador | | cotejo-x2hyu0 |
| conecta | Emparejamiento | Encuentra tus otros equipos en la red y los une con un código de 6 dígitos | mDNS + TLS | cotejo-1od1ta |
| conecta | Órdenes entre equipos | Los mensajes cortos que van de un equipo a otro | WebSocket | cotejo-f09uwv, cotejo-sb2kdm |
| conecta | Envío de archivos | Pasa capturas y archivos directo de un equipo al otro | LocalSend | cotejo-sb2kdm |

## Cómo se abre

- comando: `./dev app`
- url:
- qué esperar: en Linux se abre una ventana de Lazul con la cámara, los puntos de la mano, un indicador ACTIVO/INACTIVO, la cuenta atrás sin mano y el último gesto; la L alterna el modo táctil y los gestos controlan scroll, zoom y diapositivas en la ventana con foco. En Windows conserva el modo de transferencia. La cámara permanece encendida mientras la ventana esté abierta y la app escucha en el puerto 53317. Para el otro PC Windows: `./dev app win` desde este Linux.
