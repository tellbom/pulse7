@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\ie-result.txt
echo ===INTERACTIVE-SESSION-TEST=== > %R%
echo SESSION=%SESSIONNAME% >> %R%
echo --- I1: boxed echo stdout + /wait ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c echo TESTOK-FROM-BOX >> %R% 2>&1
echo I1-EC=%errorlevel% >> %R%
echo --- I2: exit code 42 propagation ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 42 >> %R% 2>&1
echo I2-EC=%errorlevel% >> %R%
echo --- I3: stderr capture ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo ERR-LINE 1>&2" >> %R% 2>&1
echo I3-EC=%errorlevel% >> %R%
echo --- I4: listpids + terminate ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "ping -n 30 127.0.0.1 >nul" >> %R% 2>&1
"%SBX%" /box:Win7Agent /listpids >> %R% 2>&1
echo I4-LIST-EC=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /terminate >> %R% 2>&1
echo I4-TERM-EC=%errorlevel% >> %R%
echo --- I5: file isolation ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo ISOLATION-PROOF > %USERPROFILE%\sbx-proof.txt" >> %R% 2>&1
if exist "%USERPROFILE%\sbx-proof.txt" (echo I5-LEAKED-TO-REAL-FS >> %R%) else (echo I5-ISOLATED-OK >> %R%)
echo --- I6: NON-ADMIN runtime (restricted trustlevel) ---
runas /trustlevel:0x20000 "cmd /c ""%SBX%"" /box:Win7Agent /wait cmd.exe /c echo NONADMIN-BOX-OK >> %R% 2>&1"
ping -n 8 127.0.0.1 >nul
echo --- I7: repeat stability x3 ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 1 >> %R% 2>&1
echo I7-R1=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 2 >> %R% 2>&1
echo I7-R2=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 3 >> %R% 2>&1
echo I7-R3=%errorlevel% >> %R%
echo ===IE-DONE=== >> %R%
