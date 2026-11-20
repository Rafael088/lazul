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
  guarda esa esquina; repite en la inferior derecha. Puedes pulsar **Recalibrar**
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
  exigen unos cuadros y 250 ms abierto; para apoyar, 3 cuadros cerrados seguidos.
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
milisegundos de Hand y Pose y cuántos episodios quedaron bloqueados por falta
de torso, fuera del torso o fuera del panel. El resumen guarda esas
activaciones evitadas por motivo junto a las activaciones falsas.

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
- `./dev app e2e win|linux`: la app de verdad en los dos PCs sin cámara: el
  PC que manda pasa grabaciones reales (activar, agarrar, soltar) por el mismo
  camino que la cámara, y la captura tiene que llegar al otro.
