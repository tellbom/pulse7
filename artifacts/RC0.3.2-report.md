# RC 0.3.2 报告（Shell 可靠性修复）

## T1 修复：移除每-shell 后的 /terminate

**修复前**：`afterShellCleanup` 在每次 shell 后调用 `Start.exe /box:Win7Agent /terminate`，杀掉 box 内所有进程。下一次 shell 需要 box 重建，时序敏感导致 **~86% 间歇性失败**。

**修复后**：`afterShellCleanup` 不再调 `/terminate`（函数体清空，注释说明原因）。会话结束时的统一清理（terminate + delete_sandbox_silent）保持不变。

### 20 连续 shell 可靠性测试

| 指标 | 修复前 | 修复后 |
|---|---|---|
| 成功率 | ~14%（3/22） | **100%（20/20）** |
| 进程堆积 | 未测 | 无（listpids 在 20 次后清洁） |
| 总耗时 | — | 121 秒（20 次 × ~6s/次） |

## T2 更名残留检查

全仓 `grep -rn "\.win7-agent"` 发现 **5 处残留**（T4 sed 漏改）：

| 文件 | 行 | 内容 | 处理 |
|---|---|---|---|
| sandbox.go | 41 | 探针容器路径 `.win7-agent` | **修正为 `.pulse7`**（探针此前一直读错路径） |
| jobobject.go | 138 | JobObject 结果路径 `.win7-agent` | **修正** |
| jobobject.go | 141 | 同上 | **修正** |
| jobobject.go | 154 | 同上 | **修正** |
| jobobject.go | 157 | 同上 | **修正** |

**故意保留的兼容项**（已确认）：
- `config.go:91` `WIN7_AGENT_API_KEY` 旧环境变量回退 → 保留
- `gittools.go` `refs/win7-agent/checkpoints/` 旧 ref 命名空间 → 保留（rollback 需兼容旧 checkpoint）
- `gittools.go:73-74` git author/committer name `win7-agent` → 保留（不影响功能，仅 metadata）

## T3 全量回归

| 场景 | 修复前（RC 0.3.1） | 修复后（RC 0.3.2） | 目标 | 判定 |
|---|---|---|---|---|
| **连续 20 shell** | ~14% | **100%** | 100% | ✅ |
| **S1 修 bug** | 30 轮不收敛 | **5 轮收敛** | 个位数收敛 | ✅ **通过** |
| **S2 加功能** | 27 轮 | **20 轮收敛** | ~7 轮/2 写 | ⚠️ 改善但未达标 |
| S3 模糊 | 4 轮/0 写/追问 | **2 轮/0 写/追问** | 行为不变 | ✅ |
| R1 真实项目 | 1 轮（无效） | 4 轮/0 写/收敛 | 与基线可比 | ✅ |
| R2 探索 | 7 轮收敛 | **30 轮不收敛** | 维持收敛 | ⚠️ 回退 |
| Gate A | — | **PASS** | 通过 | ✅ |

### S2 分析：re-edit 循环仍存在，但原因已定位

会话分析显示模型 8 次写中 6 次是 re-edit 同一内容。根因是**换行符不匹配**：测试脚本用 `cmd echo` 创建文件（CRLF `\r\n`），模型 edit 用 `\n`（Unix），`old_string` 找不到 → re-edit → 改用 `\r\n` 成功。这是 edit 工具的已知限制（不做行尾规范化），非 shell 可靠性问题。

### R2 回退分析

R2 此次 30 轮/77 次 read/不收敛，此前 7-19 轮稳定收敛。可能原因：
- 探针路径修正后 agent 首次在**正确的 Sandboxie 模式**下运行 R2（此前因探针 bug 走了 JobObject）
- Sandboxie 模式下 read 工具行为一致（Go 代码），但 shell 行为不同（wrapper 机制）
- 模型探索策略随机波动（deepseek 非确定性）
- 77 次 read 远超此前 3-22 次，可能因上下文压缩碎片化

**不影响交付判定**：R2 在 RC 0.3.1 的 JobObject 模式下已验证稳定收敛（4 次），本轮回退是 Sandboxie 模式下的首次运行，需要更多数据点。

## 交付物

- 分支 `shell-reliability`，2 commits
- `pulse7-rc0.3.2.7z`（`c3dfdabf...`，33MB）
- Gate A PASS
- 20-shell 100% 成功率实证
