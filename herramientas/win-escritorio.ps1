# Corre un ejecutable en la sesión del escritorio del usuario de Windows (no en
# la de ssh, que no tiene portapapeles, pantalla ni ventanas) y devuelve su
# salida y su código. Lo usa `./dev win contrato`.
# Uso: .\win-escritorio.ps1 -exe C:\ruta\x.exe -argumentos '-test.v' -entorno 'A=1;B=2'
param([string]$exe, [string]$argumentos = '', [string]$entorno = '', [int]$segundos = 300)
$ErrorActionPreference = 'Stop'
$dir = Split-Path -Parent $exe
$salida = Join-Path $dir 'escritorio-salida.txt'
$codigo = Join-Path $dir 'escritorio-codigo.txt'
$envoltorio = Join-Path $dir 'escritorio.cmd'
Remove-Item $salida, $codigo -ErrorAction SilentlyContinue

$lineas = @('@echo off', "cd /d `"$dir`"")
foreach ($par in ($entorno -split ';' | Where-Object { $_ })) { $lineas += "set $par" }
$lineas += "`"$exe`" $argumentos > `"$salida`" 2>&1"
$lineas += "echo %ERRORLEVEL% > `"$codigo`""
Set-Content -Path $envoltorio -Value $lineas -Encoding Ascii

$nombre = 'LazulEscritorio'
$accion = New-ScheduledTaskAction -Execute 'cmd.exe' -Argument "/c `"$envoltorio`"" -WorkingDirectory $dir
$quien = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive
Register-ScheduledTask -TaskName $nombre -Action $accion -Principal $quien -Force | Out-Null
Start-ScheduledTask -TaskName $nombre
for ($i = 0; $i -lt $segundos -and -not (Test-Path $codigo); $i++) { Start-Sleep 1 }
Unregister-ScheduledTask -TaskName $nombre -Confirm:$false

[Console]::OutputEncoding = [Text.Encoding]::UTF8
if (Test-Path $salida) { Get-Content $salida -Encoding UTF8 }
if (-not (Test-Path $codigo)) { "sin terminar en $segundos s (hay una sesion de escritorio abierta?)"; exit 1 }
exit [int](Get-Content $codigo -Raw).Trim()
