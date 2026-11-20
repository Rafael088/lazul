//go:build windows

// Prueba del táctil virtual en Windows (cotejo-ckwoq7): el mismo
// platform.TouchScreen que usa la app (InjectTouchInput), movido por un
// guion de órdenes. Coordenadas normalizadas 0..1 sobre la pantalla
// principal. Tiene que correr en la sesión del escritorio (no en la de ssh):
// lo lanza correr.sh con herramientas/win-escritorio.ps1.
//
//	tactil-win.exe guion archivo
//
// Órdenes (una por línea; # comenta):
//
//	tap x y | doble x y | largo x y [ms] | arrastrar x0 y0 x1 y1 [ms]
//	pellizco cx cy sep0 sep1 [ms] | apoyar x y | levantar | raton x y | esperar ms
//	abrir programa args... | correr programa args... | entorno K=V | edge args... | cerrar-edge (solo el de la prueba) | captura archivo.png | titulos (de Edge por CDP)
//	({dir} se cambia por la carpeta del guion)
//	cpu segundos (CPU de este proceso mientras sostiene un dedo moviéndose)
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Rafael088/lazul/platform"
	"github.com/Rafael088/lazul/platform/windows"
	win "golang.org/x/sys/windows"
)

var sis = windows.New()

const cuadro = 16 * time.Millisecond

func frame(ps ...platform.TouchPoint) error {
	return sis.TouchScreen.Frame(context.Background(), ps)
}

func dedo(slot int, x, y float64) platform.TouchPoint {
	return platform.TouchPoint{Slot: slot, X: x, Y: y, Down: true}
}

func tap(x, y float64, dur time.Duration) error {
	if err := frame(dedo(0, x, y)); err != nil {
		return err
	}
	for t := cuadro; t < dur; t += cuadro {
		time.Sleep(cuadro)
		if err := frame(dedo(0, x, y)); err != nil {
			return err
		}
	}
	time.Sleep(cuadro)
	return frame()
}

func trazo(a, b [][2]float64, dur time.Duration) error {
	n := max(1, int(dur/cuadro))
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(n)
		var ps []platform.TouchPoint
		for k := range a {
			ps = append(ps, dedo(k, a[k][0]+(b[k][0]-a[k][0])*f, a[k][1]+(b[k][1]-a[k][1])*f))
		}
		if err := frame(ps...); err != nil {
			return err
		}
		time.Sleep(cuadro)
	}
	return frame()
}

func num(a []string, i int, def float64) float64 {
	if i >= len(a) {
		return def
	}
	v, err := strconv.ParseFloat(a[i], 64)
	if err != nil {
		return def
	}
	return v
}

func ms(v float64) time.Duration { return time.Duration(v) * time.Millisecond }

func titulos() error {
	r, err := http.Get("http://127.0.0.1:9222/json")
	if err != nil {
		return err
	}
	defer r.Body.Close()
	var ps []struct{ Type, Title, URL string }
	if err := json.NewDecoder(r.Body).Decode(&ps); err != nil {
		return err
	}
	for _, p := range ps {
		if p.Type == "page" {
			fmt.Printf("titulo %s\n", p.Title)
		}
	}
	return nil
}

func cpuPropia() time.Duration {
	var c, s, k, u win.Filetime
	_ = win.GetProcessTimes(win.CurrentProcess(), &c, &s, &k, &u)
	return time.Duration(k.Nanoseconds() + u.Nanoseconds())
}

// cpu sostiene un dedo dando vueltas a 30 cuadros/s (lo que da la cámara)
// y mide cuánto CPU gasta la inyección.
func cpu(seg float64) error {
	ini, t0 := cpuPropia(), time.Now()
	n := 0
	for time.Since(t0) < ms(seg*1000) {
		a := float64(n) / 30
		if err := frame(dedo(0, .5+.1*math.Cos(a), .5+.1*math.Sin(a))); err != nil {
			return err
		}
		n++
		time.Sleep(33 * time.Millisecond)
	}
	_ = frame()
	pared := time.Since(t0)
	fmt.Printf("cpu %d cuadros en %.1f s: %.2f %% de un núcleo\n", n, pared.Seconds(), 100*float64(cpuPropia()-ini)/float64(pared))
	return nil
}

func orden(a []string) error {
	switch a[0] {
	case "tap":
		return tap(num(a, 1, .5), num(a, 2, .5), 60*time.Millisecond)
	case "doble":
		if err := tap(num(a, 1, .5), num(a, 2, .5), 60*time.Millisecond); err != nil {
			return err
		}
		time.Sleep(120 * time.Millisecond)
		return tap(num(a, 1, .5), num(a, 2, .5), 60*time.Millisecond)
	case "largo":
		return tap(num(a, 1, .5), num(a, 2, .5), ms(num(a, 3, 900)))
	case "arrastrar":
		return trazo([][2]float64{{num(a, 1, .5), num(a, 2, .7)}}, [][2]float64{{num(a, 3, .5), num(a, 4, .3)}}, ms(num(a, 5, 300)))
	case "pellizco":
		cx, cy, s0, s1 := num(a, 1, .5), num(a, 2, .5), num(a, 3, .05), num(a, 4, .3)
		return trazo([][2]float64{{cx - s0/2, cy}, {cx + s0/2, cy}}, [][2]float64{{cx - s1/2, cy}, {cx + s1/2, cy}}, ms(num(a, 5, 700)))
	case "apoyar": // un solo cuadro: el latido del táctil lo sostiene
		return frame(dedo(0, num(a, 1, .5), num(a, 2, .5)))
	case "levantar":
		return frame()
	case "raton":
		return sis.AbsolutePointer.MoveAbsolute(context.Background(), num(a, 1, .5), num(a, 2, .5))
	case "esperar":
		time.Sleep(ms(num(a, 1, 500)))
		return nil
	case "abrir":
		return exec.Command(a[1], a[2:]...).Start()
	case "correr": // espera a que termine y muestra su salida
		c := exec.Command(a[1], a[2:]...)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	case "entorno":
		k, v, _ := strings.Cut(a[1], "=")
		return os.Setenv(k, v)
	case "cerrar-edge":
		// Solo el Edge de esta prueba (su perfil propio), nunca el del dueño.
		return exec.Command("powershell", "-NoProfile", "-Command",
			`Get-CimInstance Win32_Process -Filter "Name='msedge.exe'" | Where-Object { $_.CommandLine -like '*lazul-tactil/edge-perfil*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`).Run()
	case "edge":
		// start busca msedge en App Paths, que exec no mira.
		return exec.Command("cmd", append([]string{"/c", "start", "", "msedge"}, a[1:]...)...).Run()
	case "captura":
		img, err := sis.Screenshot.Capture(context.Background())
		if err != nil {
			return err
		}
		fmt.Printf("captura %s %dx%d\n", a[1], img.Ancho, img.Alto)
		return os.WriteFile(a[1], img.PNG, 0o644)
	case "titulos":
		return titulos()
	case "cpu":
		return cpu(num(a, 1, 10))
	}
	return fmt.Errorf("orden desconocida %q", a[0])
}

func main() {
	if len(os.Args) != 3 || os.Args[1] != "guion" {
		fmt.Fprintln(os.Stderr, "uso: tactil-win guion archivo")
		os.Exit(2)
	}
	// La captura y el guion en píxeles físicos (este PC está al 150 %), como
	// la app, que declara DPI por monitor.
	win.NewLazySystemDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext").Call(^uintptr(3))
	if err := sis.TouchScreen.Prepare(context.Background()); err != nil {
		fmt.Println("Prepare:", err)
		os.Exit(1)
	}
	f, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()
	dir, _ := os.Getwd()
	sc := bufio.NewScanner(f)
	fallos := 0
	for sc.Scan() {
		l := strings.ReplaceAll(strings.TrimSpace(sc.Text()), "{dir}", filepath.ToSlash(dir))
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		a := strings.Fields(l)
		t0 := time.Now()
		if err := orden(a); err != nil {
			fmt.Printf("FALLA %s: %v\n", l, err)
			fallos++
			continue
		}
		fmt.Printf("ok    %s (%d ms)\n", l, time.Since(t0).Milliseconds())
	}
	if fallos > 0 {
		os.Exit(1)
	}
}
