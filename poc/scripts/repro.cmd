@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=C:\Users\user\repro.log
cd /d %B%
del %A% >nul 2>&1
echo REPRO-START > %A%
echo ===== test1: real endpoint, tool task ===== >> %A%
%W% 2>nul
%B%\win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S1 --yolo --session %E%\repro1.jsonl exec "运行 python calc.py 检查输出" >> %A% 2>&1
echo RC1-SAVED >> %A%
echo ===== test2: real endpoint, no tools ===== >> %A%
%B%\win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S1 --yolo --session %E%\repro2.jsonl exec "只回复两个字：收到" >> %A% 2>&1
echo RC2-SAVED >> %A%
echo REPRO-DONE >> %A%
