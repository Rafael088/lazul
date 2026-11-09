//go:build windows

package windows

import (
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Rafael088/lazul/platform/internal/documento"
	"github.com/Rafael088/lazul/platform/internal/navegador"
	"golang.org/x/sys/windows"
)

// Niveles 1 y 2 del gesto 4 (dec-r2ii) en Windows:
//
//   - URL: si la ventana es de un navegador, el texto de su barra de
//     direcciones por UI Automation (la API de accesibilidad de Windows, por
//     COM, sin dependencias). No se lee si la barra tiene el foco: lo que hay
//     es lo que la persona está escribiendo.
//   - Documento: una ruta del título o de la línea de órdenes del proceso
//     cuyo nombre sale en el título (platform/internal/documento). Así se
//     abren los documentos con doble clic; las apps que abren el documento
//     después (desde su menú) no lo dejan ver y caen al nivel de la app.
//
// Cualquier fallo deja el nivel vacío y el gesto cae al siguiente.

// navegadores: ejecutables cuya barra de direcciones se lee.
var navegadores = map[string]bool{
	"msedge.exe": true, "chrome.exe": true, "brave.exe": true, "firefox.exe": true,
	"vivaldi.exe": true, "opera.exe": true, "chromium.exe": true,
}

// maxUIA es lo más que se espera a UI Automation; si el navegador no
// contesta a tiempo, se sigue sin URL.
const maxUIA = 1500 * time.Millisecond

func niveles(ventana uintptr, proceso windows.Handle, ejecutable, titulo string) (url, archivo string) {
	if navegadores[strings.ToLower(ejecutable)] {
		if u := navegador.DeBarra(barraDeDirecciones(ventana, strings.EqualFold(ejecutable, "firefox.exe"))); u != "" {
			// Un PDF abierto en el navegador es un documento, no una página.
			if r := navegador.RutaDeURL(u); r != "" {
				if documento.Es(r) && esArchivo(r) {
					return "", r
				}
				return "", ""
			}
			return u, ""
		}
	}
	return "", documentoAbierto(lineaDeOrdenes(proceso), titulo)
}

func esArchivo(ruta string) bool {
	attrs, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(ruta))
	return err == nil && attrs&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_DEVICE) == 0
}

// documentoAbierto busca el documento entre las rutas del título y los
// argumentos del proceso.
func documentoAbierto(linea, titulo string) string {
	candidatas := documento.RutasEnTitulo(titulo, esArchivo)
	if args, err := windows.DecomposeCommandLine(linea); err == nil && len(args) > 1 {
		for _, a := range args[1:] {
			if r := navegador.RutaDeURL(a); r != "" {
				a = r
			}
			if filepath.IsAbs(a) && documento.Es(a) && esArchivo(a) {
				candidatas = append(candidatas, filepath.Clean(a))
			}
		}
	}
	return documento.Elegir(titulo, candidatas)
}

// lineaDeOrdenes del proceso (Windows 8.1 en adelante), o "".
func lineaDeOrdenes(proceso windows.Handle) string {
	buf := make([]byte, 2048)
	for intento := 0; intento < 3; intento++ {
		var n uint32
		err := windows.NtQueryInformationProcess(proceso, windows.ProcessCommandLineInformation,
			unsafe.Pointer(&buf[0]), uint32(len(buf)), &n)
		if err == windows.STATUS_INFO_LENGTH_MISMATCH && n > uint32(len(buf)) && n < 1<<20 {
			buf = make([]byte, n)
			continue
		}
		if err != nil {
			return ""
		}
		return (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0])).String()
	}
	return ""
}

// --- UI Automation por COM ---

var (
	ole32Niveles        = windows.NewLazySystemDLL("ole32.dll")
	oleaut32Niveles     = windows.NewLazySystemDLL("oleaut32.dll")
	coCreateInstanceUIA = ole32Niveles.NewProc("CoCreateInstance")
	variantClearUIA     = oleaut32Niveles.NewProc("VariantClear")
	sysAllocStringUIA   = oleaut32Niveles.NewProc("SysAllocString")
	sysFreeStringUIA    = oleaut32Niveles.NewProc("SysFreeString")

	clsidCUIAutomation = windows.GUID{Data1: 0xff48dba4, Data2: 0x60ef, Data3: 0x4201,
		Data4: [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e}}
	iidIUIAutomation = windows.GUID{Data1: 0x30cbe57d, Data2: 0xd9d0, Data3: 0x452a,
		Data4: [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee}}
)

const (
	uiaControlType     = 30003
	uiaHasKeyboardFoco = 30008
	uiaAutomationID    = 30011
	uiaValorValue      = 30045
	uiaEdit            = 50004
	treeScopeDescend   = 4
	clsctxInproc       = 1
	vtI4, vtBSTR       = 3, 8
	vtBool             = 11

	// Posiciones en las tablas de métodos (UIAutomationClient.h).
	iunkRelease               = 2
	uiaElementFromHandle      = 6
	uiaCreatePropertyCond     = 23
	elemFindFirst             = 5
	elemGetCurrentPropertyVal = 10
)

// variante es un VARIANT de 64 bits (24 bytes); datos es la unión.
type variante struct {
	vt    uint16
	_     [3]uint16
	datos [16]byte
}

func (v *variante) bstr() **uint16 { return (**uint16)(unsafe.Pointer(&v.datos[0])) }

// objCOM es una interfaz COM: lo primero es su tabla de métodos.
type objCOM struct{ tabla *[64]uintptr }

func (o *objCOM) llamar(metodo int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.tabla[metodo], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *objCOM) soltar() {
	if o != nil {
		o.llamar(iunkRelease)
	}
}

// barraDeDirecciones devuelve el texto de la barra de direcciones de la
// ventana, o "" si no se pudo leer a tiempo.
func barraDeDirecciones(ventana uintptr, firefox bool) string {
	listo := make(chan string, 1)
	go func() { listo <- leerBarraUIA(ventana, firefox) }()
	select {
	case v := <-listo:
		return v
	case <-time.After(maxUIA):
		return "" // la gorutina termina sola cuando UIA conteste
	}
}

func leerBarraUIA(ventana uintptr, firefox bool) string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	switch err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err {
	case nil, syscall.Errno(1): // S_FALSE: ya estaba iniciado en este hilo
		defer windows.CoUninitialize()
	default:
		if err != syscall.Errno(windows.RPC_E_CHANGED_MODE) {
			return ""
		}
	}
	var uia *objCOM
	if r, _, _ := coCreateInstanceUIA.Call(uintptr(unsafe.Pointer(&clsidCUIAutomation)), 0, clsctxInproc,
		uintptr(unsafe.Pointer(&iidIUIAutomation)), uintptr(unsafe.Pointer(&uia))); r != 0 || uia == nil {
		return ""
	}
	defer uia.soltar()
	var raiz *objCOM
	if r := uia.llamar(uiaElementFromHandle, ventana, uintptr(unsafe.Pointer(&raiz))); r != 0 || raiz == nil {
		return ""
	}
	defer raiz.soltar()

	var barra *objCOM
	if firefox { // Firefox le pone nombre propio a la barra
		barra = buscar(uia, raiz, uiaAutomationID, varianteTexto("urlbar-input"))
	}
	if barra == nil { // Chromium: la barra es el primer cuadro de texto de la ventana
		v := variante{vt: vtI4}
		*(*int32)(unsafe.Pointer(&v.datos[0])) = uiaEdit
		barra = buscar(uia, raiz, uiaControlType, &v)
	}
	if barra == nil {
		return ""
	}
	defer barra.soltar()

	var foco variante
	if barra.llamar(elemGetCurrentPropertyVal, uiaHasKeyboardFoco, uintptr(unsafe.Pointer(&foco))) == 0 &&
		foco.vt == vtBool && *(*int16)(unsafe.Pointer(&foco.datos[0])) != 0 {
		return "" // la persona está escribiendo en la barra
	}
	var valor variante
	if barra.llamar(elemGetCurrentPropertyVal, uiaValorValue, uintptr(unsafe.Pointer(&valor))) != 0 {
		return ""
	}
	defer variantClearUIA.Call(uintptr(unsafe.Pointer(&valor)))
	if valor.vt != vtBSTR || *valor.bstr() == nil {
		return ""
	}
	return windows.UTF16PtrToString(*valor.bstr())
}

// varianteTexto arma un VARIANT con un BSTR; buscar lo libera.
func varianteTexto(s string) *variante {
	p, _, _ := sysAllocStringUIA.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(s))))
	if p == 0 {
		return nil
	}
	v := &variante{vt: vtBSTR}
	*(*uintptr)(unsafe.Pointer(&v.datos[0])) = p
	return v
}

// buscar el primer descendiente con esa propiedad igual a valor.
func buscar(uia, raiz *objCOM, propiedad uintptr, valor *variante) *objCOM {
	if valor == nil {
		return nil
	}
	if valor.vt == vtBSTR {
		defer sysFreeStringUIA.Call(*(*uintptr)(unsafe.Pointer(&valor.datos[0])))
	}
	var cond *objCOM
	// El VARIANT (24 bytes) va por valor: en x64 eso es un puntero a una copia.
	if r := uia.llamar(uiaCreatePropertyCond, propiedad, uintptr(unsafe.Pointer(valor)), uintptr(unsafe.Pointer(&cond))); r != 0 || cond == nil {
		return nil
	}
	defer cond.soltar()
	var hallado *objCOM
	if r := raiz.llamar(elemFindFirst, treeScopeDescend, uintptr(unsafe.Pointer(cond)), uintptr(unsafe.Pointer(&hallado))); r != 0 {
		return nil
	}
	return hallado
}
