@echo off
set B=C:\Users\user\win7-agent
set W=C:\Users\user\ws
cd /d %B%
echo M2-SMOKE-START > m2-smoke.log
taskkill /f /im win7-agent.exe >nul 2>&1
del %W%\redirect-test.txt 2>nul
del %W%\m2extra.txt 2>nul
rmdir /s /q %W%\m2 2>nul
echo HELLO-FROM-WIN7-NOTE > %W%\note.txt
start /b cmd /c "win7-agent.exe mock 600 > mock.log 2>&1"
ping -n 4 127.0.0.1 >nul
echo ====== RUN1 M1-SMOKE sandboxie + redirect-fix ====== >> m2-smoke.log
win7-agent.exe --workspace %W% --yolo --session %B%\data\sessions\s2m1.jsonl exec "M1-SMOKE round" >> m2-smoke.log 2>&1
echo RUN1-EXIT=%errorlevel% >> m2-smoke.log
echo [redirect-test.txt expected]: >> m2-smoke.log
type %W%\redirect-test.txt >> m2-smoke.log 2>&1
echo ====== RUN2 M2-FILES ====== >> m2-smoke.log
win7-agent.exe --workspace %W% --yolo exec "M2-FILES round" >> m2-smoke.log 2>&1
echo RUN2-EXIT=%errorlevel% >> m2-smoke.log
echo [note2.txt expected M2-LINE-1-EDITED]: >> m2-smoke.log
type %W%\m2\note2.txt >> m2-smoke.log 2>&1
echo ====== RUN3 M2-GIT ====== >> m2-smoke.log
win7-agent.exe --workspace %W% --yolo --session %B%\data\sessions\s2m3.jsonl exec "M2-GIT sequence" >> m2-smoke.log 2>&1
echo RUN3-EXIT=%errorlevel% >> m2-smoke.log
if exist %W%\m2extra.txt (echo M2GIT-FAIL-extra-remains >> m2-smoke.log) else (echo M2GIT-OK-extra-removed >> m2-smoke.log)
echo [note.txt intact]: >> m2-smoke.log
type %W%\note.txt >> m2-smoke.log 2>&1
echo ====== RUN4 resume ====== >> m2-smoke.log
win7-agent.exe --workspace %W% --yolo --resume %B%\data\sessions\s2m3.jsonl exec "resume check" >> m2-smoke.log 2>&1
echo RUN4-EXIT=%errorlevel% >> m2-smoke.log
echo ====== EVIDENCE ====== >> m2-smoke.log
dir /b %W% >> m2-smoke.log
type %B%\data\sessions\audit.jsonl >> m2-smoke.log 2>&1
echo M2-SMOKE-DONE >> m2-smoke.log
