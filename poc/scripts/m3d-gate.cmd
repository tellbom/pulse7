@echo off
set B=C:\Users\user\win7-agent
cd /d %B%
del m3d-gate.log >nul 2^&1
echo M3D-START %date% %time% > m3d-gate.log
tasklist | findstr /i "win7-agent" >nul || start /b cmd /c "win7-agent.exe mock 300 > mock3d.log 2>&1"
ping -n 4 127.0.0.1 >nul
win7-agent.exe --workspace C:\Users\user\ws4 --yolo exec "M3-SMOKE headless" >> m3d-gate.log 2>&1
echo D-EXIT=%errorlevel% >> m3d-gate.log
win7-agent.exe doctor >> m3d-gate.log 2>&1
echo M3D-DONE %date% %time% >> m3d-gate.log
if not exist m3d-gate.log echo GATE-FAIL-NO-LOG-M3D >> m3d-gate.log
