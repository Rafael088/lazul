# Pista del 21-nov: temblor, aura nueva y asistencia de puntería

Pista (`./dev pista`) corrida a la vez en los dos PC el 21-nov, 18:51–18:54, con master `7b79bda`:
zona quieta del filtro y aro blanco (b61207a), toque con área de contacto
(cotejo-6eg99p) e imán sobre los botones (cotejo-mpvzi8, solo Windows).
Datos crudos: `2026-11-21-pista-linux.json`, `2026-11-21-pista-windows.json`.

| | 19-nov (línea base, Linux) | 21-nov Linux | 21-nov Windows |
|---|---|---|---|
| Aciertos a la primera | 11/14 (79 %) | 12/14 (86 %) | **14/14 (100 %)** |
| Toques fuera del blanco | 4 | 2 | 0 |
| Tiempo mediano por blanco | 5,8 s | 3,6 s | 3,6 s |
| Tiempo p95 | 15,4 s | 30,1 s | 11,0 s |
| Reposo sin toques falsos | 10 min, 0 falsos | 1,8 min, 0 falsos (cortado) | 21 s, 0 falsos (cortado) |
| fps (mediana / mínimo) | — | 30 / 12,6 | 17,4 / 14,4 |
| CPU de un núcleo (mediana) | — | 96 % | 45 % |

## Lo que dice

- **Más rápido:** la mediana por blanco bajó de 5,8 s a 3,6 s en los dos sistemas: el puntero quieto
  deja apuntar sin esperar a que se calme.
- **Windows, perfecto:** 14/14 sin un toque fuera. El imán ajustó 3 toques (`iman_ajusta` en el
  registro de la app).
- **Linux, dos fallos en el centro:** `icono-32` (centro abajo, 30 s) y `enlace-40` (centro
  izquierda, 12 s), uno con un toque fuera cada uno. Bordes y esquinas 8/8. En Linux no hay imán
  (mpvzi8 empezó por Windows); el área de contacto sola no bastó en esos dos.
- **Lag:** en Linux un solo bajón a 12,6 fps justo al abrir (18:50:04, mano_ms 39); el resto a
  30 fps. Windows sigue a ~17 fps (cotejo-y3qiti en curso).
- **Falta:** el reposo de 10 minutos se cortó en los dos, así que los toques falsos no quedan
  medidos con esta versión. Repetir solo el reposo.

## Siguiente

1. Repetir el reposo de 10 min (con las dos apps abiertas) para confirmar 0 falsos.
2. Llevar el imán a Linux (AT-SPI + hyprctl) si los fallos del centro se repiten.
3. Ver el lag de Windows con y3qiti.
