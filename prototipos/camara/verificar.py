#!/usr/bin/env python3
"""Revisa las medidas guardadas en resultados/ (cotejo-cphace) y dice si pasan.

Criterio de la prueba: con la mano en el cuadro y delegado GPU, al menos 15 fps,
cuadro→puntos p95 por debajo de 50 ms y la mano encontrada en más del 95 % de los cuadros.
Uso: python3 verificar.py   (sale con 1 si alguna medida obligatoria falta o no pasa)
"""
import json, pathlib, sys

R = pathlib.Path(__file__).resolve().parent / "resultados"
OBLIGATORIAS = {  # nombre → qué demuestra
    "win-camaramano-gpu": "Windows, WebView2, cámara directa (getUserMedia) + mano",
    "wl-procmano-gpu": "Hyprland, WebKitGTK, cámara por proceso aparte (plan B) + mano",
}

def fila(nombre):
    d = json.loads((R / f"{nombre}.json").read_text())
    w = d["web"]
    lat = w.get("captura_a_puntos_ms") or w.get("cuadro_a_puntos_ms") or {}
    return {"medida": nombre, "error": w.get("error"), "delegado": w.get("delegado_usado"), "fps": w.get("fps"),
            "fps_camara": w.get("fps_camara"), "mano_%": round(100 * w["cuadros_con_mano"] / w["resultados"], 1) if w.get("resultados") else 0,
            "inferencia_p50": (w.get("inferencia_ms") or {}).get("p50"), "lat_p50": lat.get("p50"), "lat_p95": lat.get("p95"),
            "cpu_%nucleo": round(d["cpu_pct_nucleo"], 1), "cpu_%total": round(d["cpu_pct_total"], 1)}

ok = True
for p in sorted(R.glob("*.json")):
    f = fila(p.stem)
    print(json.dumps(f, ensure_ascii=False))
for n, que in OBLIGATORIAS.items():
    if not (R / f"{n}.json").exists():
        print(f"FALTA {n}: {que}"); ok = False; continue
    f = fila(n)
    pasa = not f["error"] and f["delegado"] == "GPU" and f["fps"] >= 15 and f["lat_p95"] < 50 and f["mano_%"] > 95
    print(("PASA " if pasa else "NO PASA ") + f"{n}: {que}")
    ok &= pasa
sys.exit(0 if ok else 1)
