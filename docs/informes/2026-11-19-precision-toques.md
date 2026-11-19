# Precisión de punta a punta con las tomas reales (cotejo-uy84og, 19-nov-2026)

**Pregunta:** ¿qué parte del camino de la mano al dedo táctil hace fallar los
criterios 1 a 3 de dec-uaex (tocar a la primera 9 de 10, los 4 bordes 10 de
10, menos de 1 toque falso en 10 min), y cuánto se arregla sin datos
sintéticos?

## La métrica (fijada antes de tocar nada)

`./dev replay -toques` termina ahora con una línea `PRECISIÓN` sobre las 145
tomas reales de `fixtures/pellizco`, pasando por el mismo recorrido que la app
(filtro del panel con banda exterior antes del embrague, embrague, `ToquesDe`
y el puntero con su One Euro). Código: `gestures/precision_toques.go`.

| Cifra | Qué cuenta |
| --- | --- |
| aciertos | tomas de arrastre con exactamente un toque que es arrastre en la dirección pedida, y de zoom con exactamente dos dedos apoyados a la vez (102 tomas) |
| falsos | toques en las tomas que no piden pellizcar: entrar y salir del panel, movimiento normal (43 tomas) |
| de más | toques sobrantes en las tomas con gesto: cortes del parpadeo o pellizcos involuntarios |
| bordes | salidas del panel por un borde con el puntero ya en ese borde (alcance, criterio 2) |
| salto al apoyar | cuánto se mueve el puntero del último cuadro con la mano abierta al apoyo del dedo, en unidades del panel (0,01 ≈ 19 px en 1920) |

Las tomas no guardan blancos de pantalla, así que «aciertos» mide que el toque
salga como se pidió y el salto mide la puntería; la pista (`./dev pista`) sigue
siendo la prueba de verdad con la mano.

Línea base de la pista con la app de master (19-nov, `2026-11-19-pista-linea-base.json`):
11/14 a la primera, 4 esquinas a la primera, 0 toques falsos en 10 min de reposo.

## Antes y después

| | aciertos | falsos | de más | bordes | salto p50 | salto p90 |
| --- | --- | --- | --- | --- | --- | --- |
| línea base (master) | 63/102 (62 %) | 4 | 24 | 23/23 | 0,0113 | 0,1155 |
| 1. armar al 2.º cuadro cerrado | 64/102 | 4 | 21 | 23/23 | 0,0180 | 0,1987 |
| 2. el dedo cae en el ancla | 64/102 | 4 | 21 | 23/23 | 0,0107 | 0,1062 |
| 3. el puntero se queda en el ancla al cerrar | 64/102 | 4 | 21 | 23/23 | 0,0086 | 0,0648 |
| 4. armar al 3.er cuadro cerrado | 65/102 | 3 | 18 | 23/23 | 0,0110 | 0,0777 |
| 5. soltar tras 250 ms (antes 200) | **69/102 (68 %)** | **3** | **14** | **23/23** | **0,0108** | **0,0366** |

Cada fila es un commit con su medición. Probado y descartado (medido igual):

| Cambio | Resultado | Por qué no |
| --- | --- | --- |
| reiniciar el One Euro del puntero al apoyar | salto p50 0,0139 | el filtro es lo que la persona ve al apuntar |
| armar en toda la banda exterior (0,15) si la mano ya entró | 65/102, falsos 5, de más 27 | suma un toque falso |
| armar hasta 0,03 / 0,05 / 0,08 fuera del borde | 65 / 66 / 65, falsos 5 | cada margen suma un toque falso |
| soltar tras 300 ms | 71/102, de más 11 | un tap dura al menos el soltar; Firefox abre el menú de contexto a ~510 ms |
| armar al 4.º cuadro | 62/102 | pierde aciertos |
| ventana del ancla 600–800 ms | bordes 22/23 | pierde un borde |
| tolerancia de profundidad «lejos» 0,45–0,55 | sin cambio en las tomas | ver pendientes |
| tolerancia «cerca» 0,5 / 0,7 | 65 / 66, bordes 22/23, falsos 5 con 0,7 | pierde un borde o suma un falso |

## Lo que cambió

- **Apoyar exige 3 cuadros cerrados seguidos** (`CuadrosParaApoyar`): el
  pellizco que parpadea uno o dos cuadros salía como un tap de 200 ms.
- **El dedo cae en el ancla** (`MsAncla`, 400 ms): cerrar el pellizco mueve
  la palma; ahora el dedo cae donde estaba el puntero con la mano abierta y el
  arrastre sigue desde ahí con el mismo recorrido. Mientras se cierra, el
  puntero (y el aura) se queda en el ancla; en las tomas sin pellizco eso pasa
  en 18 de 1044 cuadros dentro del panel.
- **Soltar tras 250 ms abierto** (antes 200): el parpadeo de 3 cuadros a
  15 fps ya no corta el arrastre.
- Los bordes (o7mvnh) no se tocaron: 23/23 antes y después.

## Lo que falta

- **Validar con la mano**: repetir `./dev pista` (14 blancos y 10 min de
  reposo) con esta rama. Lo que más debería notarse: íconos pequeños de borde y
  centro (el dedo ya no salta al cerrar) y menos taps sueltos.
- **Tap frente a pulsación larga**: un tap ahora dura al menos ~250 ms más lo
  que se sostiene el pellizco. Probar en Firefox que un tap rápido no abre el
  menú de contexto.
- **Pellizcos que empiezan fuera**: es la mayor causa de fallo que queda (19 de
  las 33 tomas de gesto fallidas no dan ningún toque por eso; el resto son
  tomas con dos pellizcos de verdad o zoom en que una mano se sale). Unas son de profundidad («cerca»), pero la
  calibración de las tomas se reconstruye con el tamaño de palma mediano de
  todas, así que la profundidad de las tomas no es fiable; otras empiezan justo
  fuera del borde y armarlas suma falsos.
- **Profundidad «lejos»**: el registro del 19-nov tiene 37 manos fuera por
  lejos; ensancharla no cambia las tomas (no tienen ese caso). Para medirlo hace
  falta que las tomas nuevas guarden el tamaño de palma de la calibración.
- **One Euro**: sin tocar. Las tomas no dicen cuánto retraso molesta; afinarlo
  contra el salto premiaría filtrar de más.
