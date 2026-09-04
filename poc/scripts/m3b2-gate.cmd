@echo off
set B=C:\Users\user\win7-agent
set W=C:\Users\user\ws4
cd /d %B%
del m3b2-gate.log >nul 2^&1
echo M3B-START %date% %time% > m3b2-gate.log
taskkill /f /im win7-agent.exe >nul 2>&1
echo M3-NOTE-B > %W%\note.txt
start /b cmd /c "win7-agent.exe mock 300 > mock3b.log 2>&1"
ping -n 4 127.0.0.1 >nul
echo ===== M3-B: main product in zero-Sandboxie boot ===== >> m3b2-gate.log
win7-agent.exe --workspace %W% --yolo exec "M3-SMOKE degraded" >> m3b2-gate.log 2>&1
echo B-EXIT=%errorlevel% >> m3b2-gate.log
win7-agent.exe doctor >> m3b2-gate.log 2>&1
echo ===== lifecycle run2: wa-test after reboot ===== >> m3b2-gate.log
C:\Users\user\wa-test\win7-agent.exe --workspace C:\Users\user\ws-life --yolo exec "M3-SMOKE lifecycle" >> m3b2-gate.log 2>&1
echo LIFE2-EXIT=%errorlevel% >> m3b2-gate.log
echo ===== uninstall wa-test ===== >> m3b2-gate.log
cmd /c C:\Users\user\wa-test\uninstall.cmd >> m3b2-gate.log 2>&1
ping -n 6 127.0.0.1 >nul
if exist C:\Users\user\wa-test (echo UNINSTALL-FAIL-dir-remains >> m3b2-gate.log) else (echo UNINSTALL-OK-dir-removed >> m3b2-gate.log)
echo [ws4 kept]: >> m3b2-gate.log
dir /b %W% >> m3b2-gate.log
echo [ws-life kept]: >> m3b2-gate.log
dir /b C:\Users\user\ws-life >> m3b2-gate.log
if exist "C:\Program Files\Sandboxie\Start.exe" echo SANDBOXIE-KEPT-OK >> m3b2-gate.log
echo M3B-DONE %date% %time% >> m3b2-gate.log
if not exist m3b2-gate.log echo GATE-FAIL-NO-LOG-M3B >> m3b2-gate.log
