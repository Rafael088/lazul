# De punta a punta en 2 PCs: precisión y latencia (cotejo-sb2kdm)

Medido el 08-nov-2026 entre los 2 PCs de la fase: Linux (Hyprland, por
Wi-Fi) y Windows, en la misma red local. Rama `cotejo/cotejo-sb2kdm`.

## Las dos métricas de la fase

| Métrica | Meta | Resultado | ¿Pasa? | Cómo se midió |
| --- | --- | --- | --- | --- |
| Precisión de gestos (grabaciones reales) | ≥ 95 % | **93,9 %** (124 de 131) | **No** | `./dev replay` sobre las 182 tomas de `fixtures/gestos`, sin quitar ninguna |
| Latencia del comando: gesto reconocido → Orden ejecutada en el otro PC (URL y app del gesto 4) | < 100 ms | **p50 2,5–3,3 ms · p95 3,1–9,4 ms** | **Sí** | entre los 2 PCs reales, en los dos sentidos; el receptor anota lo que abriría en vez de abrirlo |
| Lo mismo, abriendo de verdad en pantalla | < 100 ms | **260–545 ms** (6 órdenes) | **No** | Windows `ShellExecute` y Linux `xdg-open` tardan eso en volver |
| Captura: soltar → captura guardada y comprobada en el otro PC | < 1 s (dec-q1p7) | Linux → Windows p50 78 ms · p95 119 ms (subiendo al soltar); p50 13 ms · p95 28 ms (adelantada al agarrar) | Sí (< 1 s) | entre los 2 PCs reales; capturas de 1 a 4 MB |

Falsos positivos: 0 en las 51 tomas sin gesto (0 por 30 min de movimiento
normal; meta < 1).

## Qué se midió de verdad y qué no

De verdad:

- **Los 2 PCs reales.** Un proceso controlador en un PC y un receptor en el
  otro, emparejados con el protocolo de siempre (código de 6 dígitos, prueba
  HMAC atada a la sesión TLS, huellas fijadas en los dos lados) y hablando por
  WebSocket sobre TLS 1.3. Se probó en los dos sentidos.
- **Las grabaciones reales de gestos.** Cada toma de `fixtures/gestos` pasa por
  `gestures.Clasificar`; en el cuadro en que reconoce el gesto arranca el
  reloj y el `Enrutador` del controlador manda la Orden. El reloj para cuando
  llega la Respuesta ok del otro PC, que ya hizo la orden.
- **Las capturas de verdad.** Linux con `grim` (1 a 4 MB por captura), Windows
  con su adaptador en la sesión del escritorio (40 a 56 KB: la pantalla de
  Windows estaba casi lisa). Viajan por el protocolo de LocalSend v2 y el
  receptor comprueba el sha256 antes de aceptar la Orden.
- **El catálogo de apps entre sistemas.** La ventana activa real de Windows
  (Windows Terminal por su AUMID, Edge por `msedge.exe`) llegó a Linux como
  `org.gnome.Terminal.desktop` y `microsoft-edge.desktop`; `firefox` de Linux
  llegó a Windows como `firefox.exe`.
- **Abrir en pantalla**, pocas veces para no llenar los escritorios: en
  Windows 2 capturas (visor de fotos), una URL y Firefox; en Linux una captura
  (imv) y una URL.

No de verdad (o solo en parte):

- **La cámara y MediaPipe no están en la medición.** Las grabaciones ya traen
  los puntos de la mano. La latencia empieza en el gesto reconocido, como pide
  la tarea; no cuenta lo que dura el gesto (los 400 ms de la L, el arrastre) ni
  lo que tarda la webview en dar los puntos. La webview → núcleo todavía no
  existe (`desktop/` está vacía).
- **Las tomas son sueltas.** Cada grabación trae un gesto (la de soltar arranca
  agarrando), así que «L → puño → soltar» se arma juntando una de agarrar con
  una de soltar, no con una toma continua.
- **Niveles 1 y 2 del gesto 4 (URL y archivo) con la ventana puesta a mano.**
  Los adaptadores todavía no leen la URL del navegador ni el documento abierto
  (`platform.App.URL` y `Archivo` quedan vacíos en Linux y Windows). El núcleo
  ya resuelve los tres niveles; para medirlos, el controlador rota entre una
  URL, un archivo y una app puestos a mano y la ventana real (`-ventanas`).
- **La mayoría de las órdenes no abre nada en pantalla.** El receptor de
  medición (`-abrir=false`) hace todo lo demás de verdad (recibe el archivo,
  comprueba el sha256, traduce la app por el catálogo) y anota lo que habría
  abierto. Se abrió de verdad solo en las 6 órdenes de «Abriendo de verdad».
- **Linux → Windows va por un túnel de ssh.** El cortafuegos de Windows no deja
  entrar conexiones nuevas (perfil privado, sin regla para Lazul) y abrir un
  puerto es un cambio de seguridad que decide el dueño. El túnel suma el
  cifrado de ssh. Windows → Linux va directo por la red local al puerto 53317
  (el de LocalSend, el único que deja entrar el cortafuegos de Linux).
- **La app oficial de LocalSend no se probó.** Ver «LocalSend» abajo.

## Precisión (`./dev replay`)

| Gesto | Aciertos |
| --- | --- |
| ACTIVAR (la L) | 25/25 |
| AGARRAR (puño) | 34/34 |
| SOLTAR | 22/25 |
| MANDAR_APP (pinza y arrastrar) | 42/47 |
| **Total** | **124/131 = 93,9 %** |

Las 7 que fallan: 4 de mandar app salen como AGARRAR (005, 006, 008, 022), 1
de mandar app no sale (047) y 3 de soltar con luz mala no salen (003, 004,
005). Las sintéticas dan 100 % (55 tomas), pero no cuentan para la meta. Este
trabajo no tocó el clasificador ni las grabaciones.

De punta a punta, como cada intento de captura junta agarrar y soltar, salen
22 de 25 capturas y 42 de 47 gestos 4 (los mismos fallos de arriba).

## Latencia entre los 2 PCs

Ping (ida y vuelta de un Sobre, 50 muestras): p50 2,4–3,0 ms, p95 3,1–5,2 ms.

### Linux controla, Windows recibe (túnel de ssh)

| Caso | n | gesto → Respuesta ok p50 | p95 | máx |
| --- | --- | --- | --- | --- |
| Gesto 4 · URL | 9 | 2,6 ms | 4,0 ms | 4,0 ms |
| Gesto 4 · app (catálogo) | 21 | 3,3 ms | 9,4 ms | 18,3 ms |
| Gesto 4 · archivo (34 KB por LocalSend) | 12 | 14,3 ms | 54,3 ms | 56,2 ms |
| Captura, subida al soltar (1–4 MB) | 22 | 78,2 ms | 118,7 ms | 132,0 ms |
| Captura, adelantada al agarrar | 22 | 12,6 ms | 27,9 ms | 76,9 ms |

Leer la pantalla con `grim` al agarrar: p50 283–313 ms (no entra en la
latencia de soltar, pasa al agarrar).

### Windows controla, Linux recibe (red local directa)

| Caso | n | gesto → Respuesta ok p50 | p95 | máx |
| --- | --- | --- | --- | --- |
| Gesto 4 · URL | 9 | 2,5 ms | 3,1 ms | 3,1 ms |
| Gesto 4 · app (catálogo) | 21 | 2,6 ms | 4,7 ms | 8,2 ms |
| Gesto 4 · archivo (34 KB por LocalSend) | 12 | 9,6 ms | 10,6 ms | 57,2 ms |
| Captura, subida al soltar (~56 KB) | 22 | 11,4 ms | 24,3 ms | 35,6 ms |
| Captura, adelantada al agarrar | 22 | 10,2 ms | 117,1 ms | 117,5 ms |

En la última fila, 2 de 22 Órdenes tardaron ~117 ms solo en el viaje de la
Orden (la subida ya había terminado): con el soltar a su ritmo real la red
queda quieta entre gestos, y esas esperas apuntan al ahorro de energía del
Wi-Fi. No se investigó más.

### Abriendo de verdad

| Dónde | Qué | Respuesta ok | De eso, en el receptor |
| --- | --- | --- | --- |
| Windows | 2 capturas (visor de fotos) | hasta 481 ms | 317 ms y 62 ms |
| Windows | URL | 532 ms | 467 ms |
| Windows | Firefox (catálogo) | 370 ms | 351 ms |
| Linux | captura (imv) | 545 ms | 506 ms |
| Linux | URL | 264 ms | 261 ms |

El tiempo se va en el sistema: `ShellExecute` en Windows y `xdg-open` en Linux
vuelven cuando la app ya agarró la orden. Para cumplir < 100 ms con apertura
real, el receptor tendría que contestar al despachar la apertura y avisar
después si falló. Queda propuesto, no hecho.

## Lo que se hizo

- **`core/enrutador.go`**: el controlador. Agarrar toma la captura de esta
  pantalla; soltar hacia un lado la manda al equipo de ese lado (con 2 PCs, el
  otro): los bytes por LocalSend y el aviso por `Entregar`. El gesto 4 manda
  la ventana activa por niveles (dec-r2ii): URL, documento (viaja por
  LocalSend y el otro lo abre) o app del catálogo. `Adelantar` sube la captura
  ya al agarrar cuando solo hay un destino posible.
- **`core/localsend/`**: info, prepare-upload, upload y cancel del protocolo
  LocalSend v2. El receptor guarda en una bandeja, comprueba largo y sha256,
  no pisa archivos ni deja salir el nombre de la carpeta, y da cada archivo por
  entregado una sola vez.
- **Receptor**: traduce la app por el catálogo (`catalogo_id` → el
  identificador de su sistema; si no la tiene, la web), acepta `Entregar
  archivo` solo si llegó entero con ese sha256 y lo abre.
- **Catálogo**: `PorNativo` (de la ventana activa al id del catálogo) y
  `Nativo` (del id al identificador de cada sistema).
- **Linux, `LaunchApp`**: `xdg-open` con imv no volvía hasta cerrar la imagen
  (el contrato dice que no se espera a que se cierre). Ahora espera como mucho
  500 ms y lo demás lo recoge en segundo plano. `./dev contrato` sigue en
  13/14 (la que abre una ventana se omite, como antes).
- **`./dev sim`** termina con el recorrido de punta a punta en local: las
  grabaciones reales, la captura por LocalSend y los 4 tipos del gesto 4
  (URL, documento, Code → `Code.exe` por el catálogo y Obsidian, que no está y
  cae a su web).
- **`./dev e2e win|linux`**: la medición entre los 2 PCs (comandos
  `core/cmd/controlador` y `core/cmd/receptor`).

## Seguridad y emparejamiento

No se cambió el emparejamiento ni la configuración TLS. Lo nuevo:

- LocalSend se sirve en el **mismo puerto y la misma configuración TLS** que
  las Órdenes (TLS 1.3, certificado de cliente obligatorio) y solo deja subir a
  un equipo emparejado; el cliente fija la huella del receptor. Pruebas:
  `TestLocalSendSoloEmparejados` (un no emparejado recibe 403 y no queda nada en
  la bandeja; un cliente que espera otra huella ni conecta) y
  `TestEntregarArchivoQueNoLlego`.
- El comando de medición `receptor -permitir HUELLA` da el permiso de
  emparejar sin preguntar, pero solo a esa huella (la del controlador que se
  espera). Sin la bandera pregunta en la terminal. Es solo para medir sin
  nadie al teclado; la app de verdad le preguntará a la persona.

## LocalSend

Los archivos van por el protocolo de LocalSend (dec-eof4), dentro de Lazul:
no hace falta instalar la app de LocalSend en ningún PC. Por ser un servidor
con certificado de cliente obligatorio, la app oficial de LocalSend (que no
manda certificado) **no** puede subirle archivos a un receptor de Lazul. El
cliente de Lazul sigue el protocolo para mandarle a la app oficial, pero eso
no se probó: la app pide aceptar en pantalla.

## Pendiente del dueño

1. **Precisión 93,9 % < 95 %.** Hacen falta más tomas o ajustar el
   clasificador con las que fallan (mandar app que sale como agarrar, soltar
   con luz mala).
2. **Cortafuegos de Windows.** Dejar entrar a Lazul (o al puerto 53317) en el
   perfil privado para medir Linux → Windows sin el túnel de ssh. No lo hice
   porque cambia la seguridad del PC.
3. **Probar con la cámara de verdad**, con la mano, de punta a punta (la
   webview todavía no existe).
4. **Abrir en < 100 ms**: decidir si el receptor contesta al despachar la
   apertura (propuesta de arriba).
5. **Niveles 1 y 2 del gesto 4 en los adaptadores**: leer la URL del navegador
   y el documento abierto. Hoy solo funciona el nivel 3 (app) con la ventana
   real.
6. Si interesa hablar con la app oficial de LocalSend, probarlo aceptando en
   su pantalla.

## Cómo repetirlo

```
./dev replay                       # precisión
./dev sim                          # punta a punta en local (simulado)
./dev e2e win                      # Linux controla, Windows recibe
./dev e2e win -adelantar -casos captura
./dev e2e linux                    # Windows controla, Linux recibe
LAZUL_ABRIR=1 ./dev e2e win -intentos 2 -ventanas url:https://example.org/,app:firefox
```

`./dev e2e` necesita `ssh win` y una sesión de escritorio abierta en Windows.
Lo recibido se borra al terminar.
