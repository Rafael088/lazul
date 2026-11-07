# Corre una medida en el escritorio del usuario de Windows (la sesión interactiva, no la de ssh)
# y deja el JSON en resultados\. Uso: .\medir-windows.ps1 <modo> <segundos> <delegado> <nombre>
param([string]$modo, [int]$seg, [string]$del, [string]$nombre)
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
New-Item -ItemType Directory -Force "$dir\resultados" | Out-Null
$out = "$dir\resultados\$nombre.json"; $log = "$dir\resultados\$nombre.log"
Remove-Item $out -ErrorAction SilentlyContinue
$args_ = "-modo $modo -segundos $seg -calentar 5 -delegado $del -salida `"$out`" -registro `"$log`""
$accion = New-ScheduledTaskAction -Execute "$dir\camara.exe" -Argument $args_ -WorkingDirectory $dir
$quien = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive
Register-ScheduledTask -TaskName "LazulCamara" -Action $accion -Principal $quien -Force | Out-Null
Start-ScheduledTask -TaskName "LazulCamara"
for ($i = 0; $i -lt ($seg + 120) -and -not (Test-Path $out); $i++) { Start-Sleep 1 }
Unregister-ScheduledTask -TaskName "LazulCamara" -Confirm:$false
if (-not (Test-Path $out)) { "sin resultado"; Get-Content $log -ErrorAction SilentlyContinue | Select -Last 5; exit 1 }
$d = Get-Content $out -Raw | ConvertFrom-Json; $w = $d.web
[ordered]@{ "cpu_%nucleo" = [math]::Round($d.cpu_pct_nucleo, 1); "cpu_%total" = [math]::Round($d.cpu_pct_total, 1); error = $w.error;
  delegado = $w.delegado_usado; fps = $w.fps; fps_camara = $w.fps_camara; con_mano = $w.cuadros_con_mano;
  inferencia = $w.inferencia_ms; cuadro_a_puntos = $w.cuadro_a_puntos_ms; captura_a_puntos = $w.captura_a_puntos_ms; camara = $w.camara } | ConvertTo-Json -Compress -Depth 4
