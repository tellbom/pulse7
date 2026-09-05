@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=C:\Users\user\T7\shell20.log
cd /d %B%
del %A% >nul 2>&1
echo SHELL20-START %date% %time% > %A%
del %B%\data\logs\agent.log >nul 2>&1
set /a OK=0
set /a FAIL=0
for /l %%i in (1,1,20) do (
  %B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S1 --yolo --session %E%\s20-%%i.jsonl exec "只运行 echo SHELL20-%%i-OK 这一个命令然后告诉我输出" >> %A% 2>&1
  findstr /c:"SHELL20-%%i-OK" %B%\data\logs\agent.log >nul 2>&1
  if errorlevel 1 (
    set /a FAIL+=1
    echo [%%i] FAIL >> %A%
  ) else (
    set /a OK+=1
    echo [%%i] OK >> %A%
  )
)
echo TOTAL: OK=%OK% FAIL=%FAIL% >> %A%
echo ===== box processes after ===== >> %A%
"%B%\..\..\Program Files\Sandboxie\Start.exe" /box:Win7Agent /listpids >> %A% 2>&1
echo SHELL20-DONE %date% %time% >> %A%
