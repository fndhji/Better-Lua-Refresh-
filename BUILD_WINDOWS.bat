@echo off
setlocal
cd /d "%~dp0"
where go >nul 2>&1
if errorlevel 1 (
  echo Installe Go 1.23 ou plus recent puis relance ce fichier.
  pause
  exit /b 1
)
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags="-H windowsgui -s -w" -o ExtremeLuaRefresh_Installer.exe .
if errorlevel 1 (
  pause
  exit /b 1
)
echo Compilation terminee : ExtremeLuaRefresh_Installer.exe
pause
