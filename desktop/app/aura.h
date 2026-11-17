#ifndef LAZUL_AURA_H
#define LAZUL_AURA_H

/* Aura es una superficie layer-shell transparente, encima de las ventanas
   normales. No recibe teclado ni clics: su región de entrada es vacía. */
typedef struct Aura Aura;

Aura *lazul_aura_crear(const char *html);
void lazul_aura_actualizar(Aura *aura, const char *json);
void lazul_aura_cerrar(Aura *aura);

#endif
