# Lazul en el escritorio (v0.2.0)

La app es un indicador pequeño en una esquina que mira tu mano por la cámara.
Muestra si el modo táctil está **activo** o **inactivo**, y un halo sobre toda
la pantalla sigue la mano: nada fuera del panel, suave dentro y encendido al
agarrar. No tapa la clase. En Linux y en Windows, un panel imaginario delante de la persona
mueve el puntero; al pellizcar, una pantalla táctil virtual toca la ventana que
tenga el foco (un dedo para tocar o arrastrar y dos para el zoom nativo). Los
otros gestos desplazan o cambian de diapositiva. Es la prueba de uso real
de la 0.2.0 (dec-gzgl). La vista de la cámara queda como opción para acomodarse
y está apagada por defecto.

Esta interfaz reemplaza la ventana chica fija de la 0.1.0 (cotejo-luzcr0). La
transferencia entre dos PCs de la 0.1.0 sigue disponible con
`./dev app -modo transferencia` (en Windows, `./dev app win -modo transferencia`).
El táctil virtual de Windows usa `InjectTouchInput` y no necesita pantalla
táctil (cotejo-ckwoq7, `docs/informes/2026-11-19-tactil-windows.md`).

## Abrirla

| Dónde | Cómo |
| --- | --- |
| Linux (Hyprland, modo táctil) | en la carpeta del repo: `./dev app` |
| Windows (modo táctil), desde el Linux | `./dev app win` (la copia a `%USERPROFILE%\Lazul\lazul.exe` y la abre en el escritorio de Windows) |
| Windows, después | doble clic en `%USERPROFILE%\Lazul\lazul.exe` |

La primera vez se baja MediaPipe con los modelos de manos y Pose y queda
dentro del programa.
En Linux la cámara la abre `ffmpeg` (tiene que estar instalado).

La app arranca como una píldora de `236 × 64` en la esquina inferior derecha
(con 16 px de margen), permanece sobre las demás y recuerda dónde la dejaste.
En el modo táctil, además, un halo transparente sigue la mano cuando entra en el panel calibrado (nada fuera, suave dentro y
encendido al agarrar). Esa superficie no recibe clics ni foco y no tapa la
píldora.
En Windows, la ventana nativa del indicador tampoco roba el foco. Su región de
entrada se recorta a la píldora redonda: si WebView2 conserva por error el
tamaño grande de la vista de cámara, toda la parte transparente deja pasar los
clics. Al abrir **Acomodar la cámara** se restaura la ventana interactiva
completa; al volver al indicador se recorta otra vez y el foco regresa a la app
que se estaba usando.
Esa posición se guarda en `ventana.json`, junto al estado de la app
(`~/.local/state/lazul` en Linux o `%LOCALAPPDATA%\lazul` en Windows). En
Wayland Wails no garantiza que el compositor acepte restaurar la posición.
La calibración del panel se guarda aparte en `panel.json`, en la misma carpeta.

**Acomodar la cámara:** pulsa el botón de cámara del indicador o
`Ctrl+Mayús+C`. La ventana se agranda (`920 × 660`) y muestra la cámara con
los puntos de la mano, el torso permitido en verde y todos los controles.
`Esc`, **Volver al indicador** o
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

### Hyprland: el aura encima de todo

El aura no es una ventana normal: es una superficie **layer-shell** en la capa
`overlay`, con el espacio de nombres `lazul-aura`. Por eso ya queda encima de
las ventanas, su región de entrada está vacía (los clics pasan) y su modo de
teclado es `none` (no roba el foco). No necesita una regla de ventana para eso.

Lo único que conviene añadir en la configuración Lua de Hyprland es quitarle la
animación de entrada, para que el halo no parpadee al aparecer:

```lua
hl.layer_rule({
  match = { namespace = "lazul-aura" },
  no_anim = true,
})
```

Con `hyprctl layers` se comprueba que aparece en la capa `overlay` con el
espacio de nombres `lazul-aura`. Si el compositor no soporta layer-shell (X11,
o un Wayland sin `wlr-layer-shell`), el aura no se crea y la app sigue igual:
la señal visual vuelve a ser la píldora del indicador.

### Hyprland: el toque pegado al borde de las ventanas

Omarchy trae `general:resize_on_border = true` y un área de agarre de 8 px
(`general:extend_border_grab_area`). Con eso, un toque táctil de Lazul a menos
de 8 px del borde de una ventana **redimensiona** la ventana en vez de hacer
clic, y los botones o pestañas pegados al borde quedan casi inalcanzables.

No hay que tocar `~/.config/hypr`: mientras el panel está encendido, Lazul
lee el valor con `hyprctl getoption general:resize_on_border`; si está
encendido lo apaga en caliente con `hyprctl eval` y `hl.config`
(`hl.config({ general = { resize_on_border = false } })`, el mismo mecanismo
que `hl.dsp`). Al apagarlo con la L o cerrar la app, devuelve el valor que
había. Si la app se cae, el original queda guardado en `borde.json`, al lado
del registro (`~/.local/state/lazul`), y se restaura al arrancar. La píldora lo
avisa una vez («borde: clic») para que no sorprenda que el borde ya no
arrastre. Si el dueño ya lo tenía apagado a mano, Lazul no lo enciende al
terminar.

Para comprobarlo en caliente:

```
hyprctl getoption general:resize_on_border
```

## Emparejar los dos PCs (solo para transferencias)

El modo táctil actúa en el mismo equipo y no necesita emparejar nada.
Para usar el modo de transferencia anterior, abre la app con
`./dev app -modo transferencia` y sigue estos pasos:

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

- Abre la vista de cámara y pulsa **Calibrar panel**. Imagina un rectángulo
  cómodo delante de ti: pon la palma quieta en su esquina superior izquierda y
  guarda esa esquina; repite en la inferior derecha. Por último, **junta los
  dedos** (la punta del índice contra la del pulgar, quieta) y guarda tu
  pellizco: así el pellizco solo cuenta cuando tus dedos se tocan de verdad.
  Una calibración de antes sin este paso usa los umbrales por defecto. Puedes pulsar **Recalibrar**
  cuando cambies de silla, cámara o postura.
- Dentro de ese rectángulo, la palma mueve el puntero por toda la pantalla. El
  tamaño aparente de la mano compensa cuánto te acercas a la cámara y un filtro
  One Euro suaviza el temblor. Fuera del panel o claramente por delante/detrás,
  no mueve nada.
- La mano solo cuenta si su palma está a la vez dentro del panel y delante del
  torso que Pose delimita entre hombros y caderas. Rascarse la cabeza, señalar
  fuera del cuerpo o bajar el brazo no activa gestos ni mueve el puntero. Si
  Pose pierde el torso, Lazul falla cerrado y espera a volver a verlo.
- **Pellizco pulgar-índice = dedo en la pantalla**: el puntero sigue la palma
  sin pulsar ningún botón, para anticipar dónde caerá el contacto. Al cerrar el
  pellizco se apoya un dedo táctil, al moverlo se arrastra y al abrirlo se
  levanta. Solo cuenta si empieza dentro del panel; uno que empieza fuera se
  ignora hasta soltarlo. Para que el parpadeo no corte el contacto, al soltar se
  exigen unos cuadros y 250 ms abierto. Para apoyar hay dos niveles: un
  pellizco muy cerrado apoya desde el primer cuadro (un tap rápido cuenta) y
  uno tibio, apenas cerrado, espera 3 cuadros cerrados seguidos.
  Mientras se cierra el pellizco el puntero se queda donde apuntaba la mano
  abierta (hasta 400 ms) y el dedo cae ahí, no donde la palma se movió al cerrar. Dos manos pellizcando son dos dedos:
  el navegador, PDF u otra app reconoce directamente su zoom. Cada dedo lleva
  su propio filtro contra el temblor.
- Al arrancar, la vista de cámara dice **Entrada: pantalla táctil virtual**. Si
  no se puede crear (por ejemplo, falta acceso a `/dev/uinput`), la píldora
  muestra **respaldo** y la vista explica que Lazul conserva el comportamiento
  anterior: un pellizco pulsa el botón y dos manos mandan `Ctrl` + `+` / `-`.
  Apagar la cámara, salir o recalibrar levanta cualquier dedo o botón pendiente.
- **La L** (índice arriba, pulgar al lado, los demás recogidos), quieta un
  momento: enciende o apaga el modo táctil; el indicador lo muestra.
- **Scroll como el clic central**: junta las puntas del pulgar, el índice y
  el medio, como si pellizcaras algo para agarrarlo (anular y meñique
  recogidos), y quédate quieto un instante. Aleja la mano hacia abajo o hacia arriba: el
  contenido se desplaza mientras sostengas la pose, más rápido cuanto más
  la alejes. Vuelve la mano al punto donde agarraste y se detiene; suelta los
  dedos y termina. Va por una rueda de ratón virtual, así que actúa sobre la
  ventana que está **bajo el puntero**.
- **Pinza que se abre o se cierra**: acerca o aleja.
- **Dos dedos estirados hacia la izquierda o la derecha**: cambian de
  diapositiva. Los dos dedos ya no hacen scroll.
- Si la cámara deja de ver una mano durante 3 segundos, el modo se apaga solo.
- Si sueltas y vuelves a agarrar en la dirección contraria antes de 2,5 s, se
  ignora (es la mano que vuelve); para corregir, mueve la mano sin soltar.
- El halo es la señal visual: fuera del panel calibrado no se ve nada; dentro,
  un halo suave sigue la mano; al agarrar (pellizco) se enciende. La píldora
  sigue mostrando el último gesto reconocido, sin tapar la pantalla.
- **Cambiar de pantalla con tres dedos**: estira índice, medio y anular con
  el pulgar y el meñique recogidos y desliza la mano hacia tu izquierda o tu
  derecha más o menos un palmo, sin prisa (como los tres dedos del panel
  táctil). Cuenta el recorrido desde donde hiciste la pose, no la velocidad;
  hacia arriba o abajo no cuenta. Un cambio por pose: para otro, baja la mano
  o deshaz la pose y vuelve a hacerla. Mientras se ve la pose, esa mano no
  toca ni arrastra en el panel. No cuenta mientras pellizcas.
  - **Dos pantallas**: el panel controla el monitor que tiene el foco, y el
    halo aparece en ese monitor, así se ve cuál se controla. Los tres dedos
    pasan el control al monitor de ese lado. Si cambias el foco de monitor
    con el ratón o el teclado, el panel lo sigue (lo mira una vez por
    segundo, fuera del camino de los cuadros: aunque `hyprctl` tarde o se
    cuelgue, la mano sigue procesándose). En Linux el cambio también mueve el
    foco (`hl.dsp.focus`) y la salida del táctil virtual (`hl.device`, con
    `hyprctl eval`); el táctil se recrea para tomarla (~0,7 s). En Windows
    los toques (`InjectTouchInput`) y el puntero van a las coordenadas de ese
    monitor; el foco no se mueve.
  - **Una pantalla**: hace lo del sistema. En Hyprland cambia al escritorio
    abierto de al lado (a tu derecha, el siguiente: `e+1`); en Windows cambia
    de app (a tu derecha, Alt+Tab; a tu izquierda, Alt+Mayús+Tab).
  - Cada intento se anota en el registro (`"tipo":"pantalla"`): `"resultado":"disparo"`
    con lo que hizo en `motivo` (`monitor`, `espacio`, `sin_vecino`, `error`) o
    `"resultado":"no"` con por qué no (`corto`: no llegó a una palma; `vertical`;
    `salto` del seguimiento; `dos_manos` en la pose; `ocupado`: pellizcando o
    calibrando), el lado, el recorrido en `palmas` y los `ms`; nunca posiciones.
    Para ajustar los umbrales: graba tomas «tres dedos a la derecha/izquierda»
    con `./dev grabar` y mídelas con `./dev replay -pantalla -v`.
- **Imán sobre los botones chicos**: Lazul lee con la accesibilidad del
  sistema (UI Automation en Windows, AT-SPI en Linux) dónde están los botones, enlaces, casillas y menús de la ventana
  con el foco (una vez al cambiar de ventana y cada 1,5 s con la mano en el
  panel, nunca por cuadro). Encima de un objetivo chico (lado corto ≤ 48 px)
  el puntero avanza a la mitad, y si el pellizco apoya el dedo a 18 px o menos
  de uno, pero fuera de todos, el toque cae en él (al centro; en un enlace
  largo, donde tocaste). Si dos objetivos quedan casi igual de cerca (menos de
  12 px de diferencia) no se corrige. Marcos, barras de desplazamiento,
  separadores, pestañas y objetivos pegados al borde de la ventana se
  descartan antes de frenar o ajustar. Tras levantar el dedo espera 400 ms a
  que la página se asiente; un doble toque repite el mismo ajuste. Se apaga
  con la casilla **Imán sobre los botones chicos** de la vista de cámara o
  arrancando con `-iman=false`.
  En Linux la posición de cada botón dentro de la ventana sale de AT-SPI y la
  de la ventana, de Hyprland; la barra, los bordes y nada de Hyprland cuentan
  como objetivo. **Firefox** se expone solo, pero solo si la sesión tiene la
  accesibilidad encendida al arrancar. En Hyprland y Omarchy
  `org.a11y.Status.IsEnabled` viene en `false`, y entonces Firefox y Zen ni
  siquiera se registran en el bus. Al arrancar con el imán encendido, Lazul
  lo lee en `/org/a11y/bus` del bus de la sesión y, si está en `false`, lo
  pone en `true` por D-Bus. No lo apaga al cerrar (otras apps pueden
  usarlo). Los navegadores que ya estaban abiertos no se registran hacia
  atrás: hay que reiniciarlos para que el imán los vea, y el indicador lo
  avisa una vez. La línea `accesibilidad de la sesión` de la salida y el
  registro (`"tipo":"accesibilidad"`) dicen si Lazul la encendió
  (`encendida`), si ya estaba (`ya_estaba`) o si el bus no existe
  (`sin_bus`, falta `at-spi2-core`: se avisa y la app sigue sin imán).
  **Chromium, Chrome, Brave y las apps de Electron** solo exponen la página
  si se abren con `--force-renderer-accessibility` (o con
  `ACCESSIBILITY_ENABLED=1` en el entorno); sin eso el imán ve, a lo sumo,
  los botones de la barra del navegador. Para dejarlo fijo: añadir la opción
  a `~/.config/chromium-flags.conf` (o `chrome-flags.conf`,
  `brave-flags.conf`).
  Si la línea `imán:` dice «no está en el bus de accesibilidad», la app de la
  ventana no se registró en AT-SPI. Para comprobarlo con un navegador
  delante: `LAZUL_IMAN_VIVO=1 go test ./platform/linux -run ObjetivosVivo -v`
  (`LAZUL_IMAN_ESPERA=5s` da tiempo a cambiar de ventana).
  Cada 5 s la salida dice `imán: …` con las lecturas, su tiempo y los ajustes
  y dudas; el registro anota también `iman_frena`. Para un freno o ajuste
  guarda solo el rol genérico y el tamaño del objetivo, nunca texto ni
  posiciones.

Los gestos se aplican a la ventana que tenga el foco; el panel mueve el
cursor y el pellizco toca (o usa el respaldo de botón si el táctil no está
disponible). La vista de cámara (a demanda) muestra la
cámara con los puntos de la mano y la zona del torso, el modo **ACTIVO/INACTIVO**, la cuenta atrás
cuando no ve la mano y el último gesto con su hora. Lo que llega al otro PC en el modo de
transferencia queda en la carpeta `recibidos` (`~/.config/lazul/recibidos` o
`%APPDATA%\lazul\recibidos`).

### Según la app con foco (Linux)

Cada gesto se traduce según la ventana con foco (`platform/linux/perfiles_tactiles.go`):

| App | Scroll sin rueda virtual | Barrido |
| --- | --- | --- |
| Navegador (Zen, Firefox, Chrome…) | 8 flechas | ← / → (Google Slides, Canva, reveal.js) |
| PDF (Evince, Papers, Okular, Zathura…) | 3 flechas | RePág / AvPág |
| Diapositivas (LibreOffice, OnlyOffice…) | 1 flecha | RePág / AvPág |
| Cualquier otra | 3 flechas | RePág / AvPág |

El scroll normal es continuo por la rueda virtual (`platform/linux/rueda.go`,
`/dev/uinput`, que el usuario de la sesión puede escribir); las flechas solo
se usan si no se puede crear. La velocidad está en `desktop.VelocidadRueda`.
El zoom de dos manos lo reconoce la app como dos dedos. `Ctrl` + `+` / `-`
solo se usa como respaldo si no se pudo crear la pantalla táctil virtual.

## La prueba de las activaciones falsas (30 min)

La meta: menos de 1 activación sin querer en 30 min de trabajo normal.

1. Con la cámara encendida, pulsa **Empezar prueba de trabajo normal**.
2. Trabaja 30 min como siempre (escribir, tomar café, hablar con las manos)
   **sin** hacer gestos a propósito. Mientras dure, Lazul reconoce y registra
   los gestos, pero no manda teclas, y toda activación cuenta como falsa.
3. Pulsa **Terminar prueba** y **Resumen**: dice PASA o NO PASA.

La línea de rendimiento muestra cuadros/s, CPU del árbol completo de Lazul,
promedio de manos detectadas, milisegundos de Hand y Pose, el intervalo real
de Pose y cuántos episodios quedaron bloqueados por falta de torso, fuera del
torso o fuera del panel. El resumen guarda esas activaciones evitadas por
motivo junto a las activaciones falsas.

Pose corre normalmente una vez cada 3 cuadros. Si Hand tarda 24 ms o más,
Lazul la espacia temporalmente a una vez cada 6 cuadros para aliviar la GPU.
Con una mano a la vista sigue solo esa y busca la segunda cada ~0,5 s; con
dos sigue las dos, porque el zoom las necesita. `-manos dos` sigue siempre
dos, para comparar; la línea de rendimiento anota el modo y las búsquedas. En
Windows el reloj de la cámara mantiene 30 Hz sin sumar otros 33 ms después de
cada inferencia.

Cuando el panel deja una mano fuera, el registro dice también **por qué**:
`fuera_borde` (la palma rebasó el rectángulo), `cerca` o `lejos` (está a otra
distancia del plano que se calibró), `sin_palma` (la palma salía cortada del
cuadro) o `sin_panel` (todavía no hay panel calibrado). Nunca guarda dónde
estaba. `./dev app resumen` los cuenta en la línea **Salidas del panel**, por
ejemplo `Salidas del panel: cerca 4, fuera_borde 92, lejos 8`.

Fuera de la prueba también se cuenta: si se activa sola, pulsa **La última
activación fue sin querer**; si la activaste tú y no hiciste nada después,
**Fue a propósito**. Desde el Linux también: `./dev app resumen`.

Todo queda en un archivo de este PC (`factibilidad.jsonl` en
`~/.local/state/lazul` o `%LOCALAPPDATA%\lazul`): la hora, cada gesto táctil,
su dirección, confianza, resultado y la activación a la que pertenece. También
anota si se usó la pantalla táctil o el respaldo y cuándo se apoyan o levantan
dedos, pero no sus posiciones.
**Nunca** guarda imágenes de la cámara ni de la pantalla, ni direcciones web,
ni nombres de archivos o ventanas.

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

- La lógica (clasificar, ejecutar acciones táctiles, registro, resumen,
  emparejar y mandar) está en el
  paquete `desktop` del módulo raíz, con pruebas (`./dev test ./desktop/`).
- La ventana (Wails) es `desktop/app`, módulo aparte como
  `prototipos/camara`: necesita cgo y WebKitGTK en Linux, así que no entra en
  `./dev check`. `./dev app compilar` la compila para los dos sistemas.
- El aura es una superficie layer-shell aparte de la ventana de Wails (Wails v2
  no abre más de una ventana): `desktop/app/aura.c` la crea y
  `desktop/app/aura_linux.go` la enlaza con Go. La página es
  `desktop/app/frontend/aura.html`. En Windows es una ventana por capas
  (`desktop/app/aura_windows.go`) que no recibe clics ni toques, con la misma
  figura dibujada en Go (`aura_pintar.go`). En un Linux sin cgo no se dibuja.
- En Windows, `desktop/app/entrada_ventana_windows.go` identifica la ventana
  principal por PID y por la clase `LazulIndicador`. Mientras está colapsada le
  pone `WS_EX_NOACTIVATE`, la saca de Alt+Tab y aplica una región redonda del
  tamaño de la píldora. La app arranca oculta y solo se muestra con
  `SW_SHOWNOACTIVATE`, después de aplicar esas protecciones.
- `./dev app e2e win|linux`: la app de verdad en los dos PCs sin cámara: el
  PC que manda pasa grabaciones reales (activar, agarrar, soltar) por el mismo
  camino que la cámara, y la captura tiene que llegar al otro.
