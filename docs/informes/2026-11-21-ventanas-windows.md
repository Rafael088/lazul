# Ventanas de Lazul que tapaban la pantalla en Windows

## Diagnóstico

El aura no era la capa que bloqueaba la pantalla. Desde su creación usa
`WS_EX_TRANSPARENT | WS_EX_NOACTIVATE`, responde `HTTRANSPARENT` y se muestra
con `SW_SHOWNOACTIVATE`. Su prueba viva ya comprobaba un toque debajo del halo.

La ventana principal de Wails sí podía bloquear. El indicador se pinta dentro
de una ventana translúcida `AlwaysOnTop`, pero esa ventana no tenía región de
entrada ni `WS_EX_NOACTIVATE`. Si al cerrar la vista de cámara WebView2 o Wails
conservaba durante más tiempo el tamaño de 920 × 660, solo se veía la píldora
en la esquina y el resto quedaba transparente, pero Windows seguía enviando a
Lazul los clics de toda esa superficie. La misma ventana podía convertirse en
la ventana con foco.

## Corrección

- Windows identifica el `HWND` principal por PID y por la clase exclusiva
  `LazulIndicador`; el título no importa.
- La app nace oculta. Antes de mostrar el indicador, se añade
  `WS_EX_NOACTIVATE`, se quita de Alt+Tab y se recorta la región nativa a los
  224 × 52 px de la píldora redonda. El resto de una ventana accidentalmente
  grande deja de existir para el hit-test de Windows.
- El indicador se muestra con `SW_SHOWNOACTIVATE`. El botón de cámara dentro de
  la píldora sigue recibiendo el ratón, pero abrir Lazul no cambia el foco.
- **Acomodar la cámara**, que se abre de forma explícita, quita la región y
  recupera el estilo interactivo original. Al cerrarla se vuelven a aplicar
  las protecciones y se devuelve el foco a la ventana anterior.
- La prueba viva del aura ahora revisa también sus estilos, el foco y nueve
  puntos de su rectángulo.

## Cómo comprobarlo en Windows

La prueba `TestEntradaVentanaViva` crea encima del escritorio una ventana
transparente del tamaño de la pantalla, le aplica exactamente la protección del
indicador y usa `WindowFromPoint` sobre el centro, los cuatro bordes y las cuatro esquinas.
Ninguno puede pertenecer a la ventana transparente; el centro de la píldora sí
debe seguir siendo interactivo. También comprueba que mostrarla no cambia el
foco y que abrir/cerrar la vista de cámara conmuta la región.

```powershell
$env:LAZUL_VENTANA_VIVO='1'
go -C desktop/app test -count=1 -v . -run 'Test(EntradaVentana|Aura)Vivo'
```

Sin escritorio se pueden ejecutar todas las pruebas normales y compilar el
binario de Windows; las dos pruebas vivas se omiten de forma explícita.
