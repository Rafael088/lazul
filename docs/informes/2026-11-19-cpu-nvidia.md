# CPU de la app con NVIDIA (19-nov-2026)

## Línea base

Las muestras reales del 18-nov con `./dev app` y una mano dentro del panel dieron
26–30 cuadros/s y 126–183 % de un núcleo con MediaPipe en la NVIDIA. En la Intel,
el mismo recorrido rondó 45 %. Son las muestras de referencia porque este
sandbox no expone la NVIDIA, `/dev/dri` ni el socket de Hyprland.

## Qué se separó

`prototipos/camara -modo imagen` ahora tiene ocho variantes: inferencia sola
(`base`), JSON hacia Go (`envio`), vista de vuelta (`respuesta`), indicador,
aura de 240 px, copia entre lienzos, recorrido anterior completo y recorrido
optimizado. Cada resultado guarda fps, CPU del árbol de procesos y p50/p95 del
puente a Go.

En la app se quitaron tres trabajos de la ruta de cada cuadro:

- Go ya no devuelve la vista completa: el indicador y la vista de ajuste usan
  el sondeo de estado existente cada 500 ms.
- El JPEG solo se copia al lienzo cuando la vista de cámara está abierta;
  MediaPipe sigue recibiendo el bitmap directamente cuando está cerrada.
- El aura conserva sus tres estados y sus 240 px, pero elimina estados
  idénticos y limita solo el movimiento a 25 actualizaciones/s. Mostrar,
  agarrar y ocultar siguen siendo inmediatos.

## Repetición en la NVIDIA

Desde la raíz, una sola orden compila el banco, crea la pantalla virtual visible
y corre todas las variantes:

```sh
./dev camara-cpu 20 19nov
```

Deja `prototipos/camara/resultados/partes-19nov-*.json` e imprime la tabla. El
criterio es que `despues` quede en al menos 25 cuadros/s y use menos CPU que
`antes`. La ejecución visible queda pendiente del maestro fuera del sandbox;
no se inventan cifras antes/después sin esa GPU.

## Medida disponible sin GPU

El coste puro de codificar el JSON se puede repetir con:

```sh
cd prototipos/camara
go test -run '^$' -bench 'BenchmarkJSON' -benchmem
```

Esta microprueba separa cuadro hacia Go y vista hacia la webview; no sustituye
la medida del árbol WebKit/MediaPipe en la NVIDIA. En el i5-11300H del sandbox,
con dos manos y Pose, dio 13,1 µs y 1.632 B por cuadro hacia Go; la vista de
vuelta dio 2,2 µs y 530 B. El puente y las superficies no están incluidos.

## Medida en la NVIDIA (maestro, fuera del sandbox, 19-nov)

`./dev camara-cpu 20 19nov` (banco `-modo imagen`, 20 s por variante, delegado GPU):

| variante  | fps  | CPU % núcleo |
|-----------|------|--------------|
| base      | 54,9 | 148,9 (primera corrida, con calentamiento de compilación) |
| envio     | 45,2 | 81,5 |
| respuesta | 49,7 | 86,7 |
| indicador | 53,6 | 96,5 |
| aura      | 60,0 | 97,5 |
| copia     | 60,0 | 95,9 |
| antes     | 53,2 | 94,2 |
| despues   | 53,1 | 93,1 |

Lectura: en el banco, antes y después quedan iguales (~94 % de un núcleo a
~53 fps, el doble de cuadros que la app real). Ninguna de las partes aisladas
explica por sí sola los 130-180 % de la app: el grueso parece estar en
WebKit + MediaPipe, no en el puente ni en el aura. Los cambios no empeoran
nada, pero el ahorro real solo se sabrá con la app y la cámara (`./dev app`,
mano dentro, y la línea de CPU del indicador o `./dev app resumen`).
