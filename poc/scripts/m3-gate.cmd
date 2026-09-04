@echo off
set B=C:\Users\user\win7-agent
set W=C:\Users\user\ws4
cd /d %B%
echo M3-GATE-START > m3-gate.log
taskkill /f /im win7-agent.exe >nul 2>&1
rd /s /q %W% 2>nul
mkdir %W%
echo M3-NOTE > %W%\note.txt
start /b cmd /c "win7-agent.exe mock 900 > mock3.log 2>&1"
ping -n 4 127.0.0.1 >nul
echo ===== M3-A: desktop session ===== >> m3-gate.log
win7-agent.exe --workspace %W% --yolo exec "M3-SMOKE gate" >> m3-gate.log 2>&1
echo A-EXIT=%errorlevel% >> m3-gate.log
win7-agent.exe doctor >> m3-gate.log 2>&1
echo [ws4 dir]: >> m3-gate.log
dir /b %W% >> m3-gate.log
echo [m3shell.txt content]: >> m3-gate.log
type %W%\m3shell.txt >> m3-gate.log 2>&1
echo M3-GATE-DONE >> m3-gate.log
