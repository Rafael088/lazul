# Lazul en el escritorio (v0.2.0)

La app es un indicador pequeño en una esquina que mira tu mano por la cámara.
Muestra si el modo táctil está **activo** o **inactivo** y destella un gesto
cuando lo reconoce (scroll, zoom, izquierda/derecha), sin tapar la clase. La
vista de la cámara queda como opción para acomodarse y está apagada por
defecto. La app sigue escuchando en la red local para recibir y mandar.

Esta interfaz reemplaza la ventana chica fija de la 0.1.0 (cotejo-luzcr0).

## Abrirla

| Dónde | Cómo |
| --- | --- |
| Linux (Hyprland) | en la carpeta del repo: `./dev app` |
| Windows, desde el Linux | `./dev app win` (la copia a `%USERPROFILE%\Lazul\lazul.exe` y la abre en el escritorio de Windows) |
| Windows, después | doble clic en `%USERPROFILE%\Lazul\lazul.exe` |

La primera vez se baja MediaPipe (unos 37 MB) y queda dentro del programa.
En Linux la cámara la abre `ffmpeg` (tiene que estar instalado).

La app arranca como una píldora de `236 × 64` en la esquina inferior derecha
(con 16 px de margen), permanece sobre las demás y recuerda dónde la dejaste.
Esa posición se guarda en `ventana.json`, junto al estado de la app
(`~/.local/state/lazul` en Linux o `%LOCALAPPDATA%\lazul` en Windows). En
Wayland Wails no garantiza que el compositor acepte restaurar la posición.

**Acomodar la cámara:** pulsa el botón de cámara del indicador o
`Ctrl+Mayús+C`. La ventana se agranda (`920 × 660`) y muestra la cámara con
los puntos de la mano y todos los controles. `Esc`, **Volver al indicador** o
el mismo atajo la devuelven a su esquina. La cámara sigue encendida aunque la
vista esté apagada: encenderla o apagarla es el botón **Apagar cámara** de la
vista de cámara.

Deja la app **a la vista** (basta el indicador en una esquina): si queda
escondida en otro espacio o minimizada, el sistema deja de darle cuadros y la
cámara se detiene.

### Hyprland: flotante en todos los espacios

Wails identifica la ventana en Linux con la clase `lazul`. En la configuración
Lua nueva de Hyprland, añade esta regla para hacerla flotante, fijarla a todos
los espacios, dejarla en `236 × 64` y colocarla abajo a la derecha:

```lua
hl.window_rule({
  match = { class = "lazul" },
  float = true,
  pin = true,
  size = { 236, 64 },              -- por comprobar con la versión instalada
  move = { "100%-252", "100%-80" }, -- por comprobar con la versión instalada
})
```

`float` y `pin` siguen la sintaxis Lua nueva indicada por Hyprland. Las claves
`size` y `move`, y en particular las expresiones porcentuales, están **por
comprobar** porque su representación Lua puede variar con la capa que carga la
configuración. El margen propuesto es de 12 px. El fijado a todos los espacios
depende de `pin`; `AlwaysOnTop` de Wails por sí solo no sustituye esta regla en
Wayland.

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
  momento: el modo táctil pasa a **activo** y el indicador lo muestra; otra L
  lo apaga.
- **Dos dedos arriba/abajo**: scroll. **Pellizco que abre o cierra**: zoom.
  **Barrido a un lado**: izquierda/derecha (o AvPág/RePág en diapositivas).
- Cada gesto reconocido hace **destellar** el indicador durante ~0,7 s con lo
  que se reconoció (`scroll ↑`, `zoom −`, `→`…), sin tapar la pantalla.
- Por ahora el indicador reconoce y muestra los gestos; enviarlos a la ventana
  con foco es la tarea del modo táctil (cotejo-mks5xb). El prototipo 0.1.0 de
  agarrar y soltar sigue disponible en la vista de cámara: **puño** para
  agarrar una captura, **abrir la mano moviéndola a un lado** para soltarla en
  el otro PC, y **pinza arrastrada** para mandar la ventana activa.

La vista de cámara (a demanda) muestra la cámara con los puntos de la mano, la
pose, el modo y el último gesto con su hora. Lo que llega al otro PC queda
también en la carpeta `recibidos` (`~/.config/lazul/recibidos` o
`%APPDATA%\lazul\recibidos`).

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
