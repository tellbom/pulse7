@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
echo --- T1: simple echo in box ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c echo TESTOK
echo T1-EC=%errorlevel%
echo --- T2: payload e1.cmd in box ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c C:\Users\user\win7-agent\e1.cmd
echo T2-EC=%errorlevel%
echo --- T3: listpids ---
"%SBX%" /box:Win7Agent /listpids
echo T3-EC=%errorlevel%
echo --- T4: unknown box (error path) ---
"%SBX%" /box:NoSuchBox /wait cmd.exe /c echo X
echo T4-EC=%errorlevel%
echo --- T5: Sandboxie.ini content ---
type C:\Windows\Sandboxie.ini
echo --- T6: runas limited trustlevel (non-admin runtime proof) ---
runas /trustlevel:0x20000 "cmd /c ""%SBX%"" /box:Win7Agent /wait cmd.exe /c echo NONADMIN-TESTOK > C:\Users\user\win7-agent\nonadmin-out.txt 2>&1"
ping -n 4 127.0.0.1 >nul
type C:\Users\user\win7-agent\nonadmin-out.txt 2>nul
echo DIAG-DONE
