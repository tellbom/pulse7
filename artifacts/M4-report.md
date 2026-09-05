# M4-Usability 执行报告（2026-09-05）

- 分支 `m4-usability`（自 rc-0.1 新建；m2-freeze / m3-complete / rc-0.1 三 tag 未移动）
- 约束核查：`go.mod`/`go.sum` 零 diff；M3 安装器产物零触碰（doctor 仅按 T3 明示追加 AGENT.md 报告行，报告性输出、探测/安装逻辑未动）；未做崩溃恢复器/todo/子agent/M4-B；凭据未动；VM 仅软重启（本轮零重启）。

## T0–T5 逐项

| 任务 | 内容 | 验收 | commit |
|---|---|---|---|
| T0 | 轮次上限 8→30；终答 pushMsg 入 session | session 尾部=纯文本终答；真机 S1/S2 自然收敛（5/7 轮） | 1st |
| T1 | Ctrl-C 受控停：杀子进程（JobObject 整树 / Sandboxie /terminate+杀宿主）、补齐未应答 tool_call、打印摘要、二次强退 130；LLM 等待可取消 | 四场景全过（原生 GenerateConsoleCtrlEvent 测试工具；MSYS kill 无法投递已记录） | 2nd |
| T2 | write/edit 返回带 diff：新建=行数+5 行预览；覆盖/edit=对齐式 diff（±2 行上下文、-old/+new） | 500 行覆盖→40 行封顶+`另有 462 行改动未显示`（session 内完整结果验证） | 3rd |
| T3 | AGENT.md 注入 system（8KB 截断告警）、doctor 显示、README 补节 | mock 验证 system 携带约定；真机 S4 遵守（.format() 非 f-string） | 4th |
| T4 | 75% 阈值上下文压缩（额外 LLM 摘要、保 system+近 3 轮、失败回退截断、`[上下文已压缩]` 提示）——唯一核心循环改动 | 触发+继续收敛 ✓；失败 mock 500 → 9 次回退、零 panic 干净收尾 ✓；期间揪出旧 truncateContext 每轮截史的真凶 | 5th |
| T5 | --list（时间/工作区/首条用户消息 60 字/条数，倒序 20 条）+ --resume <id|latest> | 三会话列表正确；按中间 id 与 latest 均正确恢复（latest 解析提前修复自指 bug） | 6th |
| 附 | WIN7_AGENT_API_KEY 环境变量作 api-key 兜底（flag>config>env；不落日志/git） | 真机验证通过 | 7th |

每项完成后本地 mini-suite 回归全绿；T4 后另跑全套（M3-SMOKE/M2-GIT/T2DIFF/T3AGMD）。

## 真模型验证（§2，端点 aigc789.top / deepseek-v4-flash，全部一次通过、未超 3 次预算）

| 场景 | 结果 | 指标（session 取证） |
|---|---|---|
| S1 回归 | ✅ 修复+验证（each pays: 30.0） | 5 轮 / 6 工具 / 终答 |
| S2 回归 | ✅ trim+接入（trim: hello） | 7 轮 / 9 工具 / 终答 |
| S4 AGENT.md | ✅ 约定被遵守（`"hello, {}".format(name)`） | 3 轮 / 3 工具 |
| S5 压缩场景 | ✅ 收敛（40 段总结写入 summary.txt） | 8 轮 / 9 工具；压缩触发与否不可事后判定（finding #2） |
| S3 首跑 | ✅ 存档完毕（**未追问，15 轮 25 工具直接重构项目**） | 评估留人工；快照 M4-e2e/S3-after/ |

失败样本：本轮无失败任务（唯一"失败形态"= finding #1 的 stdout 丢失，已二分定位并记录，未改核心绕过）。

## 存档

`artifacts/M4-e2e/`：m4-s1/s2/s4/s5/s3 完整 session、probe-mock/probe-real（stdout 判别实验）、S3-after/ 工作区快照、audit、s3-run.log、m4-e2e-run.log（顶层）。
