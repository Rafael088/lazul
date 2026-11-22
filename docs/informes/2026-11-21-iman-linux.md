# Imán sobre los botones en Linux (cotejo-u5ji21, 21-nov-2026)

Sigue a cotejo-mpvzi8 (`2026-11-21-iman-botones.md`), que hizo solo Windows.
En la pista del 21-nov el dueño no tuvo ninguna ayuda en Linux y falló 2 de
14 blancos en el centro.

**Respuesta: el lector está hecho y probado contra un bus D-Bus real con una
app AT-SPI falsa; contra Firefox y Chromium de verdad está sin correr.** El
sandbox de la tarea no deja abrir sockets Unix (ni el de Hyprland ni el bus
de la sesión: «Operation not permitted»), así que ni `TestObjetivosVivo` ni
la pista se pudieron correr aquí. Lo de abajo dice cómo hacerlo en el
escritorio.

## Qué cambió

- `platform/linux/objetivos.go`: `objetivosHyprland` (`platform.TargetFinder`).
  - `FocusedWindow`: `activewindow` por el socket de órdenes de Hyprland
    (`$XDG_RUNTIME_DIR/hypr/$HYPRLAND_INSTANCE_SIGNATURE/.socket.sock`, sin
    abrir un proceso cada 250 ms); si el socket falla, `hyprctl activewindow -j`.
    El id es la dirección de la ventana.
  - `Targets`: ventana activa (posición, tamaño, monitor, PID, título) y
    monitores de Hyprland, árbol de la app por AT-SPI y `aPantalla`, que
    lleva cada rectángulo a (0,0)-(1,1) del monitor que recibe el táctil (el
    de `Displays.Target`; si no hay, el que tiene el foco). `LogicalWidth` y
    `LogicalHeight` son el tamaño lógico de ese monitor. Tope: 800 ms para la
    lectura entera y 300 ms por consulta a Hyprland, como en Windows.
  - Coordenadas: AT-SPI se pide en coordenadas **de la ventana**
    (`ATSPI_COORD_TYPE_WINDOW`), porque en Wayland las de pantalla no valen,
    y se les suma la posición de la ventana de Hyprland. Se resta el origen
    del marco de la app; si el marco es más grande que la ventana por igual
    a los dos lados (la sombra de GTK con decoración propia), se quita ese
    margen; si el marco mide la ventana por la escala del monitor (XWayland
    sin escalar, píxeles físicos), todo se divide por la escala.
  - No cuentan: lo que no tiene tamaño, lo que tiene el centro fuera de la
    ventana (desplazado fuera de la vista, otra pestaña), lo que ocupa casi
    toda la ventana (≥ 90 % por eje: el borde de la ventana) y lo que cae
    fuera del monitor del táctil. Nada de Hyprland (barra, bordes, fondo) está
    en el árbol de la app, así que nunca es objetivo.
- `platform/linux/atspi.go`: `a11yDBus`, con godbus (v5.1.0, ya en la caché de
  módulos por Wails).
  - Bus de accesibilidad: `org.a11y.Bus.GetAddress` en el bus de la sesión
    (o `AT_SPI_BUS_ADDRESS`); una sola conexión, se rehace si se cae.
  - La app de la ventana se busca por PID entre los hijos del registro
    (`GetConnectionUnixProcessID`), con caché PID → nombre.
  - Su ventana es el marco con `ACTIVE`; si no, el que se llama como la
    ventana de Hyprland; si la app tiene uno solo, ese.
  - Objetivos: `Collection.GetMatches` sobre ese marco, de una vez, hasta 800:
    `SHOWING` y `SENSITIVE` y rol botón, botón conmutable, botón con menú,
    enlace, casilla, radio, pestaña, menú, elemento de menú (normal, casilla o
    radio) o combo. Si la app no tiene Collection (GTK 4, Qt), se recorre el
    árbol por niveles, solo por los nodos mostrados y hasta 4000 nodos.
  - Los rectángulos se piden todos sin esperar uno por uno, hasta 100 en
    vuelo (un dbus-daemon sin configurar corta las de más; el de
    accesibilidad admite 50 000).
- `platform/linux/linux.go`: `Targets` en `New()`, así que la app ya crea el
  imán en Linux (`desktop/app/main.go` no cambia, ni `desktop/iman.go`: misma
  caché por ventana con foco, cada 1,5 s y solo con la mano en el panel).
- `pantallas.go`: `monitoresLogicos` da también el id y la escala de cada
  monitor; `puntero.go`: `monitorDestino`.
- `desktop/README.md`: cómo encenderlo en Chromium y cómo comprobarlo.

## Chromium y Electron

Firefox se registra solo en AT-SPI. Chromium, Chrome, Brave y las apps de
Electron solo exponen el contenido de la página si arrancan con
`--force-renderer-accessibility` o con `ACCESSIBILITY_ENABLED=1` en el
entorno (para dejarlo fijo, la opción en `~/.config/chromium-flags.conf`).
Sin eso el imán ve, a lo sumo, los botones de la barra del navegador. Si la
app no está en el bus, `Targets` devuelve «la app de la ventana no está en el
bus de accesibilidad…» y la línea `imán:` de la app lo dice.

**Sin comprobar** (no se llega al escritorio desde el sandbox): que Firefox
se registre sin `org.a11y.Status.IsEnabled=true` y que la bandera de Chromium
baste en Wayland. Es lo primero que dice `TestObjetivosVivo`.

## Pruebas (`./dev check` en verde)

- `objetivos_test.go`, puras: ventana activa de hyprctl (y `{}`/`Invalid` →
  `ErrSinVentana`); `aPantalla` con un botón, la ventana entera, casi toda la
  ventana, algo desplazado fuera, algo sin ancho y algo que se sale un poco;
  píxeles físicos a escala 2; sombra de GTK y marco con origen; segundo
  monitor (y la misma ventana vista desde el otro monitor queda fuera);
  elección del monitor del táctil; elección del marco; la regla de
  `GetMatches` tiene la firma `(aiia{ss}iaiiasib)` de libatspi y los bits
  de los roles y estados de `atspi-constants.h` (sacados compilando el
  encabezado instalado); `Targets` con Hyprland y AT-SPI falsos, dirigido al
  otro monitor, sin ventana y con la app fuera del bus.
- `atspi_test.go`: arranca un `dbus-daemon` propio por TCP (no toca el bus
  del escritorio) con una app falsa de dos ventanas, dos botones, un enlace,
  un botón deshabilitado y uno dentro de un nodo oculto. `a11yDBus` elige la
  ventana activa y da los 3 objetivos, **con Collection y recorriendo el
  árbol**; un PID que no está falla.

## Costo

`BenchmarkA11yDBus800` (la app falsa en otro proceso, 800 enlaces, por TCP):

| | por lectura |
| --- | --- |
| tiempo | 86 ms |
| CPU de Lazul | 102 ms (godbus: codificar y decodificar cada mensaje y sus syscalls) |
| memoria | 5,6 MB, 233 000 asignaciones |

Son ~0,13 ms de CPU por objetivo. Con el tope de 800 y una lectura cada
1,5 s, a lo sumo **~7 % de un núcleo** mientras la mano está en el panel
sobre una página enorme; en una página normal (100-200 objetivos), ~1-2 %.
Fuera del panel o con el imán apagado no se lee. La app que contesta (el
navegador) gasta aparte. `FocusedWindow` va por el socket de Hyprland y no
abre procesos. Si hiciera falta bajarlo: tope de objetivos más chico o leer
solo los que caen en el monitor del táctil.

## Cómo medirlo en el escritorio (pendiente)

1. Firefox delante, en una página con enlaces:
   `LAZUL_IMAN_VIVO=1 LAZUL_IMAN_ESPERA=5s go test ./platform/linux -run ObjetivosVivo -v`.
   Dice cuántos objetivos ve, los 5 primeros en píxeles lógicos del monitor
   (compararlos con `hyprctl activewindow` y una captura), la media y el
   peor de 10 lecturas y la CPU de Lazul por lectura.
2. Lo mismo con Chromium sin bandera (debe decir que no está en el bus o ver
   solo la barra) y con `--force-renderer-accessibility`.
3. Pista: `./dev app` con el imán encendido, `./dev pista` en Firefox, y
   comparar con la del 21-nov (2 de 14 fallados en el centro). Después
   `./dev app resumen` para los `iman_ajusta` / `iman_duda`.

## Límites

- Solo ve lo que la app expone a AT-SPI: Chromium sin la bandera, un canvas
  o un juego no tienen objetivos.
- Los menús desplegables de Firefox y GTK son superficies aparte (popups de
  Wayland): Hyprland no los da como ventana activa, así que sus elementos se
  ubicarían con la posición de la ventana de debajo. Sin medir.
- La corrección de la sombra supone que es igual a los dos lados; una
  sombra asimétrica correría los objetivos unos píxeles.
- Apps en Flatpak: el PID que ve Hyprland puede no ser el del bus.
