@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\r7-result.txt
echo ===ROUND7=== > %R%
"%SBX%" /box:Win7Agent delete_sandbox_silent
echo K0-CLEAN-EC=%errorlevel% >> %R%

echo --- K1: boxed write into OpenFilePath dir (real ws) ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo KMARK-WS > C:\Users\user\ws\real.txt"
echo K1-EC=%errorlevel% >> %R%
if exist C:\Users\user\ws\real.txt (echo K1-REAL-WRITE-OK >> %R%) else (echo K1-NOT-IN-REAL >> %R%)

echo --- K2: boxed write into virtual dir (win7-agent) ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo KMARK2 > C:\Users\user\win7-agent\k2.txt"
echo K2-EC=%errorlevel% >> %R%
if exist C:\Users\user\win7-agent\k2.txt (echo K2-LEAKED-TO-REAL >> %R%) else (echo K2-NOT-IN-REAL >> %R%)

echo --- K3: boxed write to C root ---
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo KMARK3 > C:\k3.txt"
echo K3-EC=%errorlevel% >> %R%
if exist C:\k3.txt (echo K3-LEAKED-TO-REAL >> %R%) else (echo K3-NOT-IN-REAL >> %R%)

echo --- K4: full sandbox tree ---
dir /s /b C:\Sandbox >> %R% 2>&1
echo --- K5: container root listing ---
dir /s /b C:\Sandbox\user\Win7Agent >> %R% 2>&1
echo ===R7-DONE=== >> %R%
