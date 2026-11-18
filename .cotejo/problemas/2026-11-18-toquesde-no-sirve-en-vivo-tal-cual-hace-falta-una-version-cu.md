---
id: pro-8pqe
fecha: 2026-11-18
temas: [modo-tactil, gestos]
estado: abierto
---
# ToquesDe no sirve en vivo tal cual: hace falta una versión cuadro a cuadro (para cotejo-8fufqk)

gestures.ToquesDe recibe toda la secuencia de ResultadoEmbrague y no guarda estado entre llamadas (TestToquesDeElPrefijoNoCambiaLosCuadros lo fija). En la app habría que pasarle todo el historial en cada cuadro: costo que crece con el tiempo (30 fps × 10 min = 18 000 cuadros por llamada). Para cotejo-8fufqk: sacar el cuerpo del bucle a un tipo con estado (p. ej. TraductorToques con Cuadro(ResultadoEmbrague) CuadroToques y Parar() que devuelve el cuadro que levanta todos los dedos) y dejar ToquesDe como envoltorio que lo usa, sin cambiar sus pruebas. Desktop usa el tipo con estado, un cuadro a la vez.
