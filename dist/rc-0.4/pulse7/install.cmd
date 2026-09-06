@echo off
setlocal
set B=%~dp0
set LOG=%B%logs\install.log
mkdir "%B%logs" 2>nul
echo ===== win7-agent install ===== > "%LOG%"

rem [1] package completeness (hard requirement: our own files)
if not exist "%B%win7-agent.exe" echo [1] FATAL win7-agent.exe missing>> "%LOG%" & if not exist "%B%win7-agent.exe" exit /b 1
if not exist "%B%runtime\git\cmd\git.exe" echo [1] FATAL runtime\git incomplete>> "%LOG%" & if not exist "%B%runtime\git\cmd\git.exe" exit /b 1
echo [1] package OK>> "%LOG%"

rem [2] optional Sandboxie silent install. PRODUCT RULE: any failure here is
rem     NON-BLOCKING and we NEVER ask for system patches - the agent simply
rem     runs in JobObject mode.
set START=C:\Program Files\Sandboxie\Start.exe
if exist "%START%" (
  echo [2] Sandboxie already installed>> "%LOG%"
) else (
  if exist "%B%sandbox\Sandboxie-Classic-x64-v5.73.2.exe" (
    echo [2] installing bundled Sandboxie Classic silently...>> "%LOG%"
    "%B%sandbox\Sandboxie-Classic-x64-v5.73.2.exe" /S
    ping -n 8 127.0.0.1 >nul
  ) else (
    echo [2] no bundled Sandboxie installer - JobObject mode>> "%LOG%"
  )
)

rem [3] service health best-effort (failure = degrade, never block, no patch talk)
net start SbieSvc >nul 2>&1
sc query SbieSvc 2>nul | findstr RUNNING >nul
if errorlevel 1 (echo [3] SbieSvc not running - JobObject mode>> "%LOG%") else echo [3] SbieSvc RUNNING>> "%LOG%"

rem [4] config template (user config is never overwritten)
"%B%win7-agent.exe" init >> "%LOG%" 2>&1

rem [5] environment report + mode decision
"%B%win7-agent.exe" doctor >> "%LOG%" 2>&1

type "%LOG%"
echo ===== install done ===== >> "%LOG%"
echo INSTALL-OK - see mode decision above
