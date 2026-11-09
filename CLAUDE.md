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
| `./dev contrato` | contrato de `platform` contra el adaptador de este sistema (necesita escritorio) |
| `./dev sim [-receptores N] [-mdns]` | controlador + N receptores falsos: descubrir (mDNS en memoria; `-mdns` usa la red), rechazar al no emparejado, código erróneo y caducado, emparejar con permiso, intruso e impostor rechazados, Órdenes por TLS y, al final, de punta a punta con las grabaciones reales (captura por LocalSend, gesto 4 por niveles) |
| `./dev e2e win\|linux [args]` | de punta a punta entre los 2 PCs de verdad: `win` controla este Linux y recibe Windows (túnel de ssh: el cortafuegos de Windows no deja entrar), `linux` al revés por la red local (puerto 53317). `LAZUL_ABRIR=1` abre de verdad; args del controlador: `-adelantar`, `-intentos N`, `-casos`, `-ventanas` |
| `./dev replay [-v] [-barrer]` | precisión y falsos positivos del clasificador: grabaciones reales (`fixtures/gestos`) y sintéticas, por separado; `-barrer` prueba varios tiempos de sostén de la L |
| `./dev grabar` | página en 127.0.0.1:8137 para grabar tomas con la cámara (necesita `prototipos/camara/traer-assets.sh`) |
| `./dev gen` | regenera `proto/go` y `proto/ts/src` desde `proto/lazul/v1/lazul.proto` |
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
| `platform/platform.go` | interfaces Screenshot, Clipboard, ActiveApp, LaunchApp y errores | pequeñas; una por capacidad |
| `platform/contrato/` | pruebas de contrato que todo adaptador pasa | sus meta-tests exigen que `vacio` falle todo y `falso` pase todo |
| `platform/linux/`, `platform/windows/` | adaptadores, un archivo por capacidad | hoy devuelven `ErrNoImplementado`: terminados cuando `./dev contrato` / `./dev win contrato` pasan |
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
| `gestures/` | `PoseDe` (forma de la mano en un cuadro, `pose.go`), `Clasificar` (máquina de estados pura, `clasificador.go`), grabaciones y medición | formato en `grabacion.go`; umbrales provisionales hasta medir con las reales |
| `gestures/sintetica_test.go` | modelo sintético de la mano que genera `gestures/testdata/sinteticas` | tras cambiarlo: `go test ./gestures -run TestSinteticasAlDia -actualizar` |
| `gestures/cmd/replay/`, `gestures/cmd/grabar/` | `./dev replay` y `./dev grabar` | |
| `fixtures/gestos/` | grabaciones reales `*.jsonl` | las sintéticas van en `gestures/testdata/sinteticas` |
| `desktop/` | app Wails (vacía; la base está en `prototipos/camara`) | |
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

## Cómo se habla todo

webview (MediaPipe) → puntos → `gestures.Clasificar` → `EventoGesto` → núcleo Go →
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
- Leer la URL del navegador y el documento abierto en los adaptadores (niveles 1 y 2 del gesto 4).
- Abrir en pantalla ya no frena la Orden: el adaptador lanza (`xdg-open`, `ShellExecute`) y contesta ok tras 25–50 ms de gracia (`esperaLanzar`); un fallo inmediato (comando o archivo inexistente, código de ShellExecute ≤ 32) sí se devuelve; uno que llega después se pierde. Medido con `LAZUL_ABRIR=1 ./dev e2e`: la Orden Entregar baja de 260–545 ms a ~20–60 ms (Windows: la búsqueda en frío del ejecutable puede sumar hasta ~200 ms la primera vez).

## Reglas

- Repo público (licencia FSL-1.1-ALv2, `LICENSE.md`): nada de secretos, rutas ni datos personales.
- Seguridad, cifrado y emparejamiento no se delegan a modelos baratos (dec-obus).
- Código y mensajes en español; nombres de las interfaces de platform en inglés.
