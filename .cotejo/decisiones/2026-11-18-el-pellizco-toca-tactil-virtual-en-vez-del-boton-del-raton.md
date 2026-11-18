---
id: dec-yp1a
fecha: 2026-11-18
temas: [modo-tactil, interaccion, linux]
estado: propuesta
autor: agente
---
# El pellizco toca: táctil virtual en vez del botón del ratón

La prueba del 18-nov (docs/informes/2026-11-18-tactil-virtual.md) mostró que un táctil multitoque uinput lo reconoce Hyprland sin configurar y que Chromium, Firefox y Papers responden con tap, doble tap, pulsación larga, scroll con inercia y zoom de pellizco nativos. Por eso la salida del panel cambia: el puntero absoluto sigue a la palma solo para mostrar dónde caerá el dedo; el pellizco armado apoya un dedo táctil en ese punto, moverlo arrastra y soltarlo lo levanta; con dos manos son dos dedos y el zoom lo hace la app. Tap, doble tap y pulsación larga no se clasifican: los reconoce cada app como en una pantalla táctil. El botón del ratón y Ctrl+más/menos quedan como respaldo si no se puede crear el táctil. Windows (InjectTouchInput) y la profundidad con plano inclinado quedan fuera de esta versión.
