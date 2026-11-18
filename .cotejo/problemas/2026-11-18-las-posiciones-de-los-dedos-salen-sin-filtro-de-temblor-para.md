---
id: pro-7176
fecha: 2026-11-18
temas: [modo-tactil, gestos]
estado: abierto
---
# Las posiciones de los dedos salen sin filtro de temblor (para cotejo-8fufqk)

ResultadoEmbrague.Armadas lleva la posición cruda de cada mano (est.ultima), sin el One Euro que Panel.aplicar usa para el puntero. Con un táctil real, el temblor de MediaPipe puede pasar el margen con que las apps reconocen un tap (unos 8–10 px) y convertirlo en un arrastre corto, o mover la página al hacer pulsación larga. Para cotejo-8fufqk: filtrar cada dedo con su propio gestures.FiltroOneEuroPanel (uno por ranura, reiniciado al apoyar) antes de mandarlo a platform.TouchScreen, y que el punto donde se apoya sea el mismo que muestra el puntero en ese momento. Prueba: un pellizco corto con temblor sintético de ±1 % debe salir como apoyar y levantar en el mismo punto filtrado.
