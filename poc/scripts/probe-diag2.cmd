@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\probe-diag2.log
set D=.win7-agent\run\777
echo PD2-START > %R%
mkdir "%USERPROFILE%\%D%" 2>nul
echo echo PROBE-OK> "%USERPROFILE%\%D%\inner.cmd"
(
echo @echo off
echo cd /d "C:\Users\user\ws4"
echo call "%%USERPROFILE%%\%D%\inner.cmd" ^> "%%USERPROFILE%%\%D%\out.txt" 2^>^&1
echo echo %%errorlevel%% ^> "%%USERPROFILE%%\%D%\ec.txt"
) > "%USERPROFILE%\%D%\run.bat"
type "%USERPROFILE%\%D%\run.bat" >> %R%
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "%USERPROFILE%\%D%\run.bat"
echo STX-EC=%errorlevel% >> %R%
echo [real side]: >> %R%
dir /b "%USERPROFILE%\%D%" >> %R% 2>&1
type "%USERPROFILE%\%D%\out.txt" >> %R% 2>&1
type "%USERPROFILE%\%D%\ec.txt" >> %R% 2>&1
echo [container side]: >> %R%
dir /b "C:\Sandbox\user\Win7Agent\user\current\%D%" >> %R% 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\%D%\out.txt" >> %R% 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\%D%\ec.txt" >> %R% 2>&1
echo PD2-DONE >> %R%
