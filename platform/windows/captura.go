//go:build windows

package windows

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/windows"
)

type captura struct{}

var (
	user32Captura = windows.NewLazySystemDLL("user32.dll")
	gdi32Captura  = windows.NewLazySystemDLL("gdi32.dll")

	getForegroundWindowCaptura = user32Captura.NewProc("GetForegroundWindow")
	monitorFromWindowCaptura   = user32Captura.NewProc("MonitorFromWindow")
	getMonitorInfoCaptura      = user32Captura.NewProc("GetMonitorInfoW")
	getDCCaptura               = user32Captura.NewProc("GetDC")
	releaseDCCaptura           = user32Captura.NewProc("ReleaseDC")
	createCompatibleDCCaptura  = gdi32Captura.NewProc("CreateCompatibleDC")
	deleteDCCaptura            = gdi32Captura.NewProc("DeleteDC")
	createCompatibleBitmap     = gdi32Captura.NewProc("CreateCompatibleBitmap")
	selectObjectCaptura        = gdi32Captura.NewProc("SelectObject")
	deleteObjectCaptura        = gdi32Captura.NewProc("DeleteObject")
	bitBltCaptura              = gdi32Captura.NewProc("BitBlt")
	getDIBitsCaptura           = gdi32Captura.NewProc("GetDIBits")
)

const (
	monitorDefaultToPrimary = 1
	srccopy                 = 0x00CC0020
	captureblt              = 0x40000000
	dibRGBColors            = 0
)

type rectCaptura struct {
	izquierda, arriba, derecha, abajo int32
}

type monitorInfoCaptura struct {
	tamano           uint32
	monitor, trabajo rectCaptura
	flags            uint32
}

type bitmapInfoHeaderCaptura struct {
	tamano             uint32
	ancho, alto        int32
	planos, bits       uint16
	compresion         uint32
	tamanoImagen       uint32
	ppmX, ppmY         int32
	coloresUsados      uint32
	coloresImportantes uint32
}

type bitmapInfoCaptura struct {
	cabecera bitmapInfoHeaderCaptura
	colores  [1]uint32
}

func (captura) Capture(ctx context.Context) (platform.Imagen, error) {
	if err := ctx.Err(); err != nil {
		return platform.Imagen{}, err
	}

	ventana, _, _ := getForegroundWindowCaptura.Call()
	monitor, _, _ := monitorFromWindowCaptura.Call(ventana, monitorDefaultToPrimary)
	if monitor == 0 {
		return platform.Imagen{}, fmt.Errorf("buscar monitor para la captura")
	}
	info := monitorInfoCaptura{tamano: uint32(unsafe.Sizeof(monitorInfoCaptura{}))}
	ok, _, err := getMonitorInfoCaptura.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return platform.Imagen{}, fmt.Errorf("consultar monitor: %w", err)
	}
	ancho := int(info.monitor.derecha - info.monitor.izquierda)
	alto := int(info.monitor.abajo - info.monitor.arriba)
	if ancho <= 0 || alto <= 0 {
		return platform.Imagen{}, fmt.Errorf("monitor con tamaño inválido %dx%d", ancho, alto)
	}

	pantalla, _, err := getDCCaptura.Call(0)
	if pantalla == 0 {
		return platform.Imagen{}, fmt.Errorf("obtener DC de pantalla: %w", err)
	}
	defer releaseDCCaptura.Call(0, pantalla)
	memoria, _, err := createCompatibleDCCaptura.Call(pantalla)
	if memoria == 0 {
		return platform.Imagen{}, fmt.Errorf("crear DC de captura: %w", err)
	}
	defer deleteDCCaptura.Call(memoria)
	mapa, _, err := createCompatibleBitmap.Call(pantalla, uintptr(ancho), uintptr(alto))
	if mapa == 0 {
		return platform.Imagen{}, fmt.Errorf("crear bitmap de captura: %w", err)
	}
	defer deleteObjectCaptura.Call(mapa)
	anterior, _, err := selectObjectCaptura.Call(memoria, mapa)
	if anterior == 0 || anterior == ^uintptr(0) {
		return platform.Imagen{}, fmt.Errorf("seleccionar bitmap de captura: %w", err)
	}
	seleccionado := true
	defer func() {
		if seleccionado {
			selectObjectCaptura.Call(memoria, anterior)
		}
	}()
	ok, _, err = bitBltCaptura.Call(
		memoria, 0, 0, uintptr(ancho), uintptr(alto), pantalla,
		uintptr(info.monitor.izquierda), uintptr(info.monitor.arriba), srccopy|captureblt,
	)
	if ok == 0 {
		return platform.Imagen{}, fmt.Errorf("copiar pantalla al bitmap: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return platform.Imagen{}, err
	}
	if restaurado, _, err := selectObjectCaptura.Call(memoria, anterior); restaurado == 0 || restaurado == ^uintptr(0) {
		return platform.Imagen{}, fmt.Errorf("liberar bitmap del DC de captura: %w", err)
	}
	seleccionado = false

	bgra := make([]byte, ancho*alto*4)
	bi := bitmapInfoCaptura{cabecera: bitmapInfoHeaderCaptura{
		tamano: uint32(unsafe.Sizeof(bitmapInfoHeaderCaptura{})), ancho: int32(ancho), alto: -int32(alto),
		planos: 1, bits: 32,
	}}
	lineas, _, err := getDIBitsCaptura.Call(
		pantalla, mapa, 0, uintptr(alto), uintptr(unsafe.Pointer(&bgra[0])),
		uintptr(unsafe.Pointer(&bi)), dibRGBColors,
	)
	if int(lineas) != alto {
		return platform.Imagen{}, fmt.Errorf("leer bitmap: se obtuvieron %d de %d líneas: %w", lineas, alto, err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, ancho, alto))
	for i := 0; i < len(bgra); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bgra[i+2], bgra[i+1], bgra[i], 0xff
	}
	var salida bytes.Buffer
	codificador := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := codificador.Encode(&salida, img); err != nil {
		return platform.Imagen{}, fmt.Errorf("codificar captura como PNG: %w", err)
	}
	return platform.Imagen{PNG: salida.Bytes(), Ancho: ancho, Alto: alto}, nil
}
