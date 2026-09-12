# M3：Win7-Agent 安装器与运行环境产品化任务书

> 前置：M2 已冻结（git tag `m2-freeze`）。本任务**不修改** M2 Agent 核心（Git 工具 / 文件工具 / session / LLM loop）。
> 目标：证明"普通 Win7 用户能稳定**安装、启动、降级、清理、卸载**"，而不是继续证明"能不能跑"（已证明完成）。

## 产品规则（冻结，不可违反）

1. **KB4474419 永远只是开发/测试机的验证条件**：安装器不得检测、提示或要求安装任何系统补丁；缺 SHA-2 能力（驱动 577）不构成阻断，仅影响沙盒模式选择。
2. **运行结果只有两种**：`Sandboxie` 或 `JobObject(auto-degraded)`；禁止出现第三种"半可用 Sandboxie"状态。
3. **判断以真实运行探针为准**（`Start.exe /silent /box:<box> /listpids`，8s 超时）——"Start.exe 存在 ≠ 可用"（已实测：SbieSvc 停而驱动在会拖垮进程创建）。
4. **卸载绝不动用户数据**：workspace、用户 Git 仓库、用户文件一律保留。

## 六个交付项

### A 环境探测（`win7-agent.exe doctor`）
启动/doctor 输出：Windows 版本与位数、会话类型（Console/RDP/Services）、MinGit runtime 存在性与版本、工作区路径、Start.exe 存在性、SbieSvc/SbieDrv 状态、真实探针结果、最终模式（两种之一）、config 目录可写性。

### B Sandboxie 配置自动化
从测试脚本转产品逻辑：工作区变化 → 生成/更新 `[Win7Agent]` + `OpenFilePath=<ws>\*` → `Start.exe /reload` → 再启动 shell。幂等、可重复；不依赖用户手工编辑 Sandboxie.ini。

### C 自动清理
每次 shell：wrapper 临时目录用后即删（已有）+ 每次后 `/box /terminate`（专用 box，无副作用）；会话结束：`/terminate` + `delete_sandbox_silent`（可配置）；启动时清理 >1h 的陈旧 wrapper 目录。不触碰用户真实工作区。

### D 启动方式
默认桌面会话：Sandboxie 或 JobObject 均可；无交互桌面（会话 0）：直接 JobObject（Sandboxie 不可靠不硬撑）。仍只输出两种模式。

### E 配置文件 `config\agent.json`
仅暴露用户级配置：`base_url / api_key / model / workspace / sandbox_preference(auto|sandboxie|jobobject) / shell_timeout_sec / memory_limit_mb / max_ctx / yolo / cleanup_on_exit`。优先级：CLI flag > config > 默认。不暴露 Sandboxie 内部细节。
目录：`win7-agent\{win7-agent.exe, runtime\git, config\agent.json, data\{sessions,audit,temp}, logs}`。

### F 安装 / 卸载（install.cmd / uninstall.cmd）
安装（可提权一次）：落盘产品目录 → （可选）静默装 Sandboxie（`/S`）→ 驱动可用性检查（失败不阻断、不提补丁，走 JobObject）→ 自动生成 box 配置与 agent.json → 完成。
卸载：停进程 → 清 Win7Agent box（内容+ini 节）→ 清 wrapper/临时 → 删程序目录（`/full` 才卸 Sandboxie 本体）→ **保留用户数据**。
生命周期必须真机验证：干净安装→启动→执行→重启→再运行→卸载。

## 真机 Gate（四组，每组只跑：启动→read→write→shell→checkpoint→rollback→退出）

| Gate | 环境 | 期望 |
|---|---|---|
| M3-A | Sandboxie 服务正常（AUTO/DEMAND, RUNNING） | banner=**Sandboxie**，mini-suite 全过 |
| M3-B | 双服务 disabled + 重启（=未打补丁机等价态） | banner=**JobObject(auto-degraded)**，mini-suite 全过，零补丁提示 |
| M3-C | 普通用户令牌（trustlevel 0x2000 桌面会话） | Agent 正常（read-only 探测不要求提权） |
| M3-D | 会话 0 凭据计划任务（headless） | banner=**JobObject**（Sandboxie 不硬撑），mini-suite 全过 |

mini-suite 由内嵌 mock 的 `M3-SMOKE` 触发器驱动：read → write → checkpoint → shell → rollback → final。

## 穿插项（不立里程碑）

- 真实 LLM E2E：拿到 API Key 后跑一次完整证据链即可。
- 凭据轮换（用户执行）：VM `user`、宿主机 `Administrator` 两套 + 改公钥；之后日志不得再携带旧凭据。

## 明确不做

- M4 / B 路线（除非 Go1.20、沙盒产品化或架构出现根本性障碍）
- Agent 新功能、核心重构、大规模测试体系
