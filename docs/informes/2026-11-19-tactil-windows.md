# Táctil virtual en Windows (cotejo-ckwoq7, 19-nov-2026)

**Pregunta:** ¿sale en Windows lo mismo que en Linux (dec-yp1a)? El pellizco
armado apoya un dedo, moverlo arrastra, soltarlo lo levanta, dos manos son dos
dedos y el puntero sigue a la palma.

**Respuesta: sí.** `platform/windows` ya tiene `TouchScreen` con
`InjectTouchInput` (sin pantalla táctil en el PC), `AbsolutePointer` y
`TactileActions` con `SendInput`. Con eso, `./dev app win` arranca en modo
táctil como en Linux (antes arrancaba en modo de transferencia), con la cámara,
el panel y el aura.

PC de prueba: Ryzen 5 7520U, Windows con pantalla de 1920×1080 al 150 %,
Edge. Todo lo de abajo se inyectó por `ssh win` en la sesión del escritorio,
sin nadie tocando nada.

## Qué cambió

- `platform/windows/tactil.go`: hasta 2 contactos. Cada cuadro de la app se
  vuelve un `InjectTouchInput` con un contacto por dedo: apoyar (`DOWN`),
  mover (`UPDATE`) y levantar (`UP`) en la última posición. Un dedo quieto se
  vuelve a mandar cada 100 ms por si la cámara tarda en dar el cuadro
  siguiente. Las coordenadas van en píxeles físicos de la pantalla principal
  aunque el proceso no declare DPI.
- `platform/windows/puntero.go`: el puntero absoluto (0–65535 sobre la
  pantalla principal, la misma de los toques).
- `platform/windows/acciones_tactiles.go`: respaldo y gestos sueltos como en
  Linux (flechas, Ctrl + «+»/«-», Re Pág/Av Pág) y la rueda continua.
- `desktop/app/aura_windows.go`: el aura es una ventana por capas que no
  recibe clics, toques ni foco y no sale en la barra de tareas. La figura es
  la de `aura.html` dibujada en Go (`aura_pintar.go`), una vez por estado;
  moverla es solo `UpdateLayeredWindow`.
- `-modo auto` es táctil también en Windows.
- Arreglada una prueba que fallaba solo en Windows (`desktop`: Windows no
  tiene bits de permiso).

## Resultados en Edge

Prototipo: `prototipos/tactil-win/` (`correr.sh guion.txt`). Usa el mismo
`TouchScreen` que la app. La página de prueba anota en el título lo que
recibe, y el guion lo lee por CDP.

| Toque inyectado (igual que en Linux) | Edge 2026 |
| --- | --- |
| tap (60 ms) | `pointerdown` tipo `touch` + `click`, en el píxel pedido |
| doble (2 × 60 ms, 120 ms entre) | `dblclick` |
| largo (900 ms) | `contextmenu` (Edge lo da al levantar: 923 ms) |
| arrastre de 0,85 a 0,2 en 300 ms | scroll 0 → 925 px, con inercia; de vuelta, 925 → 20 |
| pellizco de 5 % a 30 % en 700 ms | zoom nativo 1,00 → 3,07; al cerrarlo, 1,08 |
| un dedo apoyado 3 s con **un solo** cuadro | sigue apoyado (lo sostiene el reenvío) y abre el menú de contexto |
| dedo apoyado mientras el puntero se mueve (lo que hace la app) | no se cancela; sale `click` |
| tap justo debajo del aura encendida | lo recibe la página de atrás |
| dos dedos (contrato) | Edge lo toma como clic derecho (`contextmenu`) |

## Resultados en un PDF (visor de Edge)

`prototipos/tactil-win/prueba.pdf`, 6 páginas, comparando capturas antes y
después:

- **Arrastre:** baja de la página 1 a la 2.
- **Pellizco:** zoom nativo del documento (aparece la barra horizontal);
  cerrarlo lo reduce otra vez.
- **Doble toque y pulsación larga:** sin cambio visible en el visor (no hay
  menú de contexto al tocar).

## Rendimiento

| Medida | Windows (este informe) | Linux (18/19-nov) |
| --- | --- | --- |
| cámara, sin mano en el panel | 16–20 cuadros/s (webview, `getUserMedia`) | 26–30 (NVIDIA) |
| CPU de la app (árbol de procesos) | 37–47 % de un núcleo (5 % del total) | 126–183 % de un núcleo (NVIDIA), ~45 % (Intel) |
| inferencia por cuadro | mano 12–23 ms + Pose 4,5 ms (GPU) | — |
| inyectar toques a 30 cuadros/s | 0,47 % de un núcleo | — |

Son las líneas `cámara: …` que la app escribe cada 5 s (`./dev app win -salida
app-salida.txt -duracion 75s`), en 75 s con la cámara abierta y algunos
cuadros con una mano a la vista, pero sin panel calibrado. La cámara de este PC
no pasa de unos 20 cuadros/s con esta luz; ya se había visto 16,6 en dec-7tgj.

## Pruebas

- `./dev check` en verde; `./dev win test` en verde.
- `./dev win contrato`: 18/21. Las 3 que se saltan son las que inyectan
  (`LAZUL_CONTRATO_ABRIR=1`). Esas pasaron aparte, encima de la página de
  prueba (`TouchScreen/dos_dedos`, `TactileActions/acciones_validas`).
- `desktop/app`: `TestPintarAura` (en los dos sistemas) y `TestAuraVivo`
  (Windows, escritorio): el aura se ve, `WindowFromPoint` no la encuentra y un
  tap debajo llega a la ventana de atrás.

## Pasos para el dueño (con la mano frente a la cámara del PC Windows)

1. Desde el Linux: `./dev app win`. Sale la píldora abajo a la derecha.
2. Abre la vista de cámara (botón o Ctrl+Mayús+C), pulsa calibrar y captura
   las dos esquinas del panel. Queda en `%LOCALAPPDATA%\lazul\panel.json` (en
   este PC todavía no había ninguna).
3. Cierra la vista: con la palma dentro del panel se ve el aura y el puntero la
   sigue.
4. En Edge: tap en un enlace, doble toque, pulsación larga (menú), pellizcar y
   mover para hacer scroll, dos manos pellizcando para el zoom.
5. Lo mismo en un PDF abierto en Edge.
6. Al cerrar la app, `./dev app resumen` no aplica (el registro está en
   Windows): revisa las líneas `cámara:` y `panel` de
   `%LOCALAPPDATA%\lazul\factibilidad.jsonl` para ver los cuadros por segundo
   y la CPU con la mano dentro.

## Límites

- Los toques van a la **pantalla principal**. Con varios monitores, el panel
  cubre solo la principal (en Linux, Hyprland asigna el táctil a un monitor).
- Windows dibuja su propio circulito donde cae el dedo
  (`TOUCH_FEEDBACK_DEFAULT`), además del aura.
- `Screenshot` (sin cambios aquí) captura en píxeles lógicos si el proceso no
  declara DPI. Al 150 % sale solo la esquina superior izquierda (1280×720 de
  1920×1080). La app declara DPI (Wails llama a `SetProcessDPIAware`) y no lo
  sufre; el prototipo lo declara a mano.
