# Lazul en el escritorio (prototipo feo)

Una ventana que mira tu mano por la cámara y, con gestos, agarra una captura
de la pantalla de este PC y la suelta en el otro. Sirve para comprobar si la
idea funciona de verdad (dec-q1p7), no para usar todos los días.

La misma app va en los dos PCs. Cada uno puede **recibir** (escucha en la red
local) y **mandar** (si tiene cámara).

## Abrirla

| Dónde | Cómo |
| --- | --- |
| Linux (Hyprland) | en la carpeta del repo: `./dev app` |
| Windows, desde el Linux | `./dev app win` (la copia a `%USERPROFILE%\Lazul\lazul.exe` y la abre en el escritorio de Windows) |
| Windows, después | doble clic en `%USERPROFILE%\Lazul\lazul.exe` |

La primera vez se baja MediaPipe (unos 37 MB) y queda dentro del programa.
En Linux la cámara la abre `ffmpeg` (tiene que estar instalado).

La ventana arranca pequeña, permanece sobre las demás y recuerda su tamaño y
posición. Estos datos se guardan en `ventana.json`, junto al estado de la app
(`~/.local/state/lazul` en Linux o `%LOCALAPPDATA%\lazul` en Windows). En
Wayland Wails no garantiza que el compositor acepte restaurar la posición; el
tamaño sí se conserva.

Deja la ventana **a la vista** (puede ser chica, en una esquina): si queda
escondida en otro espacio o minimizada, el sistema deja de darle cuadros y la
cámara se detiene.

### Hyprland: flotante en todos los espacios

Wails identifica la ventana en Linux con la clase `lazul`. En la configuración
Lua nueva de Hyprland, añade esta regla para hacerla flotante, fijarla a todos
los espacios, dejarla en `360 × 300` y colocarla abajo a la derecha:

```lua
hl.window_rule({
  match = { class = "lazul" },
  float = true,
  pin = true,
  size = { 360, 300 },
  move = { "monitor_w-window_w-12", "monitor_h-window_h-12" },
})
```

Comprobada en Hyprland 0.56.2: la ventana sale flotante, fijada a todos los
espacios, de 360 × 300 y a 12 px de la esquina inferior derecha. Va en un
archivo aparte (`~/.config/hypr/lazul.lua`) cargado con `require("hypr.lazul")`.
`AlwaysOnTop` de Wails por sí solo no sustituye esta regla en Wayland.

## Emparejar los dos PCs (una sola vez)

En el PC que **recibe** (por ejemplo Windows):

1. Pulsa **Mostrar código**. Sale un número de 6 cifras; dura 2 minutos.
2. Anota la **Dirección para el otro equipo** (algo como `192.168.1.16:53317`).

En el PC que **tiene la cámara** (por ejemplo Linux):

3. En **Mandar**, escribe esa dirección (basta la IP) y el código, y pulsa
   **Emparejar**. (O **Buscar en la red** y **Usar esta dirección**.)

Otra vez en el PC que recibe:

4. Aparece arriba «… quiere mandarle cosas a este equipo» con una huella
   (`XXXX-XXXX-XXXX-XXXX`). Comprueba que es la misma que se ve en «Este
   equipo» del PC de la cámara y pulsa **Permitir**.

Listo: en el PC de la cámara, «Mando a» dice el nombre del otro y
**conectado**. Queda guardado: la próxima vez no hay que repetirlo. Para que el
otro también pueda mandarte, empareja al revés.

## Usarla

- **La L** (índice arriba, pulgar al lado, los demás recogidos), quieta un
  momento: el modo pasa a **activo** durante 5 s.
- **Puño** (en activo): agarra una captura de esta pantalla. El modo pasa a
  **agarrando**.
- **Abrir la mano moviéndola a un lado** (agarrando): suelta la captura en el
  otro PC, que la abre en pantalla. «Lo último» dice cuánto tardó.
- **Pinza y arrastrar** (en activo): manda la ventana activa (la página web,
  el documento o la app) al otro PC.

La ventana muestra la cámara con los puntos de la mano, la pose, el modo y el
último gesto con su hora. Lo que llega al otro PC queda también en la carpeta
`recibidos` (`~/.config/lazul/recibidos` o `%APPDATA%\lazul\recibidos`).

## La prueba de las activaciones falsas (30 min)

La meta: menos de 1 activación sin querer en 30 min de trabajo normal.

1. Con la cámara encendida, pulsa **Empezar prueba de trabajo normal**.
2. Trabaja 30 min como siempre (escribir, tomar café, hablar con las manos)
   **sin** hacer gestos a propósito. Mientras dure, no se manda nada a
   ningún equipo y toda activación cuenta como falsa.
3. Pulsa **Terminar prueba** y **Resumen**: dice PASA o NO PASA.

Fuera de la prueba también se cuenta: si se activa sola, pulsa **La última
activación fue sin querer**; si la activaste tú y no hiciste nada después,
**Fue a propósito**. Desde el Linux también: `./dev app resumen`.

Todo queda en un archivo de este PC (`factibilidad.jsonl` en
`~/.local/state/lazul` o `%LOCALAPPDATA%\lazul`): la hora, la pose, los gestos,
las activaciones y en qué terminaron. **Nunca** guarda imágenes de la cámara ni
de la pantalla, ni direcciones web, ni nombres de archivos o ventanas.

## Si algo no va

- **«no puedo escuchar en :53317»**: otra app usa ese puerto (por ejemplo
  LocalSend abierto). Ciérrala y vuelve a abrir Lazul.
- **No conecta con Windows**: el cortafuegos de Windows solo deja entrar por
  el puerto 53317 en una red **privada**.
- **La cámara va lenta en Linux** (menos de 15 cuadros/s): con poca luz la
  cámara baja sola a 7,5 cuadros/s. Más luz, o
  `v4l2-ctl -d /dev/video0 -c exposure_dynamic_framerate=0` (imagen más oscura
  pero a 30 cuadros/s).
- **Otra cámara en Linux**: `./dev app -dispositivo /dev/video2`.

## Para quien programa

- La lógica (clasificar, registro, resumen, emparejar, mandar) está en el
  paquete `desktop` del módulo raíz, con pruebas (`./dev test ./desktop/`).
- La ventana (Wails) es `desktop/app`, módulo aparte como
  `prototipos/camara`: necesita cgo y WebKitGTK en Linux, así que no entra en
  `./dev check`. `./dev app compilar` la compila para los dos sistemas.
- `./dev app e2e win|linux`: la app de verdad en los dos PCs sin cámara: el
  PC que manda pasa grabaciones reales (activar, agarrar, soltar) por el mismo
  camino que la cámara, y la captura tiene que llegar al otro.
