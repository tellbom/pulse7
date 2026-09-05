@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set W=C:\Users\user\real-e2e\S1
set R=C:\Users\user\.pulse7\diag\wrapper.log
echo WRAPPER-TEST-START > "%R%"

rem Replicate agent's buildRunFiles exactly
set ID=999
set D=%USERPROFILE%\.pulse7\run\%ID%
mkdir "%D%" 2>nul

echo python calc.py> "%D%\inner.cmd"
(
echo @echo off
echo cd /d "%W%"
echo call "%D%\inner.cmd" ^> "%D%\out.txt" 2^>^&1
echo echo %%errorlevel%% ^> "%D%\ec.txt"
) > "%D%\run.bat"

echo [run.bat content]: >> "%R%"
type "%D%\run.bat" >> "%R%" 2>&1

echo --- test 1: agent wrapper via Start.exe --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "%D%\run.bat"
echo StartRC=%%errorlevel%% >> "%R%"
echo [real-side out.txt]: >> "%R%"
type "%D%\out.txt" >> "%R%" 2>&1
echo [real-side ec.txt]: >> "%R%"
type "%D%\ec.txt" >> "%R%" 2>&1
echo [container out.txt]: >> "%R%"
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\run\%ID%\out.txt" >> "%R%" 2>&1
echo [container ec.txt]: >> "%R%"
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\run\%ID%\ec.txt" >> "%R%" 2>&1

echo --- test 2: direct python via Start.exe (same workspace) --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "cd /d %W% && python calc.py > %USERPROFILE%\.pulse7\diag\t2.txt 2>&1"
echo StartRC=%%errorlevel%% >> "%R%"
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\t2.txt" >> "%R%" 2>&1

echo --- test 3: Sandboxie.ini content --- >> "%R%"
type C:\Windows\Sandboxie.ini >> "%R%" 2>&1

echo WRAPPER-TEST-DONE >> "%R%"
