# Pellizco más preciso (cotejo-lgbapm, 20-nov-2026)

**Problemas:** (a) el pellizco se armaba sin que los dedos se tocaran
(`UmbralCerrar` 0,30) y (b) un tap rápido no contaba: apoyar exigía 3 cuadros
cerrados (~100 ms a 30 fps, ~200 ms a 15 fps en Windows).

## Qué dicen las tomas (fixtures/pellizco)

Apertura pulgar-índice en largos de palma, mínimo de cada pellizco:

| | pellizcos | p10 | p50 | p90 |
| --- | --- | --- | --- | --- |
| de verdad (arrastres y zoom) | 221 | 0,042 | 0,088 | 0,154 |
| involuntarios (entrar, salir, normal) | 21 | 0,081 | 0,182 | 0,270 |

Con 0,30 casi todos los involuntarios cerraban; con 0,20 se queda fuera un
tercio, y de los de verdad solo 9 de 221. Arrastrando, el pellizco se afloja:
la mediana de los cuadros cerrados es 0,13.

## Lo que cambió

- **`UmbralCerrar` 0,30 → 0,20** y un **segundo nivel, `UmbralFirme` 0,12**
  (`gestures/pellizco.go`): un pellizco por debajo de 0,12 apoya desde el
  1.er cuadro; uno tibio (0,12–0,20) sigue esperando `CuadrosParaApoyar` (3).
  Abrir sigue en 0,45.
- **Paso 3 de la calibración, «junta los dedos»** (`desktop/panel.go`, vista
  de cámara): 8 cuadros con las puntas juntas (≤ 0,22 y que no varíen más de
  0,08); se guarda la mediana en `panel.json` (`pellizco_cerrado`).
  `AjustesPellizcoPara` fija cerrar = medida + 0,11 (entre 0,18 y 0,30) y
  firme = medida + 0,03 (entre 0,08 y cerrar − 0,06). El dueño de las tomas
  (~0,09) da los mismos 0,20/0,12 que por defecto. Un `panel.json` de antes,
  sin este campo, sigue valiendo con los umbrales por defecto.
- El mismo código de `gestures` y `desktop` corre en Linux y en Windows.

## Medición (`./dev replay -toques`)

| | aciertos | falsos | de más | bordes | salto p50 | salto p90 |
| --- | --- | --- | --- | --- | --- | --- |
| antes (master, uy84og) | 69/102 | 3 | 14 | 23/23 | 0,0108 | 0,0366 |
| cerrar 0,20 | 67/102 | 1 | 13 | 23/23 | 0,0108 | 0,0338 |
| **cerrar 0,20 + firme 0,12** | **68/102** | **1** | **14** | **23/23** | **0,0097** | 0,0533 |

Los toques falsos bajan de 3 a 1 y los bordes no cambian. Se pierde 1 acierto
de 102. El p90 del salto sube porque con el firme aparecen 2 toques nuevos
(n = 59 en vez de 56) que no tenían ancla. En los toques que ya existían, el
salto baja: el dedo apoya antes, cuando la palma todavía no se ha movido.

**Barrido** (cerrar × firme, el resto igual): con cerrar 0,22 y firme 0,12 sale
69/102 y 1 falso. Con cerrar 0,18 bajan los aciertos (66–67) y con 0,16 caen
mucho más (61). Con firme 0,14 sube el «de más». Elegí 0,20 porque es lo que
pedía la tarea y sale a un acierto del mejor. La calibración lo ajusta a cada
mano.

**Calibración simulada** (se aplica a todas las tomas la medida del dueño):
con medidas entre 0,04 y 0,20 salen 65–68 aciertos, 1 falso y 23/23 bordes.
Medir de más o de menos no empeora nada respecto a los umbrales por defecto.

**Apoyo al primer cuadro:** en las tomas, el dedo apoya al 1.er cuadro cerrado
en 22 de 71 toques; antes, en 5. La mediana entre cerrar y apoyar sigue en
~133 ms porque casi todos los pellizcos de las tomas se cierran despacio
(pasan primero por el tramo tibio). Las tomas no tienen taps de 1–2 cuadros.
Eso lo cubre `TestEmbragueFirmeApoyaAlPrimerCuadro`: firme apoya y se levanta,
tibio espera 3 cuadros, 0,23 (los dedos sin tocarse) no apoya nunca y un firme
que empieza fuera del panel tampoco apoya.

## Lo que falta (necesita la mano y los equipos)

- **`./dev pista`** con esta rama (14 blancos y 10 min de reposo) en Linux y en
  Windows, frente a la línea base del 19-nov (11/14 a la primera, 0 falsos).
  Desde el sandbox no hay cámara ni navegador para la pista.
- **Windows:** el código compila para Windows (`GOOS=windows go build ./...`,
  `go vet` de la app) y es el mismo que en Linux, pero no se pudo correr
  `./dev win test` porque no hay ssh al PC desde el sandbox.
- Comprobar en Firefox que el tap rápido no abre el menú contextual: el dedo
  sigue apoyado al menos los 250 ms del soltar.
- Las tomas nuevas deberían grabar taps rápidos y guardar `pellizco_cerrado`
  en la cabecera. Así el replay mediría el nivel firme y la calibración con
  datos reales y no solo con la prueba sintética.
