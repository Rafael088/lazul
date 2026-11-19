package main

import (
	"encoding/json"
	"testing"
)

func TestVistaBancoConservaEstadoDeLaMano(t *testing.T) {
	var c CuadroBanco
	if vistaDe(c).Indicador["activo"] != false {
		t.Fatal("sin mano debe quedar inactivo")
	}
	c.Manos = append(c.Manos, struct {
		Derecha bool         `json:"derecha"`
		P       [][3]float64 `json:"p"`
	}{Derecha: true})
	if vistaDe(c).Indicador["activo"] != true {
		t.Fatal("con mano debe quedar activo")
	}
}

func TestVariantesDelBanco(t *testing.T) {
	for _, nombre := range []string{"base", "envio", "respuesta", "indicador", "aura", "copia", "antes", "despues"} {
		if !variantes[nombre] {
			t.Errorf("falta variante %q", nombre)
		}
	}
}

func cuadroParaBanco() CuadroBanco {
	var c CuadroBanco
	var mano struct {
		Derecha bool         `json:"derecha"`
		P       [][3]float64 `json:"p"`
	}
	mano.Derecha = true
	for i := 0; i < 21; i++ {
		mano.P = append(mano.P, [3]float64{.2 + float64(i)/100, .4, -.01})
	}
	c.Manos = append(c.Manos, mano, mano)
	c.Cuerpo = &struct {
		P [][4]float64 `json:"p"`
	}{}
	for i := 0; i < 33; i++ {
		c.Cuerpo.P = append(c.Cuerpo.P, [4]float64{.3 + float64(i)/100, .4, 0, 1})
	}
	return c
}

func BenchmarkJSONCuadroHaciaGo(b *testing.B) {
	c := cuadroParaBanco()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONVistaHaciaWebview(b *testing.B) {
	v := vistaDe(cuadroParaBanco())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(v); err != nil {
			b.Fatal(err)
		}
	}
}
