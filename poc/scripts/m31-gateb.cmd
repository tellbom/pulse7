@echo off
set B=C:\Users\user\win7-agent
set W=C:\Users\user\ws4
cd /d %B%
del %B%\m31-gateb.log >nul 2>&1
echo M31-GATEB-START %date% %time% > %B%\m31-gateb.log
taskkill /f /im win7-agent.exe >nul 2>&1
echo M31-NOTE > %W%\note.txt
start /b cmd /c "win7-agent.exe mock 300 > mock31b.log 2>&1"
ping -n 4 127.0.0.1 >nul
win7-agent.exe --workspace %W% --yolo exec "M3-SMOKE degraded-gate" >> %B%\m31-gateb.log 2>&1
echo B-EXIT=%errorlevel% >> %B%\m31-gateb.log
win7-agent.exe doctor >> %B%\m31-gateb.log 2>&1
echo M31-GATEB-DONE %date% %time% >> %B%\m31-gateb.log
if not exist %B%\m31-gateb.log echo GATE-FAIL-NO-LOG >> %B%\m31-gateb.log
