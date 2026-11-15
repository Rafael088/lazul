# Gestos táctiles v0.2.0

El clasificador táctil es independiente del clasificador de transferencia de
la v0.1.0. La L de `dec-rwrs`, sostenida 400 ms, alterna el modo. Con el modo
activo reconoce:

- dos dedos en vertical: scroll arriba o abajo;
- un pellizco que cambia su apertura: zoom que abre o cierra;
- dos dedos en horizontal: barrido a izquierda o derecha.

No existe una salida de cursor ni de clic. Después de una acción hay que dejar
la pose durante 250 ms antes de repetirla; la L necesita 800 ms fuera antes de
que pueda volver a alternar el modo.

## Umbrales contra activaciones falsas

Las distancias se expresan en largos de palma, por lo que no dependen de la
distancia de la mano a la cámara.

| Gesto | Umbral |
| --- | --- |
| Todos los movimientos | pose estable 120 ms y final antes de 1.000 ms |
| Scroll | 0,70 palmas; el eje vertical domina al horizontal por 1,5× |
| Barrido | 1,20 palmas; dos dedos; el eje horizontal domina por 1,5× |
| Zoom | cambio de apertura de 0,90 palmas; centro se mueve como máximo 0,55 |
| Zoom que abre | empieza con apertura ≤ 0,25 palmas |
| Zoom que cierra | empieza con apertura ≥ 0,95 palmas |

El zoom exige además el índice doblado más de 25°. Esto evita confundir
dos señalamientos reales con un pellizco que cierra. El barrido usa dos dedos
porque una palma abierta en horizontal aparece al saludar y produjo falsos con
el primer diseño.

## Resultados reproducibles

Comando:

```sh
./dev replay -tactil -v
```

Resultado del 15 de noviembre de 2026:

- reales (`fixtures/gestos`): L 25/25; 0 falsos en 157 grabaciones sin gesto;
  0 falsos al forzar el modo activo en las 51 negativas reales (8,4 min);
- sintéticas anteriores: L 8/8 y 0 falsos en 47 grabaciones sin gesto;
- sintéticas táctiles nuevas: modo 8/8, scroll 8/8, zoom 8/8 y barrido 8/8;
  0 falsos en 24 negativas de umbral.

Las negativas nuevas cubren recorrido corto, diagonal, un solo dedo, zoom
corto, pinza trasladada y saludo con la palma. Las grabaciones reales que ya
había permiten validar la L y las falsas activaciones, pero el repositorio no
traía tomas reales positivas de scroll, zoom o barrido; esas tres clases
positivas se validan aquí con grabaciones sintéticas, sin presentarlas como
tomas de cámara.
