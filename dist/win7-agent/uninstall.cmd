@echo off
setlocal
set B=%~dp0
set START=C:\Program Files\Sandboxie\Start.exe
echo ===== win7-agent uninstall =====

taskkill /f /im win7-agent.exe >nul 2>&1

rem clear the dedicated agent box (content + ini section). USER WORKSPACE,
rem USER GIT REPOS AND USER FILES ARE INTENTIONALLY NOT TOUCHED.
if exist "%START%" (
  "%START%" /box:Win7Agent /terminate >nul 2>&1
  "%START%" /box:Win7Agent delete_sandbox_silent >nul 2>&1
  if exist "C:\Program Files\Sandboxie\SbieIni.exe" (
    "C:\Program Files\Sandboxie\SbieIni.exe" delete Win7Agent >nul 2>&1
    "%START%" /reload >nul 2>&1
  )
  echo agent box removed
) else (
  echo Sandboxie not present - nothing to clean
)

rem /full additionally removes Sandboxie itself (only if the user asks;
rem Sandboxie may be used for other purposes).
if /i "%~1"=="/full" (
  if exist "C:\Program Files\Sandboxie\Un.exe" (
    echo /full: uninstalling Sandboxie Classic...
    "C:\Program Files\Sandboxie\Un.exe" /S
  ) else (
    echo /full: Sandboxie uninstaller not found, remove it manually
  )
)

rem wrapper staging dirs under the user profile
rmdir /s /q "%USERPROFILE%\.win7-agent" 2>nul

rem remove product dir (delayed so this script can finish first)
cd /d "%TEMP%"
start /b cmd /c "ping -n 3 127.0.0.1 >nul & rmdir /s /q "%B%""
echo UNINSTALL-OK - user workspace untouched
