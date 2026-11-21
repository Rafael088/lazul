//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/Rafael088/lazul/platform"
	"golang.org/x/sys/windows"
)

// objetivosWindows implementa platform.TargetFinder con UI Automation: los
// botones, enlaces, casillas, pestañas y elementos de menú de la ventana con
// el foco que no estén fuera de la pantalla ni deshabilitados. Se piden de
// una vez con su rectángulo en caché (FindAllBuildCache), así que es una
// sola ida y vuelta al proceso de la app aunque la página tenga cientos de
// enlaces.
type objetivosWindows struct{ destino *destinoPantalla }

var _ platform.TargetFinder = objetivosWindows{}

const (
	// maxObjetivosUIA es lo más que se espera una lectura; después se
	// devuelve error y la gorutina termina sola cuando UIA conteste.
	maxObjetivosUIA = 800 * time.Millisecond
	// maxElementosUIA: más allá de esto se ignoran (una página enorme).
	maxElementosUIA = 800
)

// Tipos de control que cuentan como objetivo (UIA_*ControlTypeId).
var tiposObjetivo = []int32{
	50000, // Button
	50002, // CheckBox
	50003, // ComboBox
	50005, // Hyperlink
	50011, // MenuItem
	50013, // RadioButton
	50019, // TabItem
	50031, // SplitButton
}

const (
	uiaBoundingRect = 30001
	uiaIsEnabled    = 30010
	uiaIsOffscreen  = 30022
	vtArrayR8       = 0x2000 | 5 // VT_ARRAY | VT_R8

	// Posiciones en las tablas de métodos (UIAutomationClient.h).
	uiaCreateCacheRequest   = 20
	uiaCreateAndCondition   = 25
	uiaCreateOrCondNative   = 30
	cacheAddProperty        = 3
	elemFindAllBuildCache   = 8
	elemGetCachedPropertyVa = 12
	arrGetLength            = 3
	arrGetElement           = 4
)

var (
	safeArrayAccessData   = oleaut32Niveles.NewProc("SafeArrayAccessData")
	safeArrayUnaccessData = oleaut32Niveles.NewProc("SafeArrayUnaccessData")
	safeArrayGetUBound    = oleaut32Niveles.NewProc("SafeArrayGetUBound")
)

// FocusedWindow es GetForegroundWindow: no toca UIA.
func (objetivosWindows) FocusedWindow(ctx context.Context) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	v, _, _ := getForegroundWindowCaptura.Call()
	if v == 0 {
		return 0, platform.ErrSinVentana
	}
	return uint64(v), nil
}

func (o objetivosWindows) Targets(ctx context.Context) (platform.TargetSet, error) {
	if err := ctx.Err(); err != nil {
		return platform.TargetSet{}, err
	}
	ventana, _, _ := getForegroundWindowCaptura.Call()
	if ventana == 0 {
		return platform.TargetSet{}, platform.ErrSinVentana
	}
	type salida struct {
		s   platform.TargetSet
		err error
	}
	listo := make(chan salida, 1)
	go func() {
		var s platform.TargetSet
		err := conDPIFisico(func() error {
			rects, err := leerObjetivosUIA(ventana)
			if err != nil {
				return err
			}
			z := zonaDe(o.destino, pantallaPrincipal)
			s = normalizarObjetivos(rects, z, dpiEnPixel(punto{z.x + z.ancho/2, z.y + z.alto/2}))
			return nil
		})
		s.Window = uint64(ventana)
		listo <- salida{s, err}
	}()
	espera := maxObjetivosUIA
	if d, ok := ctx.Deadline(); ok && time.Until(d) < espera {
		espera = time.Until(d)
	}
	select {
	case r := <-listo:
		return r.s, r.err
	case <-time.After(espera):
		return platform.TargetSet{}, errors.New("UI Automation no contestó a tiempo")
	}
}

// rectFisico es un rectángulo de UIA en píxeles físicos de la pantalla
// virtual: izquierda, arriba, ancho y alto.
type rectFisico struct{ x, y, ancho, alto float64 }

// normalizarObjetivos lleva los rectángulos a (0,0)-(1,1) de la zona que
// recibe el táctil, con la misma escala que zonaTactil.pixel, y da el tamaño
// lógico de la zona con el DPI del monitor (0 es 100 %). Deja fuera los que
// no tocan la zona.
func normalizarObjetivos(rs []rectFisico, z zonaTactil, dpi uint32) platform.TargetSet {
	if dpi == 0 {
		dpi = dpiBase
	}
	s := platform.TargetSet{
		LogicalWidth:  float64(z.ancho) * dpiBase / float64(dpi),
		LogicalHeight: float64(z.alto) * dpiBase / float64(dpi),
	}
	if z.ancho <= 1 || z.alto <= 1 {
		return s
	}
	ax, ay := float64(z.ancho-1), float64(z.alto-1)
	for _, r := range rs {
		if r.ancho <= 0 || r.alto <= 0 {
			continue
		}
		x, y := (r.x-float64(z.x))/ax, (r.y-float64(z.y))/ay
		w, h := r.ancho/ax, r.alto/ay
		if x+w < 0 || y+h < 0 || x > 1 || y > 1 {
			continue
		}
		s.Targets = append(s.Targets, platform.Target{X: x, Y: y, Width: w, Height: h})
	}
	return s
}

// leerObjetivosUIA corre en un hilo fijo, ya con el DPI físico.
func leerObjetivosUIA(ventana uintptr) ([]rectFisico, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	switch err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err {
	case nil, syscall.Errno(1): // S_FALSE: ya estaba iniciado en este hilo
		defer windows.CoUninitialize()
	default:
		if err != syscall.Errno(windows.RPC_E_CHANGED_MODE) {
			return nil, fmt.Errorf("CoInitializeEx: %w", err)
		}
	}
	var uia *objCOM
	if r, _, _ := coCreateInstanceUIA.Call(uintptr(unsafe.Pointer(&clsidCUIAutomation)), 0, clsctxInproc,
		uintptr(unsafe.Pointer(&iidIUIAutomation)), uintptr(unsafe.Pointer(&uia))); r != 0 || uia == nil {
		return nil, fmt.Errorf("CoCreateInstance(CUIAutomation): 0x%x", r)
	}
	defer uia.soltar()
	var raiz *objCOM
	if r := uia.llamar(uiaElementFromHandle, ventana, uintptr(unsafe.Pointer(&raiz))); r != 0 || raiz == nil {
		return nil, fmt.Errorf("ElementFromHandle: 0x%x", r)
	}
	defer raiz.soltar()

	cond, err := condicionObjetivos(uia)
	if err != nil {
		return nil, err
	}
	defer cond.soltar()
	var cache *objCOM
	if r := uia.llamar(uiaCreateCacheRequest, uintptr(unsafe.Pointer(&cache))); r != 0 || cache == nil {
		return nil, fmt.Errorf("CreateCacheRequest: 0x%x", r)
	}
	defer cache.soltar()
	if r := cache.llamar(cacheAddProperty, uiaBoundingRect); r != 0 {
		return nil, fmt.Errorf("AddProperty: 0x%x", r)
	}
	var arr *objCOM
	if r := raiz.llamar(elemFindAllBuildCache, treeScopeDescend, uintptr(unsafe.Pointer(cond)),
		uintptr(unsafe.Pointer(cache)), uintptr(unsafe.Pointer(&arr))); r != 0 {
		return nil, fmt.Errorf("FindAllBuildCache: 0x%x", r)
	}
	if arr == nil {
		return nil, nil // ningún objetivo
	}
	defer arr.soltar()
	var n int32
	if r := arr.llamar(arrGetLength, uintptr(unsafe.Pointer(&n))); r != 0 {
		return nil, fmt.Errorf("get_Length: 0x%x", r)
	}
	n = min(n, maxElementosUIA)
	rs := make([]rectFisico, 0, n)
	for i := range n {
		var e *objCOM
		if r := arr.llamar(arrGetElement, uintptr(i), uintptr(unsafe.Pointer(&e))); r != 0 || e == nil {
			continue
		}
		if r, ok := rectEnCache(e); ok {
			rs = append(rs, r)
		}
		e.soltar()
	}
	return rs, nil
}

// condicionObjetivos: (tipo es alguno de tiposObjetivo) y habilitado y no
// fuera de la pantalla.
func condicionObjetivos(uia *objCOM) (*objCOM, error) {
	propiedad := func(id uintptr, v *variante) (*objCOM, error) {
		var c *objCOM
		if r := uia.llamar(uiaCreatePropertyCond, id, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&c))); r != 0 || c == nil {
			return nil, fmt.Errorf("CreatePropertyCondition(%d): 0x%x", id, r)
		}
		return c, nil
	}
	tipos := make([]*objCOM, 0, len(tiposObjetivo))
	defer func() {
		for _, c := range tipos {
			c.soltar()
		}
	}()
	for _, t := range tiposObjetivo {
		v := variante{vt: vtI4}
		*(*int32)(unsafe.Pointer(&v.datos[0])) = t
		c, err := propiedad(uiaControlType, &v)
		if err != nil {
			return nil, err
		}
		tipos = append(tipos, c)
	}
	var alguno *objCOM
	if r := uia.llamar(uiaCreateOrCondNative, uintptr(unsafe.Pointer(&tipos[0])), uintptr(len(tipos)),
		uintptr(unsafe.Pointer(&alguno))); r != 0 || alguno == nil {
		return nil, fmt.Errorf("CreateOrConditionFromNativeArray: 0x%x", r)
	}
	defer alguno.soltar()

	si := variante{vt: vtBool}
	*(*int16)(unsafe.Pointer(&si.datos[0])) = -1 // VARIANT_TRUE
	habilitado, err := propiedad(uiaIsEnabled, &si)
	if err != nil {
		return nil, err
	}
	defer habilitado.soltar()
	no := variante{vt: vtBool}
	visible, err := propiedad(uiaIsOffscreen, &no)
	if err != nil {
		return nil, err
	}
	defer visible.soltar()

	var ambas, todo *objCOM
	if r := uia.llamar(uiaCreateAndCondition, uintptr(unsafe.Pointer(habilitado)), uintptr(unsafe.Pointer(visible)), uintptr(unsafe.Pointer(&ambas))); r != 0 || ambas == nil {
		return nil, fmt.Errorf("CreateAndCondition: 0x%x", r)
	}
	defer ambas.soltar()
	if r := uia.llamar(uiaCreateAndCondition, uintptr(unsafe.Pointer(alguno)), uintptr(unsafe.Pointer(ambas)), uintptr(unsafe.Pointer(&todo))); r != 0 || todo == nil {
		return nil, fmt.Errorf("CreateAndCondition: 0x%x", r)
	}
	return todo, nil
}

// rectEnCache lee BoundingRectangle de la caché: un SAFEARRAY de 4 double
// (izquierda, arriba, ancho, alto).
func rectEnCache(e *objCOM) (rectFisico, bool) {
	var v variante
	if r := e.llamar(elemGetCachedPropertyVa, uiaBoundingRect, uintptr(unsafe.Pointer(&v))); r != 0 {
		return rectFisico{}, false
	}
	defer variantClearUIA.Call(uintptr(unsafe.Pointer(&v)))
	if v.vt != vtArrayR8 {
		return rectFisico{}, false
	}
	sa := *(*uintptr)(unsafe.Pointer(&v.datos[0]))
	if sa == 0 {
		return rectFisico{}, false
	}
	var alto int32
	if r, _, _ := safeArrayGetUBound.Call(sa, 1, uintptr(unsafe.Pointer(&alto))); r != 0 || alto < 3 {
		return rectFisico{}, false
	}
	var datos *[4]float64
	if r, _, _ := safeArrayAccessData.Call(sa, uintptr(unsafe.Pointer(&datos))); r != 0 || datos == nil {
		return rectFisico{}, false
	}
	d := *datos
	safeArrayUnaccessData.Call(sa)
	return rectFisico{d[0], d[1], d[2], d[3]}, true
}
