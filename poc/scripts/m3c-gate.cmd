@echo off
set B=C:\Users\user\win7-agent
set R=%B%\m3c-gate.log
del %R% >nul 2>&1
del %B%\m3c-inner.log >nul 2>&1
echo M3C-START %date% %time% > %R%
cd /d %B%
rd /s /q C:\Users\user\ws5 2>nul
mkdir C:\Users\user\ws5
echo M3C-NOTE > C:\Users\user\ws5\note.txt
runas /trustlevel:0x20000 "cmd /c C:\Users\user\win7-agent\m3c-inner.cmd"
ping -n 20 127.0.0.1 >nul
type %B%\m3c-inner.log >> %R% 2>nul
echo [ws5 dir]: >> %R%
dir /b C:\Users\user\ws5 >> %R%
echo M3C-DONE %date% %time% >> %R%
if not exist %R% echo GATE-FAIL-NO-LOG >> %R%
