package main

// EstadoAura es lo que el halo pinta en cada cuadro. La posición llega en
// coordenadas del panel (0..1); la página la escala a la pantalla.
type EstadoAura struct {
	Activa    bool    `json:"activa"`    // modo táctil con la vista de cámara cerrada
	Calibrado bool    `json:"calibrado"` // el panel ya tiene dos esquinas guardadas
	Dentro    bool    `json:"dentro"`    // la mano está dentro del panel calibrado
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Agarrando bool    `json:"agarrando"` // pellizco cerrado (lo expone cotejo-l9dyjg)
}
