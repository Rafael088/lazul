#ifndef LAZUL_AURA_H
#define LAZUL_AURA_H

/* Aura es una superficie layer-shell transparente, encima de las ventanas
   normales. No recibe teclado ni clics: su región de entrada es vacía. */
typedef struct Aura Aura;

Aura *lazul_aura_crear(const char *html);
/* Lado en píxeles lógicos de la superficie: cabe el halo encendido. */
#define LAZUL_AURA_LADO 180

/* x, y: posición de la mano en 0..1 sobre el monitor. */
void lazul_aura_actualizar(Aura *aura, const char *json, double x, double y);
/* Pone la superficie en el monitor cuyo origen en el escritorio es (x, y).
   Si GTK no tiene ninguno así, se queda donde estaba. */
void lazul_aura_monitor(Aura *aura, int x, int y);
void lazul_aura_cerrar(Aura *aura);

#endif
