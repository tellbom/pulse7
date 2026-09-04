@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\r6-result.txt
set CTN=C:\Sandbox\user\Win7Agent
echo ===ROUND6=== > %R%

echo --- J1: exit code fidelity ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 0
echo J1-EC0=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 1
echo J1-EC1=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 2
echo J1-EC2=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 3
echo J1-EC3=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 42
echo J1-EC42=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c exit 255
echo J1-EC255=%errorlevel% >> %R%

echo --- J2: stdout/stderr via in-box file capture ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c C:\Users\user\win7-agent\r6cap.cmd >> %R% 2>&1
echo J2-WAIT-EC=%errorlevel% >> %R%
echo [real path should be empty]: >> %R%
type C:\Users\user\win7-agent\boxcap.txt 2>nul >> %R%
echo [container path]: >> %R%
type "%CTN%\drive\C\Users\user\win7-agent\boxcap.txt" 2>nul >> %R%

echo --- J3: direct access to workspace via OpenFilePath ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo DIRECT-ACCESS-OK > C:\Users\user\ws\real.txt"
echo J3-EC=%errorlevel% >> %R%
if exist C:\Users\user\ws\real.txt (echo J3-REAL-WRITE-OK >> %R%) else (echo J3-FAIL >> %R%)

echo --- J4: non-admin runtime ---
del C:\Users\user\win7-agent\nonadmin-result.txt 2>nul
runas /trustlevel:0x20000 C:\Users\user\win7-agent\nonadmin-e.cmd
ping -n 12 127.0.0.1 >nul
type C:\Users\user\win7-agent\nonadmin-result.txt >> %R% 2>nul

echo --- J5: sandbox cleanup ---
"%SBX%" /box:Win7Agent delete_sandbox_silent
echo J5-EC=%errorlevel% >> %R%
dir /b "%CTN%" 2>nul >> %R%

echo --- J6: read outside box ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "type C:\Users\user\win7-agent\gate-a.cmd >nul && echo READ-OUTSIDE-OK > C:\Users\user\win7-agent\readok.txt"
type "%CTN%\drive\C\Users\user\win7-agent\readok.txt" 2>nul >> %R%

echo --- J7: python and MinGit inside box ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "python --version > C:\Users\user\win7-agent\pyver.txt 2>&1"
type "%CTN%\drive\C\Users\user\win7-agent\pyver.txt" 2>nul >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c "C:\Users\user\win7-agent\runtime\git\cmd\git.exe --version > C:\Users\user\win7-agent\gitver.txt 2>&1"
type "%CTN%\drive\C\Users\user\win7-agent\gitver.txt" 2>nul >> %R%

echo ===R6-DONE=== >> %R%
