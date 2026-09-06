# slow-network 超范围发现（不顺手修，仅记录）

分支：slow-network（基线 rc-0.3.3）｜日期：2026-09-06

## F-1 R2 场景当日波动（非本分支回归，A/B 已证实）

rc-0.4 下 R2（说明项目入口与模块分工，max-ctx 12000）三次均未干净收敛：
1 次第 20 轮以"需要用户回答"收尾，2 次触顶 30 轮。全程**零**网络类错误、零看门狗触发、零重试。

A/B 对照（同日同端点 aigc789/deepseek-v4-flash）：rc-0.3.3 旧 exe 跑同一场景直接
`EXEC-ERROR: context deadline exceeded`（5 分钟预算耗尽，压缩调用同报 deadline）。
结论：当日端点明显变慢，旧版猝死、新版能跑满轮次；R2 不收敛属模型轮次管理问题
（RC0.3.3 报告已记录 R2 在 Sandboxie 下 1/3 收敛的同类不稳定）。

根因线索（供后续专项）：模型用 powershell 逐段循环读取 600 行 Program.cs——
read 工具无 offset/lines 分页（正式验收报告 P2），大文件只能整读，模型转而用
shell 自行分页，轮次暴涨。修 read 分页预计能显著改善 R2。

## F-2 `maybeCompressContext` 被连续调用两次

main.go streamTurn 每轮开头连调两次 `maybeCompressContext(ctx, client, cfg, msgs)`。
第二次在第一次压缩后必然因未达阈值直接返回，无行为错误，但属冗余调用。
本轮"不改上下文压缩阈值"约束下未动。若未来清理，删一行即可。

## F-3 llm-max-retries=0 时的错误文案

`--llm-max-retries 0` 且首字节超时时，错误信息为"重试 0 次后仍失败"，措辞略绕
（未重试却报"重试 0 次"）。语义正确，不影响判断，留待文案优化。

## F-4 SSH 会话下 JobObject 嵌套警告依旧

`[warn] job assignment denied (nested job); reduced containment`——SSH 会话本身已在
job 内导致。与 rc-0.3.3 行为一致，非本分支引入。

## F-5 mock 的 FAIL500ONCE 计数器是进程级

同/mock 进程内多次 exec 会共享"第一次 500"状态，连续测试时需重启 mock 或知道
该语义。仅测试设施，不影响产品。
