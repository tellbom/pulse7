@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\r9-result.txt
echo ===ROUND9=== > %R%
"%SBX%" /reload
echo RELOAD-EC=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent delete_sandbox_silent
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo DIRECT3 > C:\Users\user\ws\real3.txt"
echo W-EC=%errorlevel% >> %R%
if exist C:\Users\user\ws\real3.txt (echo RELOAD-REAL-WRITE-OK >> %R%) else (echo STILL-VIRTUAL >> %R%)
dir /s /b C:\Sandbox\user\Win7Agent >> %R% 2>&1
echo ===R9-DONE=== >> %R%
