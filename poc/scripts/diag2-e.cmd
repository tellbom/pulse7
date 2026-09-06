@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
echo --- P0: stray processes ---
tasklist | findstr /i "Start.exe Sbie cmd.exe"
echo --- P1: append DefaultBox to ini ---
echo [DefaultBox]>>C:\Windows\Sandboxie.ini
echo Enabled=y>>C:\Windows\Sandboxie.ini
type C:\Windows\Sandboxie.ini
echo --- P2: /silent Win7Agent echo ---
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c echo TESTOK
echo P2-EC=%errorlevel%
echo --- P3: /silent DefaultBox echo ---
"%SBX%" /silent /wait cmd.exe /c echo DEFBOX-OK
echo P3-EC=%errorlevel%
echo --- P4: listpids no wait ---
"%SBX%" /box:Win7Agent /listpids
echo P4-EC=%errorlevel%
echo --- P5: application event log tail ---
wevtutil qe Application /f:text /c:6 /rd:true
echo DIAG2-DONE
