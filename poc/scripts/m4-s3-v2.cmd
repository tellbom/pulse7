@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=%E%\s3v2-run.log
cd /d %B%
del %A% >nul 2>&1
echo S3V2-START %date% %time% > %A%
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S3 --yolo --session %E%\s3v2-session.jsonl exec "把这个项目整理一下，让它更好。" >> %A% 2>&1
echo [S3 dir after]: >> %A%
dir /b %E%\S3 >> %A% 2>&1
echo S3V2-DONE %date% %time% >> %A%
