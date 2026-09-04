@echo off
set SBX=C:\Program Files\Sandboxie\Start.exe
echo PROBE-DIAG-START > C:\Users\user\win7-agent\probe-diag.log
mkdir "%USERPROFILE%\.win7-agent\probe" 2>nul
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c echo OK > "%USERPROFILE%\.win7-agent\probe\diag1.txt"
echo EC=%errorlevel% >> C:\Users\user\win7-agent\probe-diag.log
echo [real side]: >> C:\Users\user\win7-agent\probe-diag.log
dir /b "%USERPROFILE%\.win7-agent\probe" >> C:\Users\user\win7-agent\probe-diag.log 2>&1
echo [container user-current]: >> C:\Users\user\win7-agent\probe-diag.log
dir /s /b "C:\Sandbox\user\Win7Agent\user\current\.win7-agent" >> C:\Users\user\win7-agent\probe-diag.log 2>&1
echo [container drive-C]: >> C:\Users\user\win7-agent\probe-diag.log
dir /s /b "C:\Sandbox\user\Win7Agent\drive\C\Users\user\.win7-agent" >> C:\Users\user\win7-agent\probe-diag.log 2>&1
echo PROBE-DIAG-DONE >> C:\Users\user\win7-agent\probe-diag.log
