@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\r8-result.txt
echo ===ROUND8=== > %R%
net stop SbieSvc >> %R% 2>&1
net start SbieSvc >> %R% 2>&1
sc query SbieSvc | findstr STATE >> %R%
"%SBX%" /box:Win7Agent delete_sandbox_silent
echo CLEAN-EC=%errorlevel% >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo DIRECT2 > C:\Users\user\ws\real2.txt"
echo W-EC=%errorlevel% >> %R%
if exist C:\Users\user\ws\real2.txt (echo OPENFILEPATH-REAL-WRITE-OK >> %R%) else (echo OPENFILEPATH-STILL-VIRTUAL >> %R%)
dir /s /b C:\Sandbox\user\Win7Agent >> %R% 2>&1
echo ===R8-DONE=== >> %R%
