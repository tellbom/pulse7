@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set P=C:\Users\user\.pulse7\diag
mkdir "%P%" 2>nul
set R=C:\Users\user\.pulse7\diag\result.log
echo T1-DIAG-START > "%R%"

echo --- a) python --version in sandbox --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "python --version > %P%\a.txt 2>&1" 
echo StartRC=%errorlevel% >> "%R%"
type "%P%\a.txt" >> "%R%" 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\a.txt" >> "%R%" 2>&1

echo --- b) ver in sandbox --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "ver > %P%\b.txt 2>&1"
echo StartRC=%errorlevel% >> "%R%"
type "%P%\b.txt" >> "%R%" 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\b.txt" >> "%R%" 2>&1

echo --- c) MinGit in sandbox --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "C:\Users\user\win7-agent\runtime\git\cmd\git.exe --version > %P%\c.txt 2>&1"
echo StartRC=%errorlevel% >> "%R%"
type "%P%\c.txt" >> "%R%" 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\c.txt" >> "%R%" 2>&1

echo --- d) python with full path in sandbox --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "C:\Users\user\AppData\Local\Programs\Python\Python38\python.exe --version > %P%\d.txt 2>&1"
echo StartRC=%errorlevel% >> "%R%"
type "%P%\d.txt" >> "%R%" 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\d.txt" >> "%R%" 2>&1

echo --- e) echo simple in sandbox --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "echo HELLO > %P%\e.txt 2>&1"
echo StartRC=%errorlevel% >> "%R%"
type "%P%\e.txt" >> "%R%" 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\e.txt" >> "%R%" 2>&1

echo --- f) python -c print in sandbox --- >> "%R%"
"%SBX%" /silent /box:Win7Agent /wait cmd.exe /c "C:\Users\user\AppData\Local\Programs\Python\Python38\python.exe -c \"print('PYOK')\" > %P%\f.txt 2>&1"
echo StartRC=%errorlevel% >> "%R%"
type "%P%\f.txt" >> "%R%" 2>&1
type "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag\f.txt" >> "%R%" 2>&1

echo --- g) container dir listing --- >> "%R%"
dir /b "C:\Sandbox\user\Win7Agent\user\current\.pulse7\diag" >> "%R%" 2>&1

echo --- h) Sandboxie.ini OpenFilePath entries --- >> "%R%"
findstr /i "OpenFilePath\|OpenKeyPath\|OpenPipePath" C:\Windows\Sandboxie.ini >> "%R%" 2>&1

echo T1-DIAG-DONE >> "%R%"
