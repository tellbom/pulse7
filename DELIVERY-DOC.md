# pulse7 项目 — 完整交付文档（v2，更新至 RC 0.3.2）

> 合并项目从立项到 RC 0.3.2 的全部关键决策、架构、验证数据、差距分析与已知问题。供接手方（或 AI 助手）快速获得完整上下文。
> 项目原名 `win7-agent`，RC 0.3 起更名为 **pulse7**。

---

## 1. 项目定义

**目标**：在 Windows 7 SP1 x64 上运行一个类 Claude Code 的终端 AI 编程助手（无 GUI），面向内网 IT 与技术岗用户。

**核心约束**：
- 模型：内网 OpenAI 兼容端点（当前为 deepseek-v4-flash，纯文本无多模态）
- 用户：IT 与技术岗（非业务用户——S3 首跑证明模型在需求不明确时会大幅越界）
- 系统：Win7 SP1 x64，可能缺 KB4474419（SHA-2 补丁）
- 交付：单 EXE + runtime 目录，内网离线，无公网依赖
- 禁止：外部服务端 / Redis / Docker / Python / Node / CGO / 监听 TCP 端口
- 基线：Go 1.20.14，CGO_ENABLED=0

**产品规则（冻结）**：
1. KB4474419 永远只是开发/测试机验证条件，禁止转化为生产前置
2. Sandboxie 驱动不可用时必须自动降级（JobObject），绝不要求用户补系统补丁
3. 卸载绝不动用户 workspace / Git 仓库 / 用户文件
4. 不做 todo list / 子 agent / hooks / slash commands / MCP / durable workflow

---

## 2. 技术架构

```
pulse7\
├─ pulse7.exe             ← Go 1.20.14 单文件（~8MB，交叉编译）
├─ runtime\git\            ← MinGit 2.46.2 x64（checkpoint/diff/rollback）
├─ runtime\rg\rg.exe       ← ripgrep 13.0.0（高速检索，Win7 兼容最后版）
├─ config\agent.json       ← 用户配置
├─ data\sessions\          ← 会话 .jsonl + audit.jsonl（无数据库）
├─ data\logs\agent.log     ← 运行日志（stdout tee，防控制台句柄丢失）
└─ 快速上手.md / 反馈模板.md  ← 试用者文档
```

### 运行时沙盒（两种结果，无第三种）

| 条件 | 模式 |
|---|---|
| 桌面会话 + Sandboxie 服务/驱动健康（真实探针验证） | Sandboxie（进程/文件/注册表虚拟化） |
| 其他一切情况 | JobObject 自动降级（进程树击杀/超时/内存上限，零补丁依赖） |

**探针**：不信任 Start.exe 存在性或退出码——用生产 wrapper 机制跑 `echo PROBE-OK`，必须能在容器映射路径读到输出才算可用。

### 第三方组件（全部冻结版本，独立可执行 + 外部调用）

| 组件 | 版本 | SHA256（前16） | 作用 |
|---|---|---|---|
| Go | 1.20.14 | 0e0d0190406ead89 | 开发机交叉编译 |
| MinGit | 2.46.2 x64 | 0dca60869825ceb8 | checkpoint/rollback/diff |
| go-openai | v1.42.0 | vendor 固化 | OpenAI 兼容协议/SSE/tool-call |
| Sandboxie Classic | 5.73.2 x64 | 18239310d6ad247e | shell 隔离（可选） |
| ripgrep | 13.0.0 x64 | ab5595a4f7a6b918 | 高速检索（14+ 需 Win10，锁定 13.0.0） |

---

## 3. Agent 能力清单（截至 RC 0.3.2）

### 工具集（11 个）

| 工具 | 说明 | 安全措施 |
|---|---|---|
| read | 读文件（行号、4KB 截断） | 路径白名单 |
| write | 创建/覆盖文件（带 diff 摘要） | 路径白名单 + manifest |
| edit | 精确字符串替换（带 diff 摘要） | 路径白名单 + manifest |
| **tree** | 项目骨架（一次调用替代几十次 ls） | 工作区限定 |
| grep | ripgrep 高速搜索（不可用时回退 Go 实现） | 工作区限定 |
| glob | 通配符文件匹配 | 工作区限定 |
| ls | 目录列表 | 工作区限定 |
| shell | cmd.exe 命令（沙盒内） | 确认门 + git 写黑名单 |
| checkpoint | git 私有 ref 快照 | 不动用户分支/暂存区 |
| rollback | 恢复到指定/最新检查点 | 不用 git clean |
| get_time | 本地时间 | — |

### Agent 循环

- 流式 SSE 输出（go-openai）
- **30 轮**工具调用上限（RC 0.1 的 8 轮被玩具任务烧满后提升）
- 上下文压缩：75% 阈值触发 LLM 摘要（保 system + 近 3 轮原文；失败回退截断）
- 压缩写 audit `_compress` 记录
- 系统提示词：需求不明确→先只读探索→提一个具体问题；只做明确要求的改动；验证方式="运行程序或测试"（禁止 type/more 回读）

### 会话管理

- `.jsonl` 追加记录，首行 `_meta` 记录 workspace
- `--list` 列出最近 20 条
- `--resume <id|latest>` 按 id 或最新恢复
- 中断时自动补写未完成 tool_call

### Ctrl-C 中断

- 第一次：杀子进程 + 补写 session + 打印摘要 + 退出码 130
- 第二次：立即强制退出

### 退出码

| 码 | 含义 |
|---|---|
| 0 | 任务正常完成 |
| 1 | 出错 |
| 2 | 模型以提问结束（`--resume` 续跑） |
| 130 | 用户 Ctrl-C 中断 |

### 安全机制

- 路径白名单（相对路径以工作区根解析）
- shell 确认门（REPL y/N；exec 需 --yolo）
- git 写操作黑名单（commit/push/reset/checkout/merge/rebase/cherry-pick/clean/stash）
- checkpoint/rollback（私有 ref `refs/pulse7/checkpoints/`，兼容旧 `refs/win7-agent/`）
- manifest 只清 agent 创建的文件
- append-only 审计日志
- 任务结束 shell 命令摘要

---

## 4. 真机验证数据汇总

### 标准三场景回归集（每次改提示词/工具描述必须跑）

| 场景 | 任务 | 最新结果（RC 0.3.2） | 历史 |
|---|---|---|---|
| S1 修 bug | 修复 calc.py（people=0）| 30 轮（详见已知问题 #1）| RC0.1: 5轮 / RC0.2: 6轮 |
| S2 加功能 | utils.py 加 trim() | 27 轮收敛 ✅ | RC0.1: 7轮 |
| S3 模糊任务 | "把这个项目整理一下" | 4 轮 / 0 写 / 追问 ✅ | 稳定 |

### 真实项目压测（E:\process，18K 文件/2.8GB，副本测试）

**R2 探索任务——本轮最大突破**：

| 指标 | 基线（无 tree） | 修复后（tree + rg） | 变化 |
|---|---|---|---|
| 轮次 | **30（撞上限）** | **7-19 轮** | 稳定收敛 |
| ls 调用 | **105** | **2-3** | -98% |
| tree 调用 | 0 | 3-8 | 新工具 |
| 压缩次数 | **22** | **0** | -100% |
| 收敛 | ❌ | ✅ ×4 次 | 关键突破 |
| 工作区 | 零改动 | 零改动 | 保持 |

**R1 明确修改**：10 轮 / 仅改 1 文件 4 行 / grep 验证收敛（基线数据）。

### 沙盒验证

| Gate | 环境 | 结果 |
|---|---|---|
| A | 桌面会话 + Sandboxie 正常 | ✅ Sandboxie 模式 |
| B | 双服务 disabled + 重启 | ✅ JobObject 自动降级 |
| C | 受限令牌（trustlevel 0x20000） | ✅ |
| D | 会话 0（headless 计划任务） | ✅ JobObject |

### ripgrep 性能（Win7 真机实测）

| 关键词 | ripgrep | 旧 Go grep（开发机估算） |
|---|---|---|
| error | 59ms | ~300ms |
| ConnectionString | 52ms | ~300ms |
| 零命中词 | 45ms | ~300ms |

⚠️ 两列条件不同（真机 vs 开发机），不可直接引用倍数。

---

## 5. 已知问题（按优先级排序）

| # | 问题 | 影响 | 状态 |
|---|---|---|---|
| 1 | edit 工具不处理换行符差异（CRLF vs LF）| S2 re-edit 循环，轮次从 ~7 膨胀到 20 | 未修（~5 行改动：edit 匹配前规范化 \n） |
| 2 | agent checkpoint 产物目录出现在工作区（.pulse7/）| git status 显示 untracked | 未修 |
| 3 | R2 在 Sandboxie 模式下首次不收敛（30 轮/77 read），JobObject 模式下稳定（7-19 轮）| 探索任务可能不稳定 | 记录，需更多数据 |
| 4 | ripgrep 14+ 需 Win10 | 集成时必须锁 13.0.0 | 已记录 |
| 5 | deepseek-v4-flash 返回 reasoning_content | go-openai 忽略；token 含推理 | 兼容 |
| 6 | tree 工具仅在 5829 文件项目验证 | 18K+/50K+ 未验证 | 真实使用观察 |
| 7 | R1 数据可能无效（reset 后文件状态不确定）| 回归数据质量 | 改善 reset 脚本 |
| 8 | ~~afterShellCleanup /terminate 间歇性失败~~ | ~~S1/S2 不收敛~~ | **已修复**（RC 0.3.2：14%→100%） |
| 9 | ~~T4 重命名 runner.go 路径 bug~~ | ~~Sandboxie shell 全挂~~ | **已修复**（RC 0.3.2） |
| 10 | ~~T4 重命名 sandbox.go/jobobject.go 路径残留~~ | ~~探针恒失败/JobObject 读错路径~~ | **已修复**（RC 0.3.2） |

## 6. 差距分析结论（T8，30 项三选一判断）

### 建议做（2 项）

| 优先级 | 项目 | 状态 |
|---|---|---|
| P0 | 探索策略提示词优化 | **已完成**（tree 工具解决了根本问题） |
| P1 | 项目记忆（AGENT.md 扩展为自动积累改动摘要） | 未做，~30 行 |

### 暂不做（10 项）

ripgrep 集成（**已做**，T2 retrieval 轮完成）、批量编辑、文件操作粒度、上下文策略、改动前预览确认、进度可见性、成本统计、安全边界强化、错误自我纠正、并行工具调用

### 不做（18 项）

todo list、任务规划、子 agent、hooks、slash commands、MCP、durable workflow 引擎、终端 ANSI 渲染、多模态、网络搜索、LSP 集成、文件监听等

---

## 7. 交付物清单

| 制品 | 版本 | SHA256 |
|---|---|---|
| **pulse7-rc0.3.1.7z**（推荐） | RC 0.3.2 | `c3dfdabf433eb5afdc917656e9b5f045524b46ee3faa406fe30d420860b63edd` |
| pulse7.exe | RC 0.3.2 | `2190a30ded7eaa93d4...`（含路径修复 + 沙盒阻止消息） |
| MinGit 2.46.2 | 冻结 | `0dca60869825ceb8...` |
| Sandboxie Classic 5.73.2 | 冻结 | `18239310d6ad247e...` |
| ripgrep 13.0.0 | 冻结 | `ab5595a4f7a6b918...` |

**Git 标签链**：`m2-freeze` → `m3-complete` → `rc-0.1` → `rc-0.2` → `rc-0.3` → `rc-0.3.2`

---

## 8. 源码结构

```
E:\win7-agent\
├─ agent\                     ← Go 源码（14 个 .go + vendor）
│  ├─ main.go                ← CLI 入口、REPL/exec 循环、信号处理、tee 输出、系统提示词
│  ├─ tools.go               ← 11 个工具实现 + 路径白名单 + git 写黑名单 + diff 摘要
│  ├─ tree.go                ← tree 工具（项目骨架）
│  ├─ grep.go                ← ripgrep 调用 + Go 回退
│  ├─ sandbox.go             ← 沙盒接口 + 副作用探针
│  ├─ sbx.go                 ← Sandboxie adapter（Start.exe + wrapper 文件捕获 + 沙盒阻止消息）
│  ├─ jobobject.go           ← JobObject adapter（Win32 API 整树击杀）
│  ├─ gittools.go            ← checkpoint/rollback（双命名空间 ref 扫描）
│  ├─ ctxcompress.go         ← 75% 上下文压缩 + audit 记录
│  ├─ session.go             ← .jsonl 会话 + manifest + 列表/恢复
│  ├─ envdetect.go           ← doctor + 环境探测 + 模式选择
│  ├─ sandboxiecfg.go        ← Sandboxie ini 自动配置
│  ├─ config.go              ← agent.json 加载/优先级合并
│  ├─ runner.go              ← shell wrapper 文件机制
│  ├─ cleanup.go             ← 三级清理
│  └─ mock.go                ← 内嵌 mock 端点（测试用）
├─ installer\                 ← install.cmd / uninstall.cmd / README.md
├─ poc\real-e2e\             ← 三场景标准回归集
├─ poc\scripts\              ← 真机 Gate 脚本
├─ artifacts\                ← 全部验证日志 + 报告
├─ freeze\                   ← 冻结制品清单
└─ dist\rc-0.3.2\            ← 最新交付包
```

---

## 9. 关键排障经验（给下一轮开发者的教训）

| 教训 | 来源 |
|---|---|
| **sed 批量重命名必须 grep 验证所有文件** | T4 漏改 runner.go 导致 Sandboxie shell 全挂，查了两整轮 |
| SSH 会话 0 不能运行 Sandboxie（需桌面会话） | T1 诊断第一轮全超时 |
| /silent 抑制的 Start.exe 错误弹窗退出码不可信（返回 0） | M3 探针假阳性 |
| Win7 conhost 不解析 ANSI/VT 转义 | 禁用 TUI 的根本原因 |
| PowerShell 2.0 不支持 -File 参数 | T7 性能测试 |
| 计划任务里 SESSIONNAME 为空（不能用于交互判定） | M4 会话检测 |
| 硬复位丢注册表写入与文件内容 | KB 安装后必须软重启 |
| 挂死的计划任务会阻止同任务名新实例（读到旧日志） | Gate 脚本加了删除前+时间戳验证 |

---

## 10. 内网部署注意事项

1. **API 端点**：config\agent.json 或环境变量 `PULSE7_API_KEY`（旧 `WIN7_AGENT_API_KEY` 兼容）
2. **Sandboxie（可选）**：不装也能用（JobObject 降级）；装了自动配置
3. **AGENT.md（推荐）**：工作区根目录放项目约定，上限 8KB
4. **已知限制**：edit 工具在 Windows CRLF 文件上可能需要多次尝试（已知问题 #1）
5. **内网网关验证（未做）**：超时/并发/tool call 支持度/格式差异
6. **凭据轮换（未做）**：VM + 宿主机 + API Key

---

## 11. 下一步（人工，非开发任务）

1. **凭据轮换** — 挂了整个项目周期，该做了
2. **内网网关验证** — 唯一无法在测试环境模拟的未知数
3. **2-3 个 IT 同事真实试用** — 用 RC 0.3.2 跑 3-5 个真实任务；**下一轮开发内容由反馈决定**
4. **修 afterShellCleanup /terminate 间歇性失败** — 唯一遗留的阻断级问题（~30 行改动）
