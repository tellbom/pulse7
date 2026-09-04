@echo off
set B=C:\Users\user\win7-agent
cd /d %B%
echo M2-DEGRADE-START > m2-degrade.log
taskkill /f /im win7-agent.exe >nul 2>&1
start /b cmd /c "win7-agent.exe mock 300 > mock2.log 2>&1"
ping -n 4 127.0.0.1 >nul
win7-agent.exe --workspace C:\Users\user\ws --yolo exec "M1-SMOKE degrade round" >> m2-degrade.log 2>&1
echo EXIT=%errorlevel% >> m2-degrade.log
echo [redirect-test.txt]: >> m2-degrade.log
type C:\Users\user\ws\redirect-test.txt >> m2-degrade.log 2>&1
echo M2-DEGRADE-DONE >> m2-degrade.log
