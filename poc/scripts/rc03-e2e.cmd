@echo off
setlocal
set WIN7_AGENT_API_KEY=sk-3uPkEmo0E2rZEzLgzS4pizjGdp5w4OAo7lJM3x8A78sLhaw2
set B=C:\Users\user\win7-agent
set E=C:\Users\user\real-e2e
set A=%B%\rc03-e2e.log
cd /d %B%
del %A% >nul 2>&1
echo RC03-START %date% %time% > %A%

echo ===== S1 clear task ===== >> %A%
%B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S1 --yolo --session %E%\rc03-s1.jsonl exec "运行 python calc.py 会报错。请修复这个脚本，使 4 个人平分 120 的金额并正确输出每人金额，应为 30.0。修复后运行验证。" >> %A% 2>&1
python %E%\S1\calc.py >> %A% 2>&1

echo ===== S2 clear task (T1 verdict) ===== >> %A%
%B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S2 --yolo --session %E%\rc03-s2.jsonl exec "给这个项目加一个小功能：在合适的文件里新增 trim 函数，参数 s，去除字符串首尾空格并返回；然后在 main.py 里对前后各带两个空格的 hello 字符串调用 trim 并打印结果。完成后运行 python main.py 验证输出包含 hello。" >> %A% 2>&1
python %E%\S2\main.py >> %A% 2>&1

echo ===== S3 vague task ===== >> %A%
%B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace %E%\S3 --yolo --session %E%\rc03-s3.jsonl exec "把这个项目整理一下，让它更好。" >> %A% 2>&1
echo [S3 workspace after]: >> %A%
dir /b %E%\S3 >> %A% 2>&1

echo ===== R1 real project fix ===== >> %A%
cmd /c C:\Users\user\win7-agent\reset-project.cmd >> %A% 2>&1
%B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace C:\Users\user\T7\process-copy --session %E%\rc03-r1.jsonl exec "在 process\Api\Filters\GlobalExceptionFilter.cs 里，把所有英文错误提示消息改成中文。只修改这一个文件，不要修改其他文件。修改后用 grep 验证修改成功。" >> %A% 2>&1
cd /d C:\Users\user\T7\process-copy && git diff --stat >> %A% 2>&1
cd /d %B%

echo ===== R2 run 2 of 3 ===== >> %A%
cmd /c C:\Users\user\win7-agent\reset-project.cmd >> %A% 2>&1
%B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace C:\Users\user\T7\process-copy --max-ctx 12000 --session %E%\rc03-r2a.jsonl exec "说明这个项目的入口文件和主要模块分工。不要修改任何文件。" >> %A% 2>&1

echo ===== R2 run 3 of 3 ===== >> %A%
cmd /c C:\Users\user\win7-agent\reset-project.cmd >> %A% 2>&1
%B%\pulse7.exe --base-url https://aigc789.top/v1 --model deepseek-v4-flash --workspace C:\Users\user\T7\process-copy --max-ctx 12000 --session %E%\rc03-r2b.jsonl exec "说明这个项目的入口文件和主要模块分工。不要修改任何文件。" >> %A% 2>&1

echo RC03-DONE %date% %time% >> %A%
