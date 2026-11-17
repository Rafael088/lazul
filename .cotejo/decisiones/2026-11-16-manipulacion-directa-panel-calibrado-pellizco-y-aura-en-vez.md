---
id: dec-fa72
fecha: 2026-11-16
temas: [gestos, modo-tactil, interaccion]
estado: aceptada
autor: opus
aceptada_el: 2026-11-16
---
# Manipulación directa: panel calibrado, pellizco y aura en vez del agarre de tres dedos

El modo táctil 0.2.0 traduce formas de la mano a atajos (gesto como orden). Los ensayos del 16-nov fallaron por síntomas de ese modelo: el regreso de la mano contaba al revés, el agarre de tres dedos no salía y tardaba, y sin respuesta visual no se sabía qué veía la cámara. Se cambia a manipulación directa: (1) un panel virtual delante de la persona, calibrado una vez con dos puntos (esquina superior izquierda e inferior derecha), se mapea a la pantalla y mueve el ratón virtual uinput en posición absoluta, con filtro One Euro contra el temblor; la profundidad sale del tamaño aparente de la mano medido en la calibración, sin modelo 3D del cuarto; (2) pellizco pulgar-índice = agarrar, soltar = embrague (el regreso de la mano no cuenta); dos manos pellizcando = zoom; (3) aura en pantalla con tres estados: fuera, dentro del panel y agarrando; (4) la mano solo cuenta delante del torso (MediaPipe Pose) para evitar activaciones falsas. Límite aceptado: sirve para movimientos amplios (desplazar, arrastrar, zoom, cambiar de espacio), no para clics de precisión. El hardware propio (Pi o cámara de profundidad, Bluetooth HID) queda como idea hasta que esto funcione con la webcam.
