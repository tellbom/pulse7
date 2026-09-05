# Win7 CLI Agent 项目 — 完整交付文档

> 本文档合并了项目从立项到 RC 0.2 的全部关键决策、架构设计、验证数据与差距分析，供接手方（或 AI 助手）快速获得完整上下文。

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

---

## 2. 技术架构（A 路线，已验证）

```
win7-agent\
├─ win7-agent.exe        ← Go 1.20.14 单文件（8MB，交叉编译）
├─ runtime\git\           ← MinGit 2.46.2 x64（冻结版，checkpoint/diff/rollback）
├─ config\agent.json      ← 用户配置（endpoint/model/workspace/sandbox偏好/超时）
├─ data\sessions\         ← 会话 .jsonl + audit.jsonl（无数据库）
├─ data\logs\agent.log    ← 运行日志（stdout tee，防控制台句柄丢失）
└─ data\workspaces\       ← 非 git 工作区的私有 checkpoint 仓库
```

**运行时沙盒（两种结果，无第三种）**：

| 条件 | 模式 |
|---|---|
| 桌面会话 + Sandboxie 服务/驱动健康（真实探针验证） | Sandboxie（进程/文件/注册表虚拟化） |
| 其他一切情况（驱动不可用/无桌面会话/服务停止） | JobObject 自动降级（进程树击杀/超时/内存上限，零补丁依赖） |

**探针**：不信任 Start.exe 存在性或退出码——用生产 wrapper 机制跑 `echo PROBE-OK`，必须能在容器映射路径读到输出才算可用。

**第三方组件（全部冻结版本，独立可执行文件 + 外部调用模式）**：

| 组件 | 版本 | SHA256（前16） | 作用 |
|---|---|---|---|
| Go 工具链 | 1.20.14 | 0e0d0190406ead89 | 开发机交叉编译 |
| MinGit | 2.46.2 x64 | 0dca60869825ceb8 | checkpoint/rollback/diff |
| go-openai | v1.42.0 | vendor 固化 | OpenAI 兼容协议/SSE/tool-call |
| Sandboxie Classic | 5.73.2 x64 | 18239310d6ad247e | shell 隔离（可选安装） |
| （备选）ripgrep | 13.0.0 | — | 高速检索（已验证 Win7 可用，未集成） |

---

## 3. Agent 能力清单（截至 RC 0.2）

### 工具集（10 个）

| 工具 | 说明 | 安全措施 |
|---|---|---|
| read | 读文件（行号、4KB 截断） | 路径白名单 |
| write | 创建/覆盖文件（带 diff 摘要） | 路径白名单 + manifest |
| edit | 精确字符串替换（带 diff 摘要） | 路径白名单 + manifest |
| grep | Go 正则全库搜索 | 工作区限定 |
| glob | 通配符文件匹配 | 工作区限定 |
| ls | 目录列表 | 工作区限定 |
| shell | cmd.exe 命令（沙盒内） | 确认门（y/N）+ git 写操作黑名单 |
| checkpoint | git 私有 ref 快照（不动用户分支/暂存区） | — |
| rollback | 恢复到指定/最新检查点 + manifest 精确清理 | 不用 git clean |
| get_time | 本地时间 | — |

### Agent 循环

- 流式 SSE 输出（go-openai CreateChatCompletionStream）
- 30 轮工具调用上限（M4-T0 从 8 提到 30，RC0.1 玩具任务都烧满 8 轮）
- 上下文压缩：75% 阈值触发 LLM 摘要（保 system + 近 3 轮原文；失败回退截断）
- 压缩写 audit `_compress` 记录（消息数/token 估算）
- 系统提示词：任务不明确→先只读探索→提一个具体问题等回答；只做明确要求的改动

### 会话管理

- `.jsonl` 追加记录（无数据库），首行 `_meta` 记录 workspace
- `--list` 列出最近 20 条（时间/工作区/首条消息/消息数）
- `--resume <id|latest>` 按 id 或最新恢复
- 中断时自动补写未完成 tool_call 的合成结果

### 中断（Ctrl-C）

- 第一次：杀子进程（Sandboxie /terminate 或 JobObject 整树）+ 补写 session + 打印摘要 + 退出码 130
- 第二次：立即强制退出

### 退出码

| 码 | 含义 |
|---|---|
| 0 | 任务正常完成 |
| 1 | 出错 |
| 2 | 模型以提问结束（`--resume` 续跑） |
| 130 | 用户 Ctrl-C 中断 |

### 安全机制

- 路径白名单（相对路径以工作区根解析，跨盘符拒绝）
- shell 确认门（REPL 交互 y/N；exec 需 --yolo）
- git 写操作黑名单（commit/push/reset/checkout/merge/rebase/cherry-pick/clean/stash）
- checkpoint/rollback（私有 ref，用户 git log 零污染）
- manifest 只清 agent 创建的文件（禁 git clean -fdx）
- append-only 审计日志（audit.jsonl）
- 任务结束 shell 命令摘要打印

---

## 4. 真机验证数据汇总

### 标准三场景回归集（每次改提示词/工具描述必须跑）

| 场景 | 任务 | 轮次 | 工具 | 写操作 | 是否提问 | 收敛 |
|---|---|---|---|---|---|---|
| S1 修 bug | 修复 calc.py（people=0）| 6 | 7 | 1 | 否 | ✅ |
| S2 加功能 | utils.py 加 trim() | 13 | 21 | 8 | 否 | ✅ |
| S3 模糊任务 | "把这个项目整理一下" | 2 | 3 | 0 | **是** | ✅（以问题收尾） |

### 真实项目压测（E:\process，18K 文件/2.8GB，副本测试）

| | R1 明确修改 | R2 探索 |
|---|---|---|
| 轮次 | 10 | **30（撞上限）** |
| 工具 | read 3 + grep 2 + edit 4 + ckpt 1 | **ls 105** + read 26 + glob 10 + shell 2 |
| 压缩 | 0 | **22 次** |
| 耗时 | 103 秒 | ~340 秒 |
| 改动 | **仅 1 文件 4 行** ✅ | **零** ✅ |
| 收敛 | ✅ | ❌ |

**R2 失败分析**：模型不知道何时停止遍历目录（105 次 ls），导致上下文反复压缩碎片化，最终无法综合出答案。**问题不在检索速度，在探索策略。**

### 沙盒验证

| Gate | 环境 | 结果 |
|---|---|---|
| A | 桌面会话 + Sandboxie 正常 | ✅ Sandboxie 模式 |
| B | 双服务 disabled + 重启 | ✅ JobObject 自动降级 |
| C | 受限令牌（trustlevel 0x20000） | ✅ Sandboxie 正常 |
| D | 会话 0（headless 计划任务） | ✅ JobObject 正常 |

### 压缩验证（S5 大文件场景，真机）

5 次 `_compress` audit 记录，含前后消息数与 token 估算：
```
messages_compressed: 3→5→4→3→3, est_tokens_after: 5097→4095→4159→382→1380
```

---

## 5. 已知问题与限制

| # | 问题 | 影响 | 状态 |
|---|---|---|---|
| 1 | 探索任务撞 30 轮（105 次 ls + 22 次压缩死循环） | 探索类任务无法收敛 | 未修——需提示词优化 |
| 2 | exec 模式下 agent stdout 在 /it + 真端点 + 工具轮次时丢失 | 控制台不可见（已用 agent.log tee 解决） | 已缓解 |
| 3 | agent checkpoint 产物目录 `.claude/` 出现在工作区 | git status 显示 untracked | 未修——可移到产品目录 |
| 4 | ripgrep 14+ 需 Win10，Win7 需锁 13.0.0 | 如集成需冻结版本 | 已记录 |
| 5 | deepseek-v4-flash 返回 `reasoning_content` 附加字段 | go-openai 忽略之；token 消耗含推理部分 | 兼容，已记录 |
| 6 | Win7 桌面堆耗尽（大量进程后用户态失去响应） | 长期运行需注意进程清理 | 已实现三级清理 |

---

## 6. 差距分析结论（T8，30 项三选一判断）

### 建议做（2 项）

| 优先级 | 项目 | 理由 | 工作量 |
|---|---|---|---|
| **P0** | 探索策略提示词优化 | R2 失败直接原因；零代码 | 改 1 行提示词 |
| **P1** | 项目记忆（AGENT.md 扩展） | R2 失败间接原因（无积累）；IT 用户反复在同一项目工作 | ~30 行 |

### 暂不做（10 项，需真实使用反馈或项目增长）

ripgrep 集成（当前够用）、批量编辑、文件操作粒度、上下文策略、改动前预览确认、进度可见性、成本统计、安全边界强化、错误自我纠正、并行工具调用

### 不做（18 项，与用户画像/约束不符或已明确否决）

todo list、任务规划、子 agent、hooks、slash commands、MCP、durable workflow 引擎、终端 ANSI 渲染、多模态、网络搜索、LSP 集成、文件监听、等

### 最终三件事（如只能再做三件就交付试用）

1. **探索策略提示词**——在系统提示词加"先看顶层结构，选 3-5 个关键文件深入，不要逐目录遍历"
2. **项目记忆**——任务完成时自动在 AGENT.md 追加改动摘要，下次打开直接看到历史
3. **真实用户试用**——2-3 个 IT 同志在真实内网跑 3-5 个真实任务

---

## 7. 交付物清单

| 制品 | 位置 | SHA256 |
|---|---|---|
| RC 0.2 包 | `dist/rc-0.2/win7-agent-rc0.2.7z` | `70f8137b52992fb413d35ce8f7cd020104a13c29978fad172899005680ddd50b` |
| win7-agent.exe | 包内 | `31c7a0a9fb905a726f218ec0415f29ffba07a7d8a8fdc39f88f0c589bf568b03` |
| MinGit 2.46.2 | 包内 runtime\git | `0dca60869825ceb8b6108be69f0c536174fbca45e11300f2c14c34632d8238ed` |
| Sandboxie 安装包 | 内网制品库 | `18239310d6ad247e1a0d56afe0d58af961e67375dd3bc0c160e028d23cc282b6` |
| ripgrep 13.0.0（备选） | `https://github.com/BurntSushi/ripgrep/releases/download/13.0.0/ripgrep-13.0.0-x86_64-pc-windows-msvc.zip` | — |

**Git 标签**：`m2-freeze` → `m3-complete` → `rc-0.1` → `rc-0.2`

---

## 8. 源码结构

```
E:\win7-agent\
├─ agent\                     ← Go 源码（8 个 .go + vendor）
│  ├─ main.go                ← CLI 入口、REPL/exec 循环、信号处理、tee 输出
│  ├─ tools.go               ← 10 个工具实现 + 路径白名单 + git 写黑名单
│  ├─ sandbox.go             ← 沙盒接口 + 探针（副作用验证）
│  ├─ sbx.go                 ← Sandboxie adapter（Start.exe + wrapper 文件捕获）
│  ├─ jobobject.go           ← JobObject adapter（Win32 API 整树击杀）
│  ├─ gittools.go            ← checkpoint/rollback（私有 ref + for-each-ref 跨会话发现）
│  ├─ ctxcompress.go         ← 75% 上下文压缩 + audit 记录
│  ├─ session.go             ← .jsonl 会话 + manifest + 列表/恢复
│  ├─ envdetect.go           ← doctor 子命令 + 环境探测 + 模式选择
│  ├─ sandboxiecfg.go        ← Sandboxie ini 自动配置（SbieIni.exe 非管理员可写）
│  ├─ config.go              ← agent.json 加载/模板/优先级合并
│  ├─ runner.go              ← shell wrapper 文件机制（inner.cmd + run.bat）
│  ├─ cleanup.go             ← 三级清理（每 shell/会话结束/陈旧 wrapper）
│  └─ mock.go                ← 内嵌 OpenAI 兼容 mock 端点（测试用）
├─ installer\                 ← install.cmd / uninstall.cmd / README.md
├─ poc\real-e2e\             ← 三场景标准回归集（reset + runner + metrics + README）
├─ poc\scripts\              ← 真机 Gate 脚本 + E2E 脚本（含时间戳加固）
├─ artifacts\                ← 全部验证日志 + 报告 + 冻结清单
├─ freeze\M2\ / freeze\M3\   ← 冻结制品清单
├─ dist\rc-0.2\              ← RC 0.2 交付包
└─ tools\                    ← Go 1.20.14 + 安装包下载缓存
```

---

## 9. 内网部署注意事项

1. **API 端点**：config\agent.json 填 `base_url` / `api_key` / `model`；或用环境变量 `WIN7_AGENT_API_KEY`
2. **Sandboxie（可选）**：不装也能用（JobObject 降级模式）；装了则自动配置沙盒
3. **AGENT.md（推荐）**：工作区根目录放项目约定文件（Python 版本/构建命令/禁止事项），上限 8KB
4. **内网网关验证（未做）**：超时行为、并发限制、tool call 支持程度、返回格式差异——这是目前唯一无法在测试环境模拟的未知数
5. **凭据轮换（未做）**：VM 密码、宿主机密码、API Key 均需轮换
