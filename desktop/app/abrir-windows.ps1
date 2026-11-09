# Abre lazul.exe en el escritorio del usuario de Windows (no en la sesión de
# ssh, que no tiene pantalla ni cámara) y vuelve sin esperar a que se cierre.
# Lo usa `./dev app win`. Uso: .\abrir-windows.ps1 -argumentos '-nombre "PC Windows"'
param([string]$argumentos = '')
$ErrorActionPreference = 'Stop'
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $dir 'lazul.exe'
$nombre = 'LazulApp'
# -Argument no acepta una cadena vacía: sin argumentos, se omite.
if ($argumentos) {
    $accion = New-ScheduledTaskAction -Execute $exe -Argument $argumentos -WorkingDirectory $dir
} else {
    $accion = New-ScheduledTaskAction -Execute $exe -WorkingDirectory $dir
}
$quien = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive
# Sin límite de tiempo: si no, el programador la cierra a las 72 h.
$ajustes = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
Register-ScheduledTask -TaskName $nombre -Action $accion -Principal $quien -Settings $ajustes -Force | Out-Null
Start-ScheduledTask -TaskName $nombre
for ($i = 0; $i -lt 20 -and -not (Get-Process lazul -ErrorAction SilentlyContinue); $i++) { Start-Sleep -Milliseconds 500 }
$p = Get-Process lazul -ErrorAction SilentlyContinue
# Quitar la tarea no cierra la app ya abierta: no queda nada programado.
Unregister-ScheduledTask -TaskName $nombre -Confirm:$false
if (-not $p) { 'lazul.exe no arrancó (¿hay una sesión de escritorio abierta?)'; exit 1 }
"lazul.exe abierta (proceso $($p[0].Id))"
exit 0
