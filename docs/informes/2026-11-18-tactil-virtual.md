# Táctil virtual en Linux: prueba de factibilidad (18-nov-2026)

**Pregunta:** en vez de mandar clic, flechas o rueda, ¿puede Lazul hacerse pasar
por una pantalla táctil para que cada app interprete los toques a su manera
(scroll con inercia, zoom de pellizco, doble toque, pulsación larga)?

**Respuesta: sí.** Un dispositivo uinput multitoque (protocolo B,
`INPUT_PROP_DIRECT`, 2 ranuras) creado por el usuario de la sesión (la ACL de
`/dev/uinput` ya lo permite) aparece en Hyprland 0.57.2 como `Touch Device`
sin configurar nada, y las apps lo reciben como toques de verdad.

Prototipo: `prototipos/tactil/` (`go run ./prototipos/tactil tap 0.5 0.5`;
órdenes `tap`, `doble`, `largo`, `arrastrar`, `pellizco`, `guion`).
Coordenadas normalizadas 0..1 sobre el monitor.

## Resultados

| Toque inyectado | Chromium 156 (Wayland) | Firefox 160 | Papers (PDF) |
| --- | --- | --- | --- |
| tap (60 ms) | `pointerdown/up` tipo `touch` + `click` | igual | — |
| doble (2 × 60 ms, 120 ms entre) | `dblclick` | `dblclick` | — |
| largo (900 ms) | `contextmenu` a ~660 ms | `contextmenu` a ~510 ms (abre el menú real, que se queda con los toques siguientes hasta tocar fuera) | — |
| arrastre de 0,85 a 0,2 de la pantalla en 300 ms | scroll 0 → 1282 px, con inercia ~1 s después de soltar | 0 → 1240 px, con inercia | baja de la página 1 a la 2 |
| pellizco de 5 % a 30 % de separación en 700 ms | zoom nativo 1,00 → 3,45 | zoom nativo hasta 4,75 | zoom nativo del documento |

Latencia no medida; a ojo, el `click` sale al levantar el dedo (60 ms después del `pointerdown`).

## Lo que cambia para Lazul

- El scroll y el zoom dejan de ser imitaciones (3–8 flechas, Ctrl+más): la app
  aplica su propia física, igual que con una pantalla táctil real.
- El panel calibrado ya es el «vidrio»: cada punto del panel es un punto de la
  pantalla, que es justo lo que pide un táctil absoluto.
- Dos manos = dos dedos: el zoom sale gratis.
- Propuesta de gestos sobre el pellizco como «dedo apoyado»: pellizco corto
  quieto = tap, sostenido > 0,5 s = pulsación larga, dos en < 0,3 s = doble tap,
  pellizco y mover = arrastre/scroll, pellizco a dos manos = zoom.

## Límites y pendientes

- Un toque va a la superficie que está debajo, no a la ventana con foco: el
  panel tiene que cubrir la pantalla en la que se trabaja (con varios monitores,
  Hyprland asigna el táctil a uno; `input:touchdevice:output`).
- No hay cursor visible mientras se toca: hará falta el aura (o un indicador
  propio) para saber dónde va a caer el dedo **antes** de apoyarlo.
- La pulsación larga abre menús de contexto reales; conviene subir su umbral o
  no producirla por accidente (el pellizco sostenido para arrastrar la dispara
  si la mano no se mueve).
- Windows sin probar: `InjectTouchInput` / `CreateSyntheticPointerDevice`
  funcionan sin pantalla táctil, pero falta verificarlo en el PC.
- La profundidad (plano inclinado, «tocar el aire») no se probó: depende de la
  `z` de MediaPipe, que es ruidosa; medirla antes con la grabadora (tamaño de la
  palma como alternativa).
