@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set W=C:\Users\user\T7\process-copy
set A=C:\Users\user\T7\retr-r1.log
cd /d %B%
del %A% >nul 2>&1
echo RETR-R1-START %date% %time% > %A%
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %W% --session C:\Users\user\T7\retr-r1-session.jsonl exec "在 process\Api\Filters\GlobalExceptionFilter.cs 里，把所有英文错误提示消息改成中文。只修改这一个文件，不要修改其他文件。修改后用 grep 验证修改成功。" >> %A% 2>&1
echo RETR-R1-EXIT=%%ERRORLEVEL%% >> %A%
cd /d %W% && git diff --name-only >> %A% 2>&1
cd /d %B%
echo RETR-R1-DONE %date% %time% >> %A%
