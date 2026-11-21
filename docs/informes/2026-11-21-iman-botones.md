# Imán sobre los botones (cotejo-mpvzi8, 21-nov-2026)

**Pregunta:** si Lazul sabe dónde están los botones y enlaces de la ventana
con el foco, ¿puede frenar el puntero encima de los chicos y llevarles el
toque que cae casi encima sin tocar nunca uno que no se quería?

**Respuesta: en objetivos sintéticos sí (60 % → 82 % de aciertos, +0,2 puntos
de toques en otro objetivo, ninguno «a la par»). En Windows de verdad está
sin medir**: el PC Windows no se alcanza desde el sandbox de esta tarea
(`ssh win`: red inalcanzable), así que el lector de UI Automation compila y
pasa `go vet` en cruz, pero no se ha corrido. El nivel 1 (área de contacto,
cotejo-6eg99p) tampoco está medido aún: el imán va con interruptor para
comparar los dos.

## Qué cambió

- `platform/platform.go`: `TargetFinder` (opcional) con `FocusedWindow`
  (barato) y `Targets` (caro): los objetivos en las mismas coordenadas
  normalizadas que `TouchScreen`, más el tamaño lógico del monitor para medir
  radios en píxeles CSS.
- `platform/windows/objetivos.go`: UI Automation sobre la ventana de
  `GetForegroundWindow`, con DPI físico (`conDPIFisico`). Pide de una vez
  (`FindAllBuildCache` con `BoundingRectangle` en caché: una sola ida y vuelta
  aunque haya cientos de enlaces) los Button, Hyperlink, CheckBox, ComboBox,
  MenuItem, RadioButton, TabItem y SplitButton habilitados y no fuera de
  pantalla, hasta 800, con un tope de 800 ms.
- `desktop/iman.go`:
  - `Iman`: caché **por ventana con foco**, no por cuadro. Cada 250 ms mira
    qué ventana tiene el foco; lee la lista al cambiar y cada 1,5 s, solo
    mientras la mano está en el panel con el panel encendido. Levantar un
    dedo invalida la lista (un tap abre otra página, un arrastre desplaza) y
    se esperan 400 ms antes de leer otra vez. Una lista de otra ventana o de
    más de 2,5 s no corrige nada.
  - `FrenoIman`: encima de un objetivo chico (lado corto ≤ 48 px lógicos y
    largo ≤ 480) el puntero avanza la mitad de lo que avanza la mano; fuera,
    recupera el 35 % de la distancia por cuadro. Nunca se separa de la mano
    más de 18 px. Solo actúa con ningún dedo apoyado.
  - `AjustarToque`: si el dedo cae dentro de un objetivo (chico o no), no se
    mueve. Si cae fuera de todos y a ≤ 18 px (`DiametroContacto / 2`) de uno
    chico, va a su centro (en un enlace largo, al punto más cercano a 12 px
    del borde, para no saltar 150 px). Si el segundo más cercano está a menos
    de 12 px más que el primero: **duda, no se corrige**.
- `desktop/panel.go`: al apoyar con una mano, el contacto pasa por
  `AjustarToque` y el dedo entero se corre ese desfase hasta levantarlo. Así
  el tap no se convierte en un arrastre y el arrastre no salta. Un doble
  toque (≤ 500 ms y a ≤ 18 px del anterior) repite el ajuste aunque la lista
  ya no valga. Dos manos (zoom) no se ajustan. Imán nil o apagado: el código
  de siempre.
- App: `-iman` (por defecto encendido; `-iman=false` lo apaga), una casilla
  **Imán sobre los botones chicos** en la vista de cámara, y cada 5 s una
  línea `imán: …` con lecturas, ms por lectura, objetivos y
  ajustes/dudas. El registro anota `iman_ajusta`, `iman_duda` e
  `iman_repetido`, sin posiciones.
- Linux: **no hecho**. El enunciado pide empezar por Windows; en Linux falta
  leer AT-SPI y sumarle la posición de la ventana de `hyprctl clients`.
  Sin `TargetFinder`, el imán no aparece.

## Pruebas con objetivos sintéticos (`desktop/iman_test.go`)

- `TestAjustarToqueConObjetivosSinteticos`: dentro no se mueve; a 10-12 px
  va al centro; a 30 px no; entre dos botones parecidos, duda; enlace largo;
  la tarjeta grande ni atrae ni se mueve; un rectángulo repetido se junta.
- `TestFrenoIman…`: encima avanza la mitad, el desfase no pasa de 18 px y
  fuera vuelve con la mano.
- `TestImanCacheaPorVentanaYNoPorCuadro`: 6 vueltas en la misma ventana, una
  lectura; cambiar de ventana relee; tras levantar el dedo espera; apagado
  no lee ni corrige; un error deja sin lista.
- `TestPanelConIman…`: el tap a 11 px del botón cae en su centro **en todos
  los cuadros** (sin arrastre); un arrastre avanza cuadro a cuadro igual que
  sin imán; con duda o apagado el toque no se mueve; el doble toque cae dos
  veces en el mismo sitio.
- `TestImanNoTocaObjetivosQueNoSeQuerian`: 200 páginas con una barra de
  botones de 16-40 px (huecos de 4-40 px) y una columna de enlaces de una
  línea (huecos de 2-22 px); 10 000 toques a un punto del objetivo con 8 px
  de error por eje:

  | | Aciertos | En otro objetivo |
  | --- | --- | --- |
  | sin imán | 60,3 % | 2,0 % |
  | con imán (margen de duda 12 px) | 81,7 % | 2,2 % |
  | con margen 8 px (descartado) | 86,2 % | 2,8 % |
  | con margen 16 px | 77,8 % | 2,1 % |

  Los 2,0 % que ya caían en otro objetivo no cambian: el imán no mueve un
  toque que ya acierta algo. Los 0,2 puntos que agrega son dedos que cayeron
  a 1-3 px de otro objetivo y a más de 17 px del querido: la prueba exige que
  ninguno vaya a otro objetivo sin estar al menos 12 px más cerca de él, y
  que no pasen del 0,3 %. La simulación no tiene el freno, que en la mano
  real debería bajar esos errores.

## CPU

- Por cuadro (freno + ajuste con 400 objetivos): **4,1 µs**, 0 asignaciones
  (`go test ./desktop -bench Iman`). A 30 cuadros/s es el 0,012 % de un
  núcleo.
- La lectura de UI Automation corre fuera del cuadro, a lo sumo cada 1,5 s y
  solo con la mano en el panel. Su costo está **sin medir**: buena parte cae
  en el proceso del navegador (que contesta a UIA), no en el de Lazul, así
  que la línea `CPU` de la app no lo ve entero.

## Cómo medirlo en Windows

1. `./dev win test` corre `TestNormalizarObjetivosALaZonaDelTactil`.
2. Con Edge delante: `LAZUL_IMAN_VIVO=1 go test ./platform/windows -run
   ObjetivosVivo -v` en la sesión del escritorio. Dice cuántos objetivos ve y
   cuánto tarda una lectura. Si da 0 objetivos o un error de UIA, revisar
   primero las posiciones de las tablas de métodos (`uiaCreateCacheRequest`
   = 20, `CreateOrConditionFromNativeArray` = 30, `FindAllBuildCache` = 8,
   `GetCachedPropertyValue` = 12), sacadas de `UIAutomationClient.h`.
3. CPU: `./dev app win -salida a.txt -duracion 120s` con y sin `-iman=false`,
   la mano en el panel sobre una página con muchos enlaces; comparar las
   líneas `cámara: … CPU` y el CPU de `msedge.exe` en el Administrador de
   tareas. Lo pedido: no subir más de unos puntos.
4. Toques no queridos: en `herramientas/toque-contacto.html`, 10 toques a
   5-15 px de cada blanco con el imán y 10 sin él, y anotar los que caen en
   el blanco de al lado. Después `./dev app resumen` y contar `iman_duda`.

## Límites

- Solo ve lo que la app expone a la accesibilidad. Un canvas, un juego o una
  app sin UIA no tiene objetivos: el imán no hace nada.
- Un texto normal pegado a un enlace (a ≤ 18 px) puede llevarse el toque al
  enlace: el texto no es objetivo. Con dudas, apagar con la casilla.
- Con la lista de hace un momento, una página que se mueve sola (un carrusel)
  puede desplazar los objetivos antes de releer (≤ 1,5 s).
