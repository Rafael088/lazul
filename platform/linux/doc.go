// Package linux implementa platform en Linux (Hyprland primero; KDE y GNOME lo mejor posible). Un archivo por capacidad:
// captura.go (Screenshot), portapapeles.go (Clipboard), app_activa.go
// (ActiveApp), abrir_app.go (LaunchApp) y acciones_tactiles.go
// (TactileActions, solo Hyprland) y objetivos.go con atspi.go (TargetFinder:
// botones y enlaces por AT-SPI, ubicados con la ventana de Hyprland). Está terminado cuando pasa contrato_test.go:
// ./dev contrato.
package linux
