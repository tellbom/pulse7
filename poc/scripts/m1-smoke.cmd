@echo off
set B=C:\Users\user\win7-agent
cd /d %B%
echo M1-SMOKE-START > m1-smoke.log
echo HELLO-FROM-WIN7-NOTE > C:\Users\user\ws\note.txt
start /b cmd /c "win7-agent.exe mock 150 > mock.log 2>&1"
ping -n 4 127.0.0.1 >nul
win7-agent.exe --workspace C:\Users\user\ws --yolo exec "M1-SMOKE: run all tools" >> m1-smoke.log 2>&1
echo AGENT-EXIT=%errorlevel% >> m1-smoke.log
echo ---WS-DIR--- >> m1-smoke.log
dir /b C:\Users\user\ws >> m1-smoke.log 2>&1
echo ---NOTE.TXT--- >> m1-smoke.log
type C:\Users\user\ws\note.txt >> m1-smoke.log 2>&1
echo ---SHELL-OUT.TXT--- >> m1-smoke.log
type C:\Users\user\ws\shell-out.txt >> m1-smoke.log 2>&1
echo ---AUDIT--- >> m1-smoke.log
type %B%\data\sessions\audit.jsonl >> m1-smoke.log 2>&1
echo ---MOCK.LOG--- >> m1-smoke.log
type mock.log >> m1-smoke.log 2>&1
echo M1-SMOKE-DONE >> m1-smoke.log
