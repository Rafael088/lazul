# Lazul — mapa para agentes

Lazul convierte la pantalla de un PC Windows o Linux (Hyprland) en una pantalla
táctil que se maneja en el aire, con la webcam y sin hardware extra. Es para
cuando no tienes el ratón a mano (clase, presentar, PC lejos); no promete
reemplazarlo. La 0.2.0 pasa con los 8 criterios de dec-uaex (`cotejo leer dec-uaex`),
límite 2026-12-07; antes de eso, ni landing ni video. Agarrar y soltar entre
equipos (0.1.0) sigue con `-modo transferencia`: no es el núcleo ni se promete
todavía (dec-0j4m). El video y los puntos de la mano nunca salen del equipo.

## Un solo punto de entrada: `./dev`

| Comando | Qué hace |
| --- | --- |
| `./dev check` | formato, vet, compilar, pruebas, proto al día, tipos TS. Debe quedar en verde antes de entregar. |
| `./dev test [paquetes]` | pruebas de Go (por defecto `./...`) |
| `./dev contrato` | contrato de `platform` contra el adaptador de este sistema (necesita escritorio); con `LAZUL_CONTRATO_ABRIR=1` también «en vivo»: abre cada navegador y visor instalado con perfil y archivos temporales (Linux: en el espacio 6 sin foco) y comprueba la URL y el documento leídos |
| `./dev sim [-receptores N] [-mdns]` | controlador + N receptores falsos: descubrir (mDNS en memoria; `-mdns` usa la red), rechazar al no emparejado, código erróneo y caducado, emparejar con permiso, intruso e impostor rechazados, Órdenes por TLS y, al final, de punta a punta con las grabaciones reales (captura por LocalSend, gesto 4 por niveles) |
| `./dev e2e win\|linux [args]` | de punta a punta entre los 2 PCs de verdad: `win` controla este Linux y recibe Windows (túnel de ssh: el cortafuegos de Windows no deja entrar), `linux` al revés por la red local (puerto 53317). `LAZUL_ABRIR=1` abre de verdad; args del controlador: `-adelantar`, `-intentos N`, `-casos`, `-ventanas` |
| `./dev replay [-v] [-barrer|-tactil|-toques|-pantalla]` | precisión y falsos positivos del clasificador: grabaciones reales (`fixtures/gestos`) y sintéticas, por separado; `-barrer` prueba varios tiempos de sostén de la L; `-tactil` mide el modo táctil; `-pantalla` mide los tres dedos (cambiar de pantalla) en `fixtures/gestos` y `fixtures/pellizco`; `-toques` mide los toques de `fixtures/pellizco` (embrague y `ToquesDe`): tap, pulsación larga, arrastre y pellizcos descartados por empezar fuera del panel; la última línea, `PRECISIÓN`, es la métrica de cotejo-uy84og (aciertos, falsos, de más, bordes y salto al apoyar, por el mismo recorrido que la app; `gestures/precision_toques.go`) |
| `./dev grabar` | página en 127.0.0.1:8137 para grabar tomas con la cámara (necesita `prototipos/camara/traer-assets.sh`): los gestos 0.1 y las tomas del modelo nuevo de manipulación directa (pellizco y arrastre, entrar/salir del panel, zoom a dos manos (ya sin gesto, se conservan las tomas), movimiento normal y tres dedos a cada lado) |
| `./dev pista` | abre en el navegador la pista local de precisión: 14 blancos de distintos tamaños en centro, bordes y esquinas, seguidos de una zona de reposo; descarga el resumen JSON sin datos de cámara |
| `./dev gen` | regenera `proto/go` y `proto/ts/src` desde `proto/lazul/v1/lazul.proto` |
| `./dev app [args]` | la app de escritorio (en Linux con gráfica NVIDIA, MediaPipe corre en ella por EGL; `LAZUL_GPU=intel` para la integrada); en Linux y Windows abre el modo táctil y `-modo transferencia` conserva el recorrido entre PCs; `./dev app win` la copia a `%USERPROFILE%\Lazul`; `./dev app resumen` dice las activaciones de los últimos 30 min; `./dev app compilar`; `./dev app e2e win\|linux` prueba la transferencia con grabaciones |
| `./dev win test\|check\|contrato` | copia el árbol al PC Windows (`ssh win`) y corre ahí; `contrato` corre en la sesión del escritorio |

Salida breve: una línea por paso; el registro completo queda en `.dev/<paso>.log`.
Go 1.27 (`.tool-versions`), Node 24, `protoc` para `./dev gen`.

## Dónde está cada cosa

| Ruta | Qué es | Reglas |
| --- | --- | --- |
| `proto/lazul/v1/lazul.proto` | esquema único de mensajes (Sobre y todo lo que lleva) | nunca renumerar campos; tras cambiarlo, `./dev gen` |
| `proto/go/lazulv1/` | tipos de Go generados | no editar a mano; `compat_test.go` fija los bytes |
| `proto/ts/` | tipos de TS generados (`src/`) y prueba de compatibilidad con Go | no editar `src/` a mano |
| `proto/testdata/sobre_v1.bin` | el mismo Sobre que leen y escriben Go y TS | regenerar con `go test ./proto/go/lazulv1 -actualizar` |
| `platform/platform.go` | interfaces Screenshot, Clipboard, ActiveApp, LaunchApp, TactileActions, AbsolutePointer, TouchScreen, Displays (monitores: listar y dirigir táctil y puntero a uno), Spaces (los tres dedos con un monitor: escritorio o app), TargetFinder (botones y enlaces de la ventana con foco, para el imán) y errores; `pantallas.go`: vecino por lado y mapeo por monitor, puros | pequeñas; una por capacidad |
| `platform/contrato/` | pruebas de contrato que todo adaptador pasa | sus meta-tests exigen que `vacio` falle todo y `falso` pase todo |
| `platform/linux/`, `platform/windows/` | adaptadores, un archivo por capacidad | Linux añade acciones táctiles, puntero absoluto y pantalla táctil multitoque uinput; Windows tiene la pantalla táctil por `InjectTouchInput` (`tactil.go`, hasta 2 dedos); Windows lee los objetivos del imán por UI Automation (`objetivos.go`) y Linux por AT-SPI más la posición de la ventana de Hyprland (`objetivos.go`, `atspi.go`; Chromium necesita `--force-renderer-accessibility`; `accesibilidad.go` enciende `org.a11y.Status.IsEnabled` al arrancar con el imán y nunca lo apaga); terminados cuando `./dev contrato` / `./dev win contrato` pasan |
| `platform/internal/navegador/` | nivel 1 del gesto 4: URL de la sesión de Firefox/Zen (`recovery.jsonlz4`) y de Chromium/Edge (`Sessions/Session_*`, SNSS), y de la barra de direcciones (Windows) | puro, sin sistema; la URL nunca se registra (puede llevar tokens) |
| `platform/internal/documento/` | nivel 2: elige el documento abierto (nombre en el título, mismos tipos que `core/seguro.go`) | una prueba exige las mismas extensiones que el receptor |
| `platform/falso/`, `platform/vacio/` | sistema en memoria (para sim y pruebas) y el que no hace nada | |
| `core/` | núcleo: `Receptor` atiende Órdenes, transporte WebSocket sobre TLS (`transporte.go`) | nunca llama al sistema, solo a `platform` |
| `core/pareja.go`, `identidad.go`, `codigo.go`, `pares.go` | emparejamiento: identidad (cert autofirmado P-256, `Huella` = SHA-256 de la clave pública), código de 6 dígitos (2 min, 3 intentos, un uso), prueba HMAC atada a la sesión TLS, `Puerta` (permiso de la persona) y `Pares` fijados | solo librería estándar; el protocolo y su límite están en el comentario de `pareja.go`; pruebas primero |
| `core/descubre/` | interfaz `Red`: `MDNS` (`_lazul._tcp`, hashicorp/mdns) y `Memoria` (sim y pruebas) | lo anunciado no es de confianza: la da la huella; la prueba con red real va con `LAZUL_MDNS=1` |
| `core/enrutador.go` | el controlador: gesto → Órdenes (agarrar la captura de esta pantalla, soltarla en el otro, gesto 4 por niveles) | lo agarrado sale del equipo de la cámara |
| `core/localsend/` | protocolo LocalSend v2 (info, prepare-upload, upload): los bytes de capturas y archivos | se sirve en el mismo puerto TLS que las Órdenes y solo para emparejados |
| `core/ensayo/` | pasa grabaciones reales por el clasificador y le da los gestos al Enrutador; percentiles | lo usan `./dev sim` y `./dev e2e` |
| `core/sim/`, `core/cmd/sim/` | el recorrido de `./dev sim` | |
| `core/cmd/receptor/`, `core/cmd/controlador/` | receptor y controlador de verdad para `./dev e2e` | `-permitir HUELLA` da el permiso solo a esa huella (para medir sin nadie) |
| `platform/real/` | el adaptador del sistema en que corre | |
| `docs/informes/` | informes de medición (punta a punta: `2026-11-08-punta-a-punta.md`) | |
| `gestures/` | `PoseDe` (forma de la mano en un cuadro, `pose.go`), `Clasificar` (máquina de estados pura, `clasificador.go`), `MedidorEmbrague` y `ToquesDe` (pellizco armado → dedos del táctil virtual, `toques.go`), grabaciones y medición | formato en `grabacion.go`; umbrales provisionales hasta medir con las reales |
| `gestures/sintetica_test.go` | modelo sintético de la mano que genera `gestures/testdata/sinteticas` | tras cambiarlo: `go test ./gestures -run TestSinteticasAlDia -actualizar` |
| `gestures/cmd/replay/`, `gestures/cmd/grabar/` | `./dev replay` y `./dev grabar` | |
| `pista/` | página local para medir aciertos a la primera, fallos, toques fuera, tiempo por blanco y toques falsos en reposo | se abre con `./dev pista`; el resumen puro se prueba con Node dentro de `./dev check`; no lee cámara ni coordenadas |
| `fixtures/gestos/` | grabaciones reales `*.jsonl` de los gestos 0.1 | las sintéticas van en `gestures/testdata/sinteticas` |
| `fixtures/pellizco/` | tomas reales del modelo nuevo: cabecera con `toma` (y `panel` si hay calibración) y el estado del pellizco de cada mano por cuadro | se graban con `./dev grabar`; los puntos crudos de las dos manos van siempre |
| `desktop/` | lógica de la app de escritorio, con pruebas: zona de torso MediaPipe Pose + `Panel` (calibración persistente, puntero absoluto y pellizco → TouchScreen, con respaldo de botón/teclado), `Tactil` (cuadro → acción local, interruptor de la L sin apagado por tiempo y registro), `Indicador` (modo táctil activo/inactivo y destello por gesto), `Vivo` (transferencia anterior), `Registro` y `Resumir` (activaciones falsas/evitadas con el motivo de cada salida del panel — fuera_borde, cerca o lejos — y costo fps/CPU, JSONL en la carpeta de estado), `Iman` (`iman.go`: asistencia de puntería nivel 2 — caché por ventana de los objetivos de `platform.TargetFinder`, freno del puntero sobre los chicos y `AjustarToque`, que no corrige con duda; `-iman=false` lo apaga), `Nucleo` (receptor en :53317 + controlador con el Enrutador, `destinos.json`) | el registro nunca lleva imágenes, posiciones, URL ni rutas (una prueba lo exige); cómo usarla: `desktop/README.md` |
| `desktop/app/` | la ventana Wails (módulo aparte: cgo y WebKitGTK); abre como indicador pequeño y la vista de cámara es opcional; cámara por ffmpeg en Linux y getUserMedia en Windows; MediaPipe en la webview (`traer-assets.sh`, no se versiona); `frontend/seguimiento.mjs` (puro, con pruebas en `./dev check`) sigue una mano, busca la segunda cada ~0,5 s y fija lado y orden de cada mano; `-manos dos` sigue siempre dos (informe 2026-11-21-cpu-linux; no usar `setOptions`: rehace el grafo y pierde la mano) | fuera de `./dev check`; compila con `./dev app compilar` |
| `herramientas/` | scripts de apoyo (`win-escritorio.ps1`) y `toque-contacto.html`, la página que dice qué área de dedo recibe el navegador y si corrige el toque a blancos chicos | |
| `prototipos/camara/` | prueba de cámara medida (módulo aparte) | no romper; fuera de `./dev check` |
| `.cotejo/` | tareas, decisiones, arquitectura (Cotejo) | se escribe con el CLI `cotejo`, no a mano |

## Gestos del modo transferencia (dec-rwrs)

`Clasificar` arranca en reposo; solo la L sostenida (400 ms) activa. En
activo (5 s), el puño agarra y la pinza arrastrada manda la app; agarrando,
abrir la mano moviéndola a un lado suelta. La dirección es la de la persona
(la imagen no se espeja). La L se separa de la pinza (puntas juntas) y de
señalar (pulgar pegado, a medias o perdido) por el pulgar: lejos del índice,
a unos 90° y con las puntas separadas. El puño exige el índice de verdad
recogido (a medio doblar no lo es) y que el sostén de 150 ms sea al menos 70 %
puño: dos cuadros sueltos unidos por la gracia no agarran.

## Modo táctil (dec-fa72, dec-yp1a, dec-uaex)

Es el modo por defecto en Linux y Windows. La L sostenida es un interruptor:
arranca apagado, otra L lo apaga y no hay apagado por tiempo. Apagado, el panel
no mueve el puntero ni toca; los tres dedos tampoco cambian de pantalla.

`./dev app` permite calibrar dos esquinas de un panel imaginario. La
palma solo cuenta si está dentro del panel y del torso hombros-caderas que
MediaPipe Pose ve en el mismo cuadro (sin caderas visibles, sentado, el torso se estima
bajando 1,25 anchos de hombro; calibrando, la mano no pasa por este filtro; con el panel
ya calibrado, Pose solo exige ver los dos hombros y la zona es el panel); entonces mueve un puntero absoluto
sin botón, como anticipo del contacto. Su tamaño aparente compensa la profundidad
y un filtro One Euro quita temblor. La calibración se guarda en `panel.json`; su tercer paso («junta los dedos») guarda la apertura del pellizco del dueño y `AjustesPellizcoPara` fija a su mano los umbrales de cerrar (por defecto 0,20) y firme (0,12: apoya desde el 1.er cuadro; el tibio espera 3).
Con dos monitores, `desktop.Pantallas` decide cuál controla el panel: el del
foco al arrancar (y lo sigue si el foco cambia). Los tres dedos
(`gestures.DetectorTresDedos`, `tres.go`: índice, medio y anular estirados con
pulgar y meñique recogidos, deslizando ~1 palma a un lado desde donde empezó la
pose, sin velocidad mínima; un disparo por pose) pasan al vecino de ese lado con
`platform.Displays.Target` y mueven el aura; con un monitor hacen lo del
sistema con `platform.Spaces` (Hyprland: `hl.dsp.focus({ workspace = "e±1" })`;
Windows: Alt+Tab / Alt+Mayús+Tab). Mientras se ve la pose, esa mano no llega al
panel ni al clasificador. Cada intento va a `factibilidad.jsonl` (`"tipo":"pantalla"`,
`resultado` disparo —`motivo` monitor, espacio, sin_vecino, error— o no —corto,
vertical, salto, dos_manos, ocupado—, con `palmas` y `ms`). `./dev replay -pantalla`
mide aciertos en las tomas `tres-derecha`/`tres-izquierda` y falsos en el resto
(0 en fixtures/gestos y fixtures/pellizco). Linux: `hyprctl eval` con `hl.device` (salida del táctil),
`hl.dsp.focus` y el puntero llevado al rectángulo del monitor; Windows: toques
y puntero en las coordenadas de ese monitor.
El pellizco pulgar-índice pasa cuadro a cuadro por `TraductorToques` y apoya,
mueve o levanta un dedo de `platform.TouchScreen`. **Sin zoom** (decisión del
dueño, 23-nov): un solo dedo a la vez; mientras una mano pellizca, el embrague no
arma la otra. Si TouchScreen no se prepara, `Panel` conserva el botón principal,
lo muestra y lo registra sin coordenadas. Parar, apagar o recalibrar levanta todas las ranuras.

Con las manos que pasan el panel corre además `ClasificarTactil`: la L y, con el
modo encendido, los gestos anteriores al táctil virtual, que siguen activos
(agarre de pulgar, índice y medio = scroll con la rueda virtual; dos dedos en
horizontal = Re Pág/Av Pág; la pinza que abre o cierra ya no hace zoom). Van a
la ventana con foco por `platform.TactileActions`. Cada gesto y su resultado se
anota en `factibilidad.jsonl`, sin contenido de cámara o pantalla.

## Cómo se habla todo

Táctil (todo en el equipo): webview (MediaPipe Hands + Pose) → filtro de torso →
`desktop.Pantallas` (tres dedos) → `Panel.Filtrar` → `desktop.Tactil` (L y gestos)
→ `Panel.Cuadro` (puntero, imán, toques) → `platform`.

Transferencia: webview (MediaPipe Hands + Pose) → filtro torso+panel → `gestures.Clasificar` → `EventoGesto` → núcleo Go →
`Sobre{Orden}` por WebSocket sobre TLS 1.3 (huella fijada en los dos lados) →
receptor → `platform` → `Sobre{Respuesta}`. Antes, una vez por par de equipos:
mDNS → `/pareja` con el código de 6 dígitos → la persona en el receptor permite.
Los archivos (capturas) no van en el Sobre: viajan por el protocolo de LocalSend
(`core/localsend`, mismo puerto TLS) y después `Entregar{Archivo}` con su sha256;
el receptor solo lo acepta si llegó entero.

## Límites conocidos

- Lo pendiente está en Cotejo (`cotejo tareas --proyecto lazul`), no en este mapa.
- El sandbox de los agentes no tiene cámara ni GPU: fps y CPU reales se miden con
  `./dev app` en el escritorio y se leen de `factibilidad.jsonl`
  (`"tipo":"rendimiento"`).
- Transferencia, gesto 4. Windows: URL por UI Automation (Firefox y Brave sin probar); documento solo si su ruta va en la línea de órdenes o en el título (Office por COM no). Linux: URL de la sesión guardada del navegador (Chromium y Edge con hasta ~2,5 s de atraso, Firefox y Zen hasta 15 s; privadas nunca). Flatpak/snap sin probar.
- Transferencia: abrir en pantalla contesta ok tras 25–50 ms de gracia (`esperaLanzar`); un fallo que llega después se pierde.

## Reglas

- Repo público (licencia FSL-1.1-ALv2, `LICENSE.md`): nada de secretos, rutas ni datos personales.
- Seguridad, cifrado y emparejamiento no se delegan a modelos baratos (dec-obus).
- Código y mensajes en español; nombres de las interfaces de platform en inglés.
