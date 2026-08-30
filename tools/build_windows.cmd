@echo off
setlocal

set "GO_EXE=%LOCALAPPDATA%\Programs\go\bin\go.exe"
set "FYNE_EXE=%USERPROFILE%\go\bin\fyne.exe"

for /d %%D in ("%LOCALAPPDATA%\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.MCF.UCRT_*") do set "GCC_BIN=%%~fD\mingw64\bin"

if not exist "%GO_EXE%" (
    echo Go was not found at "%GO_EXE%".
    exit /b 1
)

if not exist "%GCC_BIN%\gcc.exe" (
    echo WinLibs GCC was not found.
    exit /b 1
)

set "PATH=%GCC_BIN%;%LOCALAPPDATA%\Programs\go\bin;%PATH%"
set "CC=%GCC_BIN%\gcc.exe"
set "CGO_ENABLED=1"

if /i "%~1"=="test" (
    "%GO_EXE%" test ./...
    exit /b %ERRORLEVEL%
)

if not exist "%FYNE_EXE%" (
    "%GO_EXE%" install fyne.io/tools/cmd/fyne@latest || exit /b 1
)

"%FYNE_EXE%" package -os windows --release --tags migrated_fynedo