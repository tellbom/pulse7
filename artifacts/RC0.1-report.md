# RC 0.1 打包报告（2026-09-05）

- 分支：`rc-0.1-packaging`（master 已合并 `m3.1-hardening`，ff 至 0a32e21 后新建）
- **解读声明**：任务书"硬禁止"段写明不使用真实 API Key / 不执行 S1-S3，但其后第 5 条给出了真实端点、模型与 API Key 并指名"真模型 E2E（S1 / S2）"。按"后出且更具体指令优先"执行：**S1/S2 已用该端点真跑，S3（模糊任务）按第 5 条范围留人工**。Key 未写入任何 git 提交文件，VM 上的临时脚本已删除。若此解读有误，S1/S2 可随时重跑复现。

## 1. 合并与回归

- master ← m3.1-hardening 快进合并；构建 `win7-agent.exe` SHA256 `483fad6f8adc90c222f761b069d1e001735bcb0d24c77e2a44d696a147608b37`。
- 本地 mini-suite：✅ 5/5 工具结果、git 守卫 3/3 拦截。
- **真机抽跑（按任务书只跑 A、B 两组）**：
  - **Gate A**（服务正常，桌面会话）：banner `Sandboxie`，read/write/checkpoint/shell/rollback 全链路 PASS，`checkpoints available: 11`（T2 跨会话 ref 扫描真机生效）。日志 `artifacts/rc01-gate-a.log`。
  - **Gate B**（双服务 disabled + 软重启 = 未打补丁等价态）：banner `JobObject (auto-degraded ... no system patch required)`，全链路 PASS；时间戳加固生效（START 14:20:19 → DONE 14:20:40）。日志 `artifacts/rc01-gate-b.log`。测后服务与会话已恢复（RUNNING + console 自动登录）。

## 2. RC 0.1 交付包

```
dist/rc-0.1/win7-agent-rc0.1.7z   SHA256: c123e9394e3a9c722d31b9e377c6e108c2330fd9902e34162d643c63a75042a7 (33.0 MB)
内含（349 文件，逐文件 SHA256 见包内 SHA256SUMS.txt）：
  win7-agent.exe        483fad6f...  config\agent.json（占位符模板）
  runtime\git\          MinGit 2.46.2 x64（冻结版）
  install.cmd / uninstall.cmd / README.md（用户向中文：安装/配置/使用/排障/卸载/安全说明）
```

## 3. 真模型测试脚手架（poc\real-e2e\）

- `agent.template.json`：endpoint/model 已填（`https://aigc789.top/v1` + `deepseek-v4-flash`），api_key 留占位；真实 Key 走 `agent.local.json`（已 .gitignore）或运行时 flag。
- 三个场景（**S3 未执行，留人工**）：
  - `S1/`：calc.py（people=0 触发 ValueError）+ TASK.md（修复并验证，预期 5-8 轮）
  - `S2/`：utils.py/main.py + TASK.md（新增 trim 并接入，预期 4-6 轮）
  - `S3/`：notes.txt/old_tmp.py + 故意模糊的 TASK.md（观察追问 vs 硬猜）
- `summarize.sh`：跑完汇总轮次/调用分布/首末跨度/总耗时/token 估算/收敛判定（本轮已修复进程替换路径兼容问题）。

## 4. 真模型 E2E 结果（S1 / S2，真机 Win7 + Sandboxie 模式 + deepseek-v4-flash）

| 场景 | 结果 | 关键验证 | 耗时 | 指标（artifacts/rc01-e2e/summary.md） |
|---|---|---|---|---|
| S1 修复报错脚本 | ✅ 成功 | `people = 4`，`python calc.py` → `each pays: 30.0` | 22s | 8 轮 / 9 调用（read×4, shell×3, edit×1, ls×1）/ 跨度 17s / ctx≈915 tok |
| S2 加小功能 | ✅ 成功 | utils.py 新增 `trim`，main.py 接入，输出 `trim: hello` | 25s | 8 轮 / 11 调用（read×4, shell×4, edit×2, ls×1）/ 跨度 22s / ctx≈1213 tok |

行为观察（对后续提示词调优有用的真实样本）：
- 模型会主动 `ls`+多次 `read` 探索后再动手，编辑一次到位，均自行运行 python 验证后给终答——符合预期的"探索→修改→验证→收敛"模式；
- 一次越界 `read`（读工作区外的运行日志）被路径白名单正确拒绝，模型随即改用工作区内路径——安全边界在真实模型下工作正常；
- 任务结束摘要（T4）在真模型场景正常打印 shell 清单。

## 5. 本轮发现（未修，遵守"不改核心"禁令）

| # | 发现 | 建议 |
|---|---|---|
| 1 | 会话文件不记录最终 assistant 终答（streamTurn 直接 return，未 pushMsg）→ summarize 的收敛判定恒为 NO（本次以 EXEC-DONE+产物验证兜底） | 下一轮加一行 pushMsg（行为无影响，仅补记录） |
| 2 | E2E 运行脚本里 `%%errorlevel%%` 经 heredoc 转义写入为字面量，退出码未捕获 | 脚本模板改用 `%errorlevel%` 单百分号 |
| 3 | deepseek-v4-flash 返回 `reasoning_content` 附加字段，go-openai 忽略之（取 content）——兼容，但 token 消耗含推理部分，成本估算需留意 | 记录在案 |
| 4 | API Key 已出现在对话与（已删除的）临时脚本中 | 建议与 VM/宿主机凭据一并轮换 |

## 6. 人工待办

- S3 模糊任务执行与评估；凭据轮换（VM + 宿主机 + 本 API Key + 公钥化）；小规模内网试用对象确定。
