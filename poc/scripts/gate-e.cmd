@echo off
setlocal
set SBX=C:\Program Files\Sandboxie-Plus\Start.exe
set BOX=Win7Agent
echo ===== GATE E : Sandboxie Plus Start.exe CLI =====

echo ----- E0: services -----
sc query SbieSvc | findstr STATE
sc query SbieDrv | findstr STATE

echo ----- E1: stdout/stderr/exitcode -----
"%SBX%" /box:%BOX% /wait cmd.exe /c "echo hello-from-box & echo err-from-box 1>&2 & exit 42"
echo E1-DIRECT-EXITCODE=%errorlevel%
if "%errorlevel%"=="42" (echo MARK e1-exitcode-ok) else (echo MARK e1-exitcode-NOT-propagated)

echo ----- E2: listpids with long job running -----
start /b "" "%SBX%" /box:%BOX% /wait cmd.exe /c "ping -n 60 127.0.0.1 >nul"
ping -n 3 127.0.0.1 >nul
"%SBX%" /box:%BOX% /listpids

echo ----- E3: terminate -----
"%SBX%" /box:%BOX% /terminate && echo MARK e3-terminate-ok || echo MARK e3-terminate-FAIL
ping -n 3 127.0.0.1 >nul
echo [listpids after terminate, expect empty]:
"%SBX%" /box:%BOX% /listpids

echo ----- E4: file isolation (write must be virtualized) -----
"%SBX%" /box:%BOX% /wait cmd.exe /c "echo ISOLATION-PROOF > %USERPROFILE%\sbx-proof.txt"
if exist "%USERPROFILE%\sbx-proof.txt" (echo MARK e4-FAIL-leaked-to-real-fs) else (echo MARK e4-isolation-ok)
echo [container copy of the boxed write]:
dir /s /b C:\Sandbox\user\Win7Agent\*sbx-proof.txt 2>nul

echo ----- E5: read outside sandbox (documented default behavior) -----
"%SBX%" /box:%BOX% /wait cmd.exe /c "type C:\Users\user\win7-agent\gate-a.cmd >nul && echo READ-OUTSIDE-OK"

echo ----- E6: cleanup -----
"%SBX%" /box:%BOX% delete_sandbox_silent
echo MARK e6-cleanup-issued
if exist C:\Sandbox\user\Win7Agent (echo container-dir-still-exists) else (echo container-dir-removed)

echo ----- E7: repeat stability x3 -----
"%SBX%" /box:%BOX% /wait cmd.exe /c "exit 1"
echo E7-RUN1-EXITCODE=%errorlevel%
"%SBX%" /box:%BOX% /wait cmd.exe /c "exit 2"
echo E7-RUN2-EXITCODE=%errorlevel%
"%SBX%" /box:%BOX% /wait cmd.exe /c "exit 3"
echo E7-RUN3-EXITCODE=%errorlevel%

echo ----- E8: python inside box -----
"%SBX%" /box:%BOX% /wait cmd.exe /c "python --version"

echo ----- E9: bundled MinGit inside box -----
"%SBX%" /box:%BOX% /wait cmd.exe /c "C:\Users\user\win7-agent\runtime\git\cmd\git.exe --version"

echo ===== GATE E DONE =====
