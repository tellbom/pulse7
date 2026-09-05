@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set W=C:\Users\user\T7\process-copy
set A=C:\Users\user\T7\retr-r2.log
cd /d %B%
del %A% >nul 2>&1
echo RETR-R2-START %date% %time% > %A%
del %B%\data\logs\agent.log >nul 2>&1
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %W% --max-ctx 12000 --session C:\Users\user\T7\retr-r2-session.jsonl exec "说明这个项目的入口文件和主要模块分工。不要修改任何文件。" >> %A% 2>&1
echo RETR-R2-EXIT=%%ERRORLEVEL%% >> %A%
cd /d %W% && git status --porcelain >> %A% 2>&1
cd /d %B%
echo ===== compress records ===== >> %A%
findstr /c:"_compress" %B%\data\sessions\audit.jsonl >> %A% 2>&1
echo RETR-R2-DONE %date% %time% >> %A%
