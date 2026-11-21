# Lag intermitente de MediaPipe (21-nov-2026)

## Diagnóstico

El registro real de `~/.local/state/lazul/factibilidad.jsonl` separa la causa
principal de un problema adicional de monitores:

- En la sesión `20261121-160410.188`, entre 16:04:32 y 16:07:37, las 38
  muestras promedian **22,6 fps**, Hand **29,6 ms**, Pose **6,9 ms** y CPU
  **120,4 % de un núcleo**. Desde 16:07:42, las 52 muestras siguientes se
  recuperan a **29,1 fps**, Hand **16,6 ms**, Pose **7,1 ms** y CPU **108,4 %**.
  El peor punto fue 16,2 fps con Hand en 41,6 ms.
- El período de cada cuadro se correlaciona mucho más con Hand (0,849) que
  con Pose (0,558). La CPU no se saturó y MediaPipe había anotado delegado
  `GPU`; Hand y Pose se alargaron a la vez. Eso apunta a contención o
  limitación temporal de la GPU, no a Go, al puente ni a la cámara.
- No hay evidencia de carga por dos manos: durante todo el tramo lento los
  eventos de panel anotaron `"dedos":1` y nunca hubo un apoyo de dos dedos.
  El registro histórico no guardaba cuántas manos detectó MediaPipe, así que
  no permite descartarlo por completo. Las muestras nuevas incluyen
  `manos_media` y `pose_cada` para resolverlo sin guardar puntos ni imágenes.
  Se conserva `numHands: 2`, necesario para el zoom.
- El 20-nov repite el mismo patrón en episodios cortos: las 9 muestras por
  debajo de 25 fps promedian 21,5 fps, Hand 25,5 ms y Pose 7,5 ms. El error
  aislado `consultar los monitores con hyprctl: signal: terminated` ocurrió
  después, a las 23:18:27. No puede explicar `mano_ms`: ese cronómetro se
  cierra en JavaScript antes de entrar a Go, pero la consulta sí podía
  congelar hasta un segundo el procesamiento del cuadro.

## Cambios

- Cuando la media suavizada de Hand llega a 24 ms, Pose pasa de una ejecución
  cada 3 cuadros a una cada 6. El torso cambia despacio y se reutiliza entre
  inferencias; al recuperarse Hand vuelve automáticamente a 1/3.
- En Windows el temporizador ya descuenta el tiempo consumido por MediaPipe.
  Antes esperaba 33 ms *después* de inferir; ahora apunta a 30 Hz y, si llega
  tarde, continúa sin intentar una ráfaga atrasada.
- Listar monitores, seguir el foco y ejecutar un barrido se hacen fuera de
  `Pantallas.Cuadro`. Las consultas tienen tiempo límite y no se solapan; un
  barrido que llega mientras se consulta queda pendiente. La prueba bloquea
  `List` y comprueba que el cuadro vuelve en menos de 50 ms (antes tardaba
  1,000 s).

## Medición repetible

El análisis del registro y el presupuesto conservador del cambio se repiten
sin la cámara:

```sh
prototipos/camara/analizar-lag.py ~/.local/state/lazul/factibilidad.jsonl 20261121-160410.188
```

Para las 35 muestras con Hand ≥ 24 ms da 22,2 fps, Hand 30,4 ms y Pose
7,0 ms. Espaciar Pose libera **3,5 ms por cuadro** y, aun suponiendo que Hand
no se acelera nada, lleva el modelo a **24,1 fps**. La cifra real requiere la
GPU; este cálculo solo acota el ahorro directo.

La línea base guardada del banco sigue pasando en ambos sistemas:

```text
python3 prototipos/camara/verificar.py
PASA win-camaramano-gpu
PASA wl-procmano-gpu
```

Los resultados guardados son 16,6 fps / inferencia p50 14,5 ms en Windows
(la cámara entregó 16,6 fps) y 29,83 fps / p50 15 ms en Hyprland. Para medir
la protección en la GPU real, manteniendo dos manos y cambiando solo Pose:

```sh
prototipos/camara/medir-lag.sh 30 21nov-lag
```

El comando deja `resultados/21nov-lag-{antes,despues}.json` y compara fps,
p50 de Hand/Pose y CPU. Este sandbox no expone `/dev/dri`, `/dev/video*` ni el
socket de Hyprland, por lo que no se inventa una cifra nueva de GPU; la prueba
automatizada cubre la decisión adaptativa y el reloj de Windows.

En Windows, con `camara.exe` ya compilado, la misma comparación es:

```powershell
.\medir-windows.ps1 imagen 30 GPU 21nov-lag-antes 2 3
.\medir-windows.ps1 imagen 30 GPU 21nov-lag-despues 2 6
```
