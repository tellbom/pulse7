@echo off
setlocal
set SBX=C:\Program Files\Sandboxie\Start.exe
set BOX=Win7Agent
set CTN=C:\Sandbox\user\Win7Agent
echo --- F0: application error events (first, so we never lose them) ---
wevtutil qe Application "/q:*[System[(Level=2)]]" /f:text /c:5 /rd:true
echo --- F0b: system error events ---
wevtutil qe System "/q:*[System[(Level=2)]]" /f:text /c:3 /rd:true
echo --- F1: kill stray Start.exe ---
taskkill /f /im Start.exe 2>nul
echo --- F2: async launch (no /wait), payload writes into box ---
"%SBX%" /box:%BOX% cmd.exe /c "echo ASYNC-MARK > C:\Users\user\win7-agent\async-out.txt"
echo F2-LAUNCH-EC=%errorlevel%
echo --- F3: poll container for the file ---
for /L %%i in (1,1,10) do (
  if not exist "%CTN%\drive\C\Users\user\win7-agent\async-out.txt" ping -n 2 127.0.0.1 >nul
)
type "%CTN%\drive\C\Users\user\win7-agent\async-out.txt" 2>nul
echo --- F4: listpids ---
"%SBX%" /box:%BOX% /listpids
echo F4-EC=%errorlevel%
echo --- F5: terminate box ---
"%SBX%" /box:%BOX% /terminate
echo F5-EC=%errorlevel%
echo DIAG5-DONE
