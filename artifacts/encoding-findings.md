# encoding-pagination 超范围发现（未修，仅记录）

分支：encoding-pagination（基线 rc-0.4）｜日期：2026-09-06

## F-1 read/grep 遇到非 UTF-8 编码的源文件仍乱码（文件编码 ≠ 控制台编码）

T2 修的是 **shell 输出**（控制台 ANSI 代码页）。但工作区里的**文件本身**可能是 GBK 编码
（如 process-copy 项目部分 C# 文件含 GBK 中文注释）：`read` 原样读字节进上下文，
session 里变 U+FFFD（Sandboxie R2 日志中 3 处实证，位于 `// ═══` 注释与 tree 结果引用附近）。
修法需在 read/grep 对非 UTF-8 内容走与 decodeShellOutput 相同的兜底（utf8.Valid 检查 +
CP_ACP 解码），属独立改动，本轮未做。**同理 write/edit 写回时的编码保持也未处理。**

## F-2 Sandboxie 模式第 13 轮无声退出（一次）

R2-Sandboxie run3：第 13 轮完成后进程直接消失——无 EXEC-DONE / EXEC-ERROR / 中断提示，
批处理随即继续（RUN3-DONE 打出）。与 RC0.3.3 记录的 Sandboxie 模式 1/3 收敛不稳定、
本轮 T1 排查时偶发的 `start.exe 0x40010004` 同族。本轮 Sandboxie 2/3 收敛（8/8 轮）已优于
基线，未深挖根因。建议专项：加 exitcode 与崩溃现场记录（批处理层 echo %errorlevel%）。

## F-3 reset-project.cmd 在 Sandboxie 序列中报 "fatal: detected dubious ownership"

git safe.directory 校验偶发（R2-SBX 日志 START 行后 4 个 U+FFFD 附近），导致一次 reset
可能未生效。属测试设施脚本问题，不影响产品；修法 `git config --global --add safe.directory *`。

## F-4 Windows sshd 给远端命令追加尾引号，粘在最后一个参数上

现象：`plink "cmd /c x.cmd a b c"` → 批处理里 `%3` 收到 `c"`。历史上 snrun 系列把引号
吞进 `--session` 值（生成了带引号文件名的 session，无害但脏）。本轮场景脚本改为
**每个场景一个硬编码路径的 cmd** 规避。测试台架注意事项，非产品问题。

## F-5 TEST-08 重跑轮次偏多（31 条 shell）

主要消耗在找 Go 工具链（PATH 未含 C:\uat\go\bin，agent 遍历磁盘搜索）。与编码无关；
提示：给中文项目场景的启动器预设 PATH 可省 20+ 轮。台架层面解决即可。
