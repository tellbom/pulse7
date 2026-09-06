@echo off
setlocal
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=%B%\stdout-probe.log
cd /d %B%
del %A% >nul 2>&1
echo PROBE-START > %A%
echo ===== mock-source exec ===== >> %A%
start /b cmd /c "win7-agent.exe mock 90 > mock-probe.log 2>&1"
ping -n 4 127.0.0.1 >nul
win7-agent.exe --workspace %E%\S1 --yolo --session %E%\probe-mock.jsonl exec "M3-SMOKE probe" >> %A% 2>&1
echo MOCK-RC=%errorlevel% >> %A%
echo ===== real-source exec ===== >> %A%
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S1 --yolo --session %E%\probe-real.jsonl exec "只回复两个字：收到" >> %A% 2>&1
echo REAL-RC=%errorlevel% >> %A%
echo PROBE-DONE >> %A%
