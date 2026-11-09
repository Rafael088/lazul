---
id: dec-9xxg
fecha: 2026-11-08
temas: [gestos, clasificador]
estado: propuesta
autor: sonnet
---
# Clasificador: el puño exige índice recogido y sostén mayoritario de puño

Refina dec-rwrs sin reemplazarla. Causa vista en las grabaciones reales de mandar app: al pinzar, el índice se dobla a medias y la pose salía «puño»; además, dos cuadros sueltos de puño unidos por la gracia (250 ms) contaban como un puño sostenido y AGARRAR disparaba antes de que la pinza se arrastrara. Cambios: (1) el puño exige el índice recogido de verdad (extensión < 0,0 largos de palma; antes < 0,30). Los puños reales de AGARRAR/SOLTAR tienen mediana de -0,12 a -0,47; los de mandar-app-buena-022 estaban en +0,25. (2) AGARRAR exige que al menos 70 % de los cuadros del sostén de 150 ms sean puño (meseta estable entre 60 y 80 %). Efecto: ./dev replay reales 93,9 a 95,4 % (MANDAR_APP 42 a 44 de 47; AGARRAR 34/34, ACTIVAR 25/25, SOLTAR 22/25), sintéticas 100 %, 0 falsos positivos en 51 tomas sin gesto y 5,2 min de movimiento normal. Costo: un puño dudoso (índice a medio doblar) ya no agarra; probado y descartado: que la pinza gane al puño con índice más doblado (indicePinzaMin -0,15 a -0,30 rompe AGARRAR reales) y bajar el umbral de arrastre (rompe agarrar-buena-012). Sin resolver, límite conocido: tomas donde la mano descansa cerrada más de 1 s antes de pinzar (mandar-app-buena-005 y 006) son idénticas, en pose, a un AGARRAR real; y la 047 (pinza que se abre mientras arrastra) no se arregla sin romper agarrar-mala-010. Las 3 de soltar con luz mala (MediaPipe pierde la mano) se dejan.
