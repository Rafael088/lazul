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
| `./dev sim [-receptores N]` | controlador + N receptores falsos por WebSocket en 127.0.0.1 |
| `./dev replay [-dir D] [-v]` | precisión y falsos positivos del clasificador sobre `fixtures/gestos` |
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
| `core/` | núcleo: `Receptor` atiende Órdenes, transporte WebSocket | nunca llama al sistema, solo a `platform` |
| `core/sim/`, `core/cmd/sim/` | el recorrido de `./dev sim` | |
| `gestures/` | tipos de la mano, `Clasificar` (función pura), grabaciones y medición | formato de grabación en `grabacion.go` |
| `gestures/cmd/replay/` | `./dev replay` | |
| `fixtures/gestos/` | grabaciones reales `*.jsonl` | las sintéticas van en `gestures/testdata` |
| `desktop/` | app Wails (vacía; la base está en `prototipos/camara`) | |
| `herramientas/` | scripts de apoyo (`win-escritorio.ps1`) | |
| `prototipos/camara/` | prueba de cámara medida (módulo aparte) | no romper; fuera de `./dev check` |
| `.cotejo/` | tareas, decisiones, arquitectura (Cotejo) | se escribe con el CLI `cotejo`, no a mano |

## Cómo se habla todo

webview (MediaPipe + `gestures.Clasificar`) → `EventoGesto` → núcleo Go →
`Sobre{Orden}` por WebSocket → receptor → `platform` → `Sobre{Respuesta}`.
Los archivos (capturas) no van en el Sobre: viajan por LocalSend.

## Pendiente en otras tareas

- Adaptadores: Linux `cotejo-42s7rt`, Windows `cotejo-232tso` (hacer pasar el contrato; no tocar `platform/contrato` para que pase).
- Clasificador `cotejo-tw9i76`; grabaciones `cotejo-x2hyu0`; catálogo de apps `cotejo-qxb94p`.
- Emparejamiento, TLS y permisos `cotejo-1od1ta` (hoy el transporte es WebSocket sin cifrar, solo en 127.0.0.1).
- De punta a punta y LocalSend `cotejo-sb2kdm`.

## Reglas

- Repo público (licencia FSL-1.1-ALv2, `LICENSE.md`): nada de secretos, rutas ni datos personales.
- Seguridad, cifrado y emparejamiento no se delegan a modelos baratos (dec-obus).
- Código y mensajes en español; nombres de las interfaces de platform en inglés.
