@echo off
set B=C:\Users\user\win7-agent
cd /d %B%
echo M3C-INNER-START > m3c-inner.log
whoami >> m3c-inner.log
win7-agent.exe --workspace C:\Users\user\ws5 --yolo exec "M3-SMOKE restricted" >> m3c-inner.log 2>&1
echo INNER-EXIT=%errorlevel% >> m3c-inner.log
win7-agent.exe doctor >> m3c-inner.log 2>&1
echo M3C-INNER-DONE >> m3c-inner.log
