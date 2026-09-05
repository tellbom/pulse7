# Sandbox-Exec 修复报告

## T1 根因（详见 sandbox-exec-rootcause.md）

**一行代码 bug**：T4 重命名时 `sed` 漏改 `runner.go`，wrapper 写入 `.win7-agent\` 而读取 `.pulse7\`，路径不匹配导致 Sandboxie 模式下**所有** shell 返回 exitcode=-1。

## T3 修复（方案：直接修路径，非 A/B/C 三选一）

T1 结论显示环境本身无问题（python 在沙盒内完全可运行，Go 复刻测试证实），因此 A（Sandboxie 例外）、B（走 JobObject）、C（仅提示）均不需要。修复 = runner.go 两行 `.win7-agent` → `.pulse7`。

## T2 沙盒阻止消息

当 wrapper 无输出且无退出码时，返回明确消息告知模型"这是环境限制，重试同一命令不会成功"。（此消息在修复后应极少触发，但保留作为防御层。）

## T4 回归结果

| 场景 | RC 0.3（修复前） | 本轮（修复后） | 判定 |
|---|---|---|---|
| S1 修 bug | 27 轮，无 python 输出 | 30 轮，**python 输出出现**（each pays: 30.0 ×3）| ⚠️ 路径修复生效，但仍有循环 |
| S2 加功能 | 30 轮，无 python 输出 | 27 轮，**python 输出出现**（hello）| ✅ 收敛 |
| S3 模糊 | 2 轮 / 0 写 / 追问 | 4 轮 / 0 写 / 追问 | ✅ 保持 |
| R1 真实项目 | 6 轮（无效数据） | 1 轮 / 1 读 / 收敛 | ✅（读文件后直接回答） |
| R2 探索 | 19/15/18 轮 | 7 轮 / 0 写 / 收敛 | ✅ 最佳 |

## 核心判据判定

**S1 未通过**（30 轮不收敛）。但根因已修复——python 输出在沙盒内正常出现（日志中 `each pays: 30.0` 出现 3 次）。不收敛的原因是**另一个 bug**：`afterShellCleanup`（M3-C 引入的每次 shell 后 `/terminate`）导致下一次 shell 调用间歇性失败，模型在"成功→失败→重试→成功→失败"中循环。

**S2 通过**（27 轮收敛，python 输出正确）。轮次偏高的原因同上。

## 发现：afterShellCleanup 导致 shell 间歇性失败

**现象**：S1 模型运行 `python calc.py` 22 次，但日志中只出现 3 次 `each pays: 30.0`。约 86% 的 shell 调用返回空/失败。

**根因假设**：每次 shell 结束后 `afterShellCleanup` 调用 `Start.exe /box:Win7Agent /terminate` 杀掉 box 内所有进程。下一次 shell 调用时 box 需要重新初始化，部分调用因时序问题失败。

**证据**：
- Go 复刻测试（单次调用，无 /terminate）100% 成功
- Agent 连续调用（每次后 /terminate）~14% 成功率
- 模型连续重试同一命令有时成功有时失败（非确定性 = 时序问题）

**修复方向**（本轮不做）：移除或防抖 `afterShellCleanup` 中的 `/terminate`；或改为会话结束时统一清理（原 M3-C 设计意图）而非每次 shell 后。

## 交付物

- 分支 `sandbox-exec-fix`，3 commits（rootcause + fix + sandbox-blocked message）
- go.mod 零 diff，所有旧 tag 未动
- `artifacts/sandbox-exec-rootcause.md`：根因报告
- `artifacts/sandbox-exec-e2e/`：全部对话存档
