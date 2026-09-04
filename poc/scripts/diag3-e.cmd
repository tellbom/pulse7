@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
echo --- C0: kill stray Start.exe / SbieSvc ---
taskkill /f /im Start.exe 2>nul
taskkill /f /im SbieSvc.exe 2>nul
ping -n 3 127.0.0.1 >nul
echo --- C1: restart SbieSvc cleanly ---
net start SbieSvc
sc query SbieSvc | findstr STATE
tasklist | findstr /i "SbieSvc Start.exe"
echo --- C2: container root ---
dir C:\Sandbox 2>nul
echo --- C3: retry silent echo in box ---
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c echo TESTOK
echo C3-EC=%errorlevel%
echo --- C4: application error events ---
wevtutil qe Application "/q:*[System[(Level=2)]]" /f:text /c:4 /rd:true
echo DIAG3-DONE
