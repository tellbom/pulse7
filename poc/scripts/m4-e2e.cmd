@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=%B%\m4-e2e.log
cd /d %B%
del %A% >nul 2>&1
echo M4-E2E-START %date% %time% > %A%

echo ===== S1 regression ===== >> %A%
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S1 --yolo --session %E%\m4-s1.jsonl exec "运行 python calc.py 会报错。请修复这个脚本，使 4 个人平分 120 的金额并正确输出每人金额，应为 30.0。修复后运行验证。" >> %A% 2>&1
python %E%\S1\calc.py >> %A% 2>&1

echo ===== S2 regression ===== >> %A%
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S2 --yolo --session %E%\m4-s2.jsonl exec "给这个项目加一个小功能：在合适的文件里新增 trim 函数，参数 s，去除字符串首尾空格并返回；然后在 main.py 里对前后各带两个空格的 hello 字符串调用 trim 并打印结果。完成后运行 python main.py 验证输出包含 hello。" >> %A% 2>&1
python %E%\S2\main.py >> %A% 2>&1

echo ===== S4 AGENT.md convention ===== >> %A%
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S4-agmd --yolo --session %E%\m4-s4.jsonl exec "新建 greet.py：定义变量 name 值为 Win7，然后用一句问候语打印 hello, name。用 python greet.py 验证。" >> %A% 2>&1
type %E%\S4-agmd\greet.py >> %A% 2>&1
python %E%\S4-agmd\greet.py >> %A% 2>&1

echo ===== S5 big-file compression (max-ctx 6000) ===== >> %A%
win7-agent.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S5-big --yolo --max-ctx 6000 --session %E%\m4-s5.jsonl exec "通读 big.txt 的全部 40 个 section，逐段阅读，最后用中文一句话概括每个 section 的编号与要点，全部列出。" >> %A% 2>&1

echo M4-E2E-DONE %date% %time% >> %A%
