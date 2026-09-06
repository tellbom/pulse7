@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set BOX=Win7Agent
set CTN=C:\Sandbox\user\Win7Agent
echo --- D0: container state before ---
dir /s /b %CTN% 2>nul
echo --- D1: pure exit, no console IO, /wait /silent ---
"%SBX%" /silent /box:%BOX% /wait cmd.exe /c exit
echo D1-EC=%errorlevel%
echo --- D2: boxed write into container path, NO /wait ---
"%SBX%" /silent /box:%BOX% cmd.exe /c "echo NOWAIT-MARK > C:\Users\user\win7-agent\boxout.txt"
echo D2-LAUNCH-EC=%errorlevel%
ping -n 6 127.0.0.1 >nul
echo [real path should be empty]:
type C:\Users\user\win7-agent\boxout.txt 2>nul
echo [container path should have it]:
type "%CTN%\drive\C\Users\user\win7-agent\boxout.txt" 2>nul
echo --- D3: interactive sessions ---
query user
echo --- D4: container tree after ---
dir /s /b %CTN% 2>nul
echo DIAG4-DONE
