// Package desktop es la lógica de la app de escritorio de Lazul (el
// prototipo de factibilidad, cotejo-egtbal): todo lo que no necesita la
// ventana, para probarlo sin Wails ni cámara.
//
//   - Vivo (vivo.go): cuadro a cuadro, de los puntos de MediaPipe a la pose,
//     el modo y los gestos (gestures.Clasificar), y sigue cada activación
//     hasta saber en qué terminó.
//   - Registro (registro.go): el archivo local de la prueba de factibilidad
//     (JSONL). Nunca lleva imágenes, URL ni nada de lo que hay en pantalla.
//   - Resumen (resumen.go): activaciones en los últimos 30 min de uso, para
//     medir «menos de 1 activación falsa en 30 min» (dec-q1p7).
//   - Nucleo (nucleo.go): cada equipo es receptor (escucha en el puerto de
//     LocalSend, empareja con código y permiso explícito) y controlador (la
//     cámara: los gestos van al Enrutador y salen como Órdenes al receptor
//     emparejado).
//
// La ventana (Wails, módulo aparte para no meter cgo en ./dev check) está en
// desktop/app; cómo usarla, en desktop/README.md.
package desktop
