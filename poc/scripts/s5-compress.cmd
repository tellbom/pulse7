@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=%E%\s5-compress.log
cd /d %B%
del %A% >nul 2>&1
echo S5-COMPRESS-START %date% %time% > %A%
del %B%\data\logs\agent.log >nul 2>&1
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S5-big --yolo --max-ctx 6000 --session %E%\s5c-session.jsonl exec "通读 big.txt 的全部 40 个 section，逐段阅读，最后用中文一句话概括每个 section 的编号与要点，全部列出。" >> %A% 2>&1
echo ===== agent.log ===== >> %A%
type %B%\data\logs\agent.log >> %A% 2>&1
echo ===== audit compress records ===== >> %A%
findstr /c:"_compress" %B%\data\sessions\audit.jsonl >> %A% 2>&1
echo S5-COMPRESS-DONE %date% %time% >> %A%
