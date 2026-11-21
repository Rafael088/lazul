# Toque con área de contacto (cotejo-6eg99p, 21-nov-2026)

**Pregunta:** si el dedo virtual llega con el área de un dedo de verdad
(~30-40 px), ¿el navegador o el sistema llevan solo al botón o enlace chico
un toque que cae casi encima?

**Respuesta: el código ya manda el área en los dos sistemas, pero todavía no
está medido en ningún navegador.** El sandbox de esta tarea no tiene
`/dev/uinput`, no llega por `ssh win` al PC Windows y no deja arrancar
Chromium. La tabla de abajo es lo que espero y por qué. La página
`herramientas/toque-contacto.html` dice qué área recibe el navegador y si el
toque cayó en el blanco, así que llenarla es un paseo de 10 minutos con la
cámara.

## Qué cambió

- `platform/platform.go`: `DiametroContacto = 36`, el lado del dedo en
  píxeles lógicos (los CSS a escala 1). Es la única constante que hay que
  tocar para ajustarlo.
- `platform/linux/tactil.go`: el táctil uinput declara `ABS_MT_TOUCH_MAJOR`
  y `ABS_MT_TOUCH_MINOR` (rango 0..2880, las mismas unidades que X/Y) y los
  manda **al apoyar** cada dedo. Las unidades salen del ancho lógico del
  monitor que recibe el táctil, porque Hyprland estira el rango sobre esa
  salida: son 72 en la principal (2880 px a escala 2 = 1440 lógicos) y 54 en
  una de 1920 a escala 1. `Target` le pasa ese ancho a `Recrear`.
- `platform/windows/tactil.go`: `rcContact` de `POINTER_TOUCH_INFO` pasa de
  ±2 px a ±18 px lógicos, en píxeles físicos según el DPI efectivo del monitor
  donde se apoya el dedo (`MonitorFromPoint` + `GetDpiForMonitor`): ±18 a
  100 %, ±27 a 150 %, ±36 a 200 %. Si `shcore.dll` no está, se toma el 100 %.
- El área se fija al apoyar y no cambia hasta levantar el dedo (también en el
  latido de 100 ms y en el `UP`). Con eso el arrastre, el scroll y el zoom con
  dos dedos ven el mismo contacto de siempre: la posición no cambia, solo el
  tamaño del dedo. Las pruebas `TestTactilAreaDeContacto` (Linux) y
  `TestEstadoTactilAreaDeContacto` (Windows) lo comprueban.

## Qué espero en cada navegador (sin medir)

| Sistema | Navegador | ¿Corrige? | Por qué |
| --- | --- | --- | --- |
| Linux (Hyprland) | Chromium | **no, creo** | libinput no da el tamaño de los toques de pantalla táctil (`libinput_event_touch_*` solo da posición, ranura y hora), así que Hyprland no tiene qué mandar en `wl_touch.shape`. El navegador recibe un dedo sin área. |
| Linux (Hyprland) | Firefox | igual que sin área | Firefox no usa el área del dedo: corrige con un radio propio en mm (`ui.touch.radius.*` en about:config) si `ui.touch.radius.enabled` está activo. Hay que mirarlo ahí. |
| Windows | Edge | **sí, creo** | Chromium lee `rcContact` del `WM_POINTER` como ancho y alto del toque, y el *touch adjustment* de Blink busca el blanco dentro de esa área. |
| Windows | Chromium | **sí, creo** | El mismo motor que Edge. |
| Windows | Firefox | igual que sin área | Igual que en Linux: su radio propio, no el del dedo. |

En Linux la parte de uinput queda declarada (no estorba: es lo que mandan las
pantallas táctiles reales) y sirve el día que libinput o Hyprland pasen el
área, o con apps que lean evdev directo. Un riesgo a mirar en la prueba: que
libinput tome un contacto de 72 unidades por palma. Para pantallas táctiles
solo lo hace con un *quirk* `AttrPalmSizeThreshold`, que este dispositivo no
tiene.

## Cómo medirlo

1. `./dev app` (Linux) o `./dev app win` (Windows), con el panel calibrado.
2. Abrir `herramientas/toque-contacto.html` en cada navegador, al 100 % de
   zoom.
3. «Área recibida» dice qué llegó: `1×1` o `0×0` es un dedo sin área; ~36 es
   el área nueva. En Firefox, anotar también `ui.touch.radius.enabled`.
4. Tocar 10 veces a 5-15 px de cada blanco, sin acertarle. Los que se marcan
   en verde son toques corregidos. Anotar cuántos de cada 10.
5. Repetir un arrastre, un scroll y un zoom con dos dedos en cualquier página
   larga para ver que no cambian.

| Sistema | Navegador | Área recibida | Corregidos / 10 | Arrastre, scroll y zoom |
| --- | --- | --- | --- | --- |
| Linux | Chromium | | | |
| Linux | Firefox | | | |
| Windows | Edge | | | |
| Windows | Chromium | | | |
| Windows | Firefox | | | |

## Pruebas

`./dev check`: todo bien (formato, vet, compilar, pruebas, pista, proto y
tipos-ts). Las pruebas de `platform/windows` compilan con
`GOOS=windows go test -c`, pero aquí no se pudieron correr (`./dev win test`
no llega al PC).
