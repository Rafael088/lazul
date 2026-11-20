# Lazul — mapa para agentes

Lazul convierte una webcam en el control de los equipos de la red local: con
gestos de la mano agarras una captura, texto, una URL o una app en un equipo y
la sueltas en otro. Todo en local. La 0.1.0: 2 PCs (Windows y Linux Hyprland),
gestos 1 a 4, precisión ≥ 95 % y latencia de comando < 100 ms.

## Un solo punto de entrada: `./dev`

| Comando | Qué hace |
| --- | --- |
| `./dev check` | formato, vet, compilar, pruebas, proto al día, tipos TS. Debe quedar en verde antes de entregar. |
| `./dev test [paquetes]` | pruebas de Go (por defecto `./...`) |
| `./dev contrato` | contrato de `platform` contra el adaptador de este sistema (necesita escritorio); con `LAZUL_CONTRATO_ABRIR=1` también «en vivo»: abre cada navegador y visor instalado con perfil y archivos temporales (Linux: en el espacio 6 sin foco) y comprueba la URL y el documento leídos |
| `./dev sim [-receptores N] [-mdns]` | controlador + N receptores falsos: descubrir (mDNS en memoria; `-mdns` usa la red), rechazar al no emparejado, código erróneo y caducado, emparejar con permiso, intruso e impostor rechazados, Órdenes por TLS y, al final, de punta a punta con las grabaciones reales (captura por LocalSend, gesto 4 por niveles) |
| `./dev e2e win\|linux [args]` | de punta a punta entre los 2 PCs de verdad: `win` controla este Linux y recibe Windows (túnel de ssh: el cortafuegos de Windows no deja entrar), `linux` al revés por la red local (puerto 53317). `LAZUL_ABRIR=1` abre de verdad; args del controlador: `-adelantar`, `-intentos N`, `-casos`, `-ventanas` |
| `./dev replay [-v] [-barrer|-tactil|-toques]` | precisión y falsos positivos del clasificador: grabaciones reales (`fixtures/gestos`) y sintéticas, por separado; `-barrer` prueba varios tiempos de sostén de la L; `-tactil` mide el modo táctil; `-toques` mide los toques de `fixtures/pellizco` (embrague y `ToquesDe`): tap, pulsación larga, arrastre y pellizcos descartados por empezar fuera del panel; la última línea, `PRECISIÓN`, es la métrica de cotejo-uy84og (aciertos, falsos, de más, bordes y salto al apoyar, por el mismo recorrido que la app; `gestures/precision_toques.go`) |
| `./dev grabar` | página en 127.0.0.1:8137 para grabar tomas con la cámara (necesita `prototipos/camara/traer-assets.sh`): los gestos 0.1 y las tomas del modelo nuevo de manipulación directa (pellizco y arrastre, entrar/salir del panel, zoom a dos manos y movimiento normal) |
| `./dev pista` | abre en el navegador la pista local de precisión: 14 blancos de distintos tamaños en centro, bordes y esquinas, seguidos de una zona de reposo; descarga el resumen JSON sin datos de cámara |
| `./dev gen` | regenera `proto/go` y `proto/ts/src` desde `proto/lazul/v1/lazul.proto` |
| `./dev app [args]` | la app de escritorio (en Linux con gráfica NVIDIA, MediaPipe corre en ella por EGL; `LAZUL_GPU=intel` para la integrada); en Linux abre el modo táctil (scroll, zoom y diapositivas) y `-modo transferencia` conserva el recorrido entre PCs; `./dev app win` la copia a `%USERPROFILE%\Lazul`; `./dev app resumen` dice las activaciones de los últimos 30 min; `./dev app compilar`; `./dev app e2e win\|linux` prueba la transferencia con grabaciones |
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
| `platform/platform.go` | interfaces Screenshot, Clipboard, ActiveApp, LaunchApp, TactileActions, AbsolutePointer, TouchScreen, Displays (monitores: listar y dirigir táctil y puntero a uno) y errores; `pantallas.go`: vecino por lado y mapeo por monitor, puros | pequeñas; una por capacidad |
| `platform/contrato/` | pruebas de contrato que todo adaptador pasa | sus meta-tests exigen que `vacio` falle todo y `falso` pase todo |
| `platform/linux/`, `platform/windows/` | adaptadores, un archivo por capacidad | Linux añade acciones táctiles, puntero absoluto y pantalla táctil multitoque uinput; Windows deja TouchScreen como no soportado para activar el respaldo; terminados cuando `./dev contrato` / `./dev win contrato` pasan |
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
| `desktop/` | lógica de la app de escritorio, con pruebas: zona de torso MediaPipe Pose + `Panel` (calibración persistente, puntero absoluto y pellizco → TouchScreen, con respaldo de botón/teclado), `Tactil` (cuadro → acción local, apagado sin mano y registro), `Indicador` (modo táctil activo/inactivo y destello por gesto), `Vivo` (transferencia anterior), `Registro` y `Resumir` (activaciones falsas/evitadas con el motivo de cada salida del panel — fuera_borde, cerca o lejos — y costo fps/CPU, JSONL en la carpeta de estado), `Nucleo` (receptor en :53317 + controlador con el Enrutador, `destinos.json`) | el registro nunca lleva imágenes, posiciones, URL ni rutas (una prueba lo exige); cómo usarla: `desktop/README.md` |
| `desktop/app/` | la ventana Wails (módulo aparte: cgo y WebKitGTK); abre como indicador pequeño y la vista de cámara es opcional; cámara por ffmpeg en Linux y getUserMedia en Windows; MediaPipe en la webview (`traer-assets.sh`, no se versiona) | fuera de `./dev check`; compila con `./dev app compilar` |
| `herramientas/` | scripts de apoyo (`win-escritorio.ps1`) | |
| `prototipos/camara/` | prueba de cámara medida (módulo aparte) | no romper; fuera de `./dev check` |
| `.cotejo/` | tareas, decisiones, arquitectura (Cotejo) | se escribe con el CLI `cotejo`, no a mano |

## Gestos (dec-rwrs)

`Clasificar` arranca en reposo; solo la L sostenida (400 ms) activa. En
activo (5 s), el puño agarra y la pinza arrastrada manda la app; agarrando,
abrir la mano moviéndola a un lado suelta. La dirección es la de la persona
(la imagen no se espeja). La L se separa de la pinza (puntas juntas) y de
señalar (pulgar pegado, a medias o perdido) por el pulgar: lejos del índice,
a unos 90° y con las puntas separadas. El puño exige el índice de verdad
recogido (a medio doblar no lo es) y que el sostén de 150 ms sea al menos 70 %
puño: dos cuadros sueltos unidos por la gracia no agarran.

## Modo táctil (dec-gzgl)

En Linux, `./dev app` permite calibrar dos esquinas de un panel imaginario. La
palma solo cuenta si está dentro del panel y del torso hombros-caderas que
MediaPipe Pose ve en el mismo cuadro (sin caderas visibles, sentado, el torso se estima
bajando 1,25 anchos de hombro; calibrando, la mano no pasa por este filtro; con el panel
ya calibrado, Pose solo exige ver los dos hombros y la zona es el panel); entonces mueve un puntero absoluto uinput
sin botón, como anticipo del contacto. Su tamaño aparente compensa la profundidad
y un filtro One Euro quita temblor. La calibración se guarda en `panel.json`; su tercer paso («junta los dedos») guarda la apertura del pellizco del dueño y `AjustesPellizcoPara` fija a su mano los umbrales de cerrar (por defecto 0,20) y firme (0,12: apoya desde el 1.er cuadro; el tibio espera 3).
Con dos monitores, `desktop.Pantallas` decide cuál controla el panel: el del
foco al arrancar (y lo sigue si el foco cambia); un barrido rápido de una mano
abierta hacia un lado (`gestures.DetectorBarridoPantalla`: ≥ 2,5 palmas en
350 ms, horizontal, sin vaivén ni saltos ni pinza; 0 disparos en todas las
grabaciones reales) pasa al vecino de ese lado con `platform.Displays.Target`
y mueve el aura. Linux: `hyprctl eval` con `hl.device` (salida del táctil),
`hl.dsp.focus` y el puntero llevado al rectángulo del monitor; Windows: toques
y puntero en las coordenadas de ese monitor. Un solo monitor: nada cambia.
El pellizco pulgar-índice pasa cuadro a cuadro por `TraductorToques` y apoya,
mueve o levanta un dedo de `platform.TouchScreen`; dos manos son dos dedos y
la app hace su zoom nativo. Cada ranura tiene su filtro One Euro. Si TouchScreen
no se prepara, `Panel` conserva el botón principal y Ctrl +/-, lo muestra y lo
registra sin coordenadas. Parar o recalibrar levanta todas las ranuras. Además,
cada cuadro pasa por `ClasificarTactil`: la L
sostenida alterna el modo; el agarre (pulgar, índice y medio con las puntas
juntas) hace scroll continuo con la rueda virtual mientras se sostiene, según
cuánto se aleje la mano del punto de agarre; una pinza que abre/cierra
hace zoom; dos dedos en horizontal mandan Page Up/Page Down para las
diapositivas. Las acciones se aplican localmente a la ventana con foco mediante
`platform.TactileActions`. El modo se apaga tras 3 s sin mano. Cada gesto y su
resultado se anota en `factibilidad.jsonl`, sin contenido de cámara o pantalla.

## Cómo se habla todo

webview (MediaPipe Hands + Pose) → filtro torso+panel → `gestures.Clasificar` → `EventoGesto` → núcleo Go →
`Sobre{Orden}` por WebSocket sobre TLS 1.3 (huella fijada en los dos lados) →
receptor → `platform` → `Sobre{Respuesta}`. Antes, una vez por par de equipos:
mDNS → `/pareja` con el código de 6 dígitos → la persona en el receptor permite.
Los archivos (capturas) no van en el Sobre: viajan por el protocolo de LocalSend
(`core/localsend`, mismo puerto TLS) y después `Entregar{Archivo}` con su sha256;
el receptor solo lo acepta si llegó entero.

## Pendiente en otras tareas

- Adaptadores: Linux `cotejo-42s7rt`, Windows `cotejo-232tso` (hacer pasar el contrato; no tocar `platform/contrato` para que pase).
- Grabaciones reales `cotejo-x2hyu0` (con `./dev grabar`; después ajustar umbrales y el sostén de la L con `./dev replay -barrer`); catálogo de apps `cotejo-qxb94p`.
- Precisión real 93,9 % (< 95 %): más tomas o ajustar con las que fallan (informe de `cotejo-sb2kdm`).
- Niveles 1 y 2 del gesto 4 hechos (`cotejo-82qb85`, `niveles.go` de cada adaptador); quedan sus límites. Windows: URL por UI Automation (Chrome y Edge probados; Firefox y Brave sin probar, no están en el PC), documento si su ruta va en la línea de órdenes o en el título (doble clic sí; abierto desde el menú de la app no; Office por COM no). Linux: URL leída de la sesión que el navegador ya guarda (sin depuración remota ni cambiar nada): Chromium y Edge con hasta ~2,5 s de atraso, Firefox y Zen hasta 15 s, y solo si el título guardado es el de la ventana (privadas: nunca); documento por título, argumentos o `/proc/<pid>/fd` (imv, evince, mpv probados). Flatpak/snap sin probar.
- Abrir en pantalla ya no frena la Orden: el adaptador lanza (`xdg-open`, `ShellExecute`) y contesta ok tras 25–50 ms de gracia (`esperaLanzar`); un fallo inmediato (comando o archivo inexistente, código de ShellExecute ≤ 32) sí se devuelve; uno que llega después se pierde. Medido con `LAZUL_ABRIR=1 ./dev e2e`: la Orden Entregar baja de 260–545 ms a ~20–60 ms (Windows: la búsqueda en frío del ejecutable puede sumar hasta ~200 ms la primera vez).

## Reglas

- Repo público (licencia FSL-1.1-ALv2, `LICENSE.md`): nada de secretos, rutas ni datos personales.
- Seguridad, cifrado y emparejamiento no se delegan a modelos baratos (dec-obus).
- Código y mensajes en español; nombres de las interfaces de platform en inglés.
