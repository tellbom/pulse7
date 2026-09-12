# pulse7 — 长期方向裁决（Direction Decisions）

> 日期：2026-09-08｜基线：阶段二 B1–B5 完成，rc-0.9 待打
> 目的：把后续会长期生效、且**改起来很贵**的决策一次性定下来，避免边做边议
> 适用对象：GPT（后端实现）、GLM（前端实现）、后续任何接手方

---

## 0. 北极星与停止规则

**北极星**：让内网 IT 与技术岗**真的用它写代码**。不是功能对齐 Claude Code。

**停止规则**（任何人想加功能时先过这三条）：

1. **这个功能解决的问题，在真实使用中出现过吗？** 没出现过就先不做——写进 backlog，等它出现。
2. **不做它，用户会放弃使用吗？** 不会的话，优先级自动降到 P2 以下。
3. **它会让代码规模翻倍吗？** 会的话，必须先找到"最小可用版本"，做不出最小版本就不做。

**已用这三条否决、不再重开讨论的**：durable workflow 引擎（7 候选全否）、子 agent、hooks、MCP、todo list、任务规划、持久 shell。

---

## 决策一：对标 Claude Code 的正确姿势——抄"制品"，不抄"架构"

Claude Code 是为「快模型 + 200k 窗口 + 额外调用很便宜」调优的。pulse7 是「慢端点 + 单模型 + 每次往返都贵」。**同一个机制在两边的价值可能相反。**

### 值得逐字抄的（都是纯文本或常量，成本≈0）

| 制品 | 来源 | 为什么抄 |
|---|---|---|
| 九段摘要 Prompt | `CC:services/compact/prompt.ts:61-143` | 纯文本，直接译用。pulse7 现在只有一句话摘要 |
| `<analysis>` 草稿区 + 剥离 | `CC:prompt.ts:31-59, 311-335` | 先梳理再写摘要，草稿不进最终上下文。免费提升质量 |
| 微压缩哨兵替换 | `CC:microCompact.ts:36, 446-530` | `[Old tool result content cleared]` + 可压缩工具白名单 + 保留最近 N 个（**下限强制为 1**，注意 `slice(-0)` 陷阱） |
| 文件新鲜度校验 | `CC:tools/FileEditTool.ts:275-306` | 读后才可改。约 30 行 |
| Skills 的渐进披露结构 | 见决策二 | frontmatter 常驻、正文按需 |
| 事件类型命名 | `CC:cli/print.ts:848-928` | 前端协议事件名沿用，别自创 |
| stdout 守卫思路 | `CC:cli/print.ts:588-596` | 协议模式下人类可读输出全量改道 stderr |

### 明确不抄的（架构级，规模不匹配）

四层压缩体系（只取微压缩 + 完整压缩两层，cache_edits 依赖 Anthropic 特有 API）、memdir 全套子系统（见决策三）、会话树/分支/编辑重发（只加字段不做功能）、101 个 slash command（只做已定的少数几个）、Node BFF 中间层（本地 TCP 已放行，pulse7 直接起 HTTP + SSE）。

### token 计量：按「端点不回传 usage」设计

用户判断内网 DeepSeek **大概率不回传 `usage`**。仍应实测确认（发流式请求时带 `stream_options: {"include_usage": true}`，看末尾 chunk 有无 `usage`），但**实现必须按不回传的路径设计**：

1. **修好估算器**：当前 `contextChars` 不计 `ToolCalls.Function.Arguments`——20KB 工具参数被算成 39 字节（C-1）。必须计入全部消息字段。
2. **留安全边际**：估算不准，触发线要保守。压缩阈值从 75% 下调到 **65%**，宁可多压一次，不要撞窗口。
3. **最关键：把"撞窗口"做成可恢复错误，而不是任务死亡。**

第 3 条是没有 `usage` 时的核心补偿。识别端点返回的上下文超限错误（`context_length_exceeded` 等），**立即触发一次紧急压缩然后重试当前请求**，而不是 `EXEC-ERROR` 退出。

> 理由：既然测不准，就必然会有估错的时候。估错的代价必须是"多等一次压缩"，不能是"任务当场死掉、用户重头再来"——在慢网关上后者的代价极高。

---

## 决策二：Skills —— **优先级高于自动记忆，建议本轮就做**

### 判断

MCP 与子 agent 已否决，但 **Skills 不在被否之列，而且它对 pulse7 是高价值低成本项**。

Claude Code 的 Skills 机制：<cite index="10-1">会话开始时 Claude 只看到每个 skill 的名称与描述，不是全文；当它判断某个 skill 与当前任务相关时才加载完整 SKILL.md；skill 引用的其他文件（参考资料、脚本）只在真正需要时才加载</cite>。<cite index="8-1">启动时扫描可用 skills，把简短的名称与描述编成 YAML 元数据放进系统提示，大约 100 tokens</cite>。

### 为什么对 pulse7 特别合适

**1. 实现成本极低——几乎不需要新机制。**

```
启动时：扫描 skills 目录 → 解析各 SKILL.md 的 frontmatter（name + description）
      → 拼进 system 段（每条几十 token）
运行时：模型判断相关 → 用【已有的 read 工具】读取完整 SKILL.md
      → SKILL.md 里引用的其他文件，模型继续用 read 读
```

**不需要新工具、不需要新协议、不需要额外 LLM 调用。** 估计 50–80 行。

**2. 它精准命中你的实际场景。** 内网 IT 团队有大量重复流程：某个老项目怎么构建、发布要走哪几步、公司代码规范、某台服务器怎么连。**写一次 SKILL.md，全组共享**（提交进 git 仓库即可）。

**3. 它比自动记忆更可靠。** Skills 是人**有意编写**的，内容准确、可审阅、可版本管理；自动记忆是模型**推测提取**的，可能记错且用户难以察觉。对 IT 用户来说，"我自己写的流程文档 agent 会自动用上"比"agent 自己记了些什么我也不知道"更可接受。

**4. 它与 AGENT.md 分工清晰**：

| | 加载时机 | 放什么 |
|---|---|---|
| `AGENT.md` | 每次都加载 | 项目约定（语言版本、禁止事项）——**短** |
| Skills | 按需加载 | 具体流程（怎么部署、怎么调试某模块）——**可以长** |

### 最小实现规格

**目录**（两级，与配置分层一致）：

```
<workspace>\.pulse7\skills\<name>\SKILL.md   项目级，可提交进 git 团队共享
%USERPROFILE%\.pulse7\skills\<name>\SKILL.md  个人级
```

**SKILL.md frontmatter**：

```markdown
---
name: 发布流程
description: 本项目打包与发布的完整步骤。当用户提到发布、打包、出版本、release 时使用。
---
（正文：具体步骤，可以很长，也可以引用同目录下的其他 md 文件）
```

**注入格式**（system 段）：只放清单，不放正文：

```
可用技能（相关时用 read 工具读取完整内容）：
- 发布流程 (.pulse7/skills/release/SKILL.md)：本项目打包与发布的完整步骤。当用户提到发布、打包...
- 数据库迁移 (.pulse7/skills/db-migrate/SKILL.md)：...
```

**上限**（慢端点下必须有）：

- 最多 20 个 skill；
- 单条 description ≤ 200 字符（超出截断并告警）；
- 清单总长超过 2KB 时告警，提示用户精简。

**description 的质量决定一切**——写"帮助处理文档"这种模糊描述，模型永远不会在对的时候调用它。<cite index="6-1">描述里要包含用户实际会说的触发词，说明它需要什么输入、产出什么</cite>。这一点要写进用户文档并给示例。

**`/skills` 命令**：列出当前可用的 skill 清单。

---

## 决策三：跨会话自动记忆 —— 降级到"暂缓"，且改用文件 + ripgrep

### 排期变更

GPT 的评审把跨会话记忆（memdir）标为唯一 P0。**驳回该评级。**

理由：Skills（决策二）以极低成本覆盖了"把知识固化下来跨会话复用"这个核心诉求的绝大部分，且更可靠。**先做 Skills，自动记忆等真实使用反馈。**

### 若将来要做，方案定死如下（不用 SQLite）

```
项目记忆：<workspace>\.pulse7\memory\*.md
个人记忆：%USERPROFILE%\.pulse7\memory\*.md
召回：ripgrep 关键词 + mtime 排序，零 LLM 调用
```

**为什么不用 SQLite**：CGO=0 下只能用 `modernc.org/sqlite`，而 M2.5 调研已确认其**在 Win7 上无任何公开验证**；会破坏 `go.mod` 零 diff；数据量根本不需要数据库；**文件可 grep、可读、可改、可提交**，IT 同事看到内容不对能直接编辑修掉。

**为什么不用 LLM 召回**：你的内网慢，每轮多一次往返直接恶化体感；你只有单一模型，没有便宜的小模型；**而你已经随包分发了 ripgrep**，关键词召回毫秒级。GPT 自己把"关键词 + mtime 本地召回"列为降级方案——**在你的环境里它是正解，不是降级**。

---

## 决策四：现在必须定死的 schema（改起来最贵的三样）

在 GPT 写阶段三之前定稿。

### 4.1 会话记录 schema —— 只加字段，不做树形功能

```json
{
  "uuid": "本条消息唯一 id",
  "parentUuid": "上一条的 uuid，首条为 null",
  "timestamp": "ISO8601",
  "sessionId": "会话 id",
  "cwd": "工作区绝对路径",
  "version": "schema 版本号",
  "role": "...", "content": "...", "tool_calls": [...]
}
```

现在加是几行，以后回填是迁移工程。分支、编辑重发暂不做，字段先留着。

同时修：resume 时校验 `_meta.workspace`（P1-05）；单行超大导致 `loadSession` 静默返回 0 条且 `err==nil` 改为明确报错（P2-04）。

### 4.2 前端事件 schema —— 沿用 Claude Code 命名，不自创

```
session_init        sessionId / workspace / model / tools / skills 清单 / 上下文预算
assistant_delta     流式文本增量
tool_call           id / name / args
tool_result         id / ok / 摘要 / 完整结果引用
permission_request  工具 / 参数（严格档位下才出现）
permission_response 用户应答
context_state       usedTokens / budget / percentLeft / warningLevel
compaction          触发原因 / 前后 token / 摘要可展开
skill_loaded        本轮加载了哪个 skill
background_task     启动 / 有新输出 / 结束
heartbeat           已等待时长
turn_result         success / max_rounds / interrupted / need_answer / error
```

**`heartbeat` 与 `context_state` 是你环境的刚需**——前端必须能区分"在等"和"卡住"，用户必须看得到上下文余量。

**同一份 schema 走两个出口**：SSE 给 GUI，NDJSON（`--output-format stream-json`）给自动化。Schema 是资产，传输层很便宜。

### 4.3 配置 schema

```
全局：%USERPROFILE%\.pulse7\config.json   endpoint / model / api_key / 权限档位 / 超时
项目：<workspace>\.pulse7\config.json     覆盖全局，只放项目相关项
优先级：flag > 项目 > 全局 > 默认
```

**api_key 只允许出现在全局配置或环境变量，代码层面禁止写入项目配置**——否则会被提交进 git。

---

## 决策五：权限档位默认值

阶段二 B1 的权限引擎**已建成，不要拆**。只改默认值：

- **默认档位 = 放行**；
- **保留两条 deny 硬边界，任何档位不可覆盖**：① 工作区外的**写**操作（读放行）；② `.git` 目录直接写入；
- 自动放行的操作**必须记入审计并在事件流中可见**——"不用点 y"不等于"不告诉用户做了什么"。

**并且必须补文件新鲜度校验**：确认门取消后，这是唯一能挡住"agent 用旧内容覆盖用户刚改的文件"的机制，而 git checkpoint 挡不住（checkpoint 拍在用户改动之前）。约 30 行，**优先级高于本文件其余所有新功能**。

---

## 决策六：分发与多人使用

- **分发**：内网 MinIO 目录 + SHA256 清单 + `latest.txt`。**不用 npm**（目标机无 Node，且 npm 是公网）；
- **升级**：`doctor` 读 `latest.txt` 比对本地版本，只提示不自动更新；
- **团队共享**：`AGENT.md` 与 `.pulse7/skills/` 放在项目里，**提交进 git 全组共享**；个人配置、个人 skills、api_key 放 `%USERPROFILE%`，不进项目。

---

## 补充修复清单（小改动，随批次带上）

| ID | 项 | 说明 |
|---|---|---|
| C-4 | **压缩后重注入已读文件** | 压缩后模型忘了读过什么，会**从头再读一遍**。你的网络慢、read 不便宜，这是实打实的浪费。按最近 5 个文件 / token 预算重新注入（`CC:compact.ts:1415-1464`）。**改动中等，收益直接** |
| — | **采样参数** | 当前未设 temperature / top_p，走端点默认。写代码场景默认值常偏高导致输出不稳。设低温度，**一行** |
| M-4 | AGENT.md 按字节截断劈裂 UTF-8 | 8KB 那刀切在中文字符中间。改为按 rune 边界，几行 |
| C-10 | 短历史时既不压缩也不截断 | 逃逸口，顺手补 |
| C-9 | `maybeCompressContext` 重复调用 | 第二次必空转，删一行 |
| C-8 | 截断兜底产生孤儿 tool_result | 改为按 tool_call/tool_result 成对丢弃 |

---

## 给前端设计（Figma）的必备要素

以下三项是真实使用中的高频动作，cmd 里做不了、GUI 里很自然。**设计稿里没有，后面加就要返工。**

**1. `@` 引用文件。** 用户打 `@src/main.go` 直接把文件带进对话，不必描述"就是那个处理登录的文件"。需要输入框支持路径自动补全与文件浏览。

**2. 粘贴大段文本。** 用户最常做的事之一是把一坨报错日志/堆栈贴进来问"这什么意思"。要能贴、能自动折叠、作为附件而非把输入框撑爆。

**3. 首次运行引导。** 同事双击 exe，没有 config、没有 api_key、不知道选哪个目录——**现在会直接报错退出**。这是采纳率杀手。第一屏必须是配置向导：填端点 → 点「测试连接」→ 选工作区 → 开始。

**另外三项常驻区域**（不能埋在对话流里）：上下文余量（`context_state`）、后台任务列表、当前可用 skills。

---

## 排期（按此顺序，不要调换）

```
1. rc-0.9：完成阶段二验收 + 打 tag              ← 正在做，做完
2. 核对：两份评审清单 vs 当前 HEAD 逐条比对      ← 20 分钟，剔除已修项
3. 小批修复：文件新鲜度 + 权限默认档位
             + 补充修复清单全部 + schema 定稿
             + 上下文超限可恢复（决策一第 3 条）
4. Skills（决策二）                              ← 便宜且高价值，别往后排
5. 阶段三：事件 schema → HTTP/SSE → 占位页 → 前端契约文档
6. 前端：Figma 设计 → GLM 实现
7. 微压缩 + 九段摘要 Prompt + C-4（可与 5/6 并行）
8. 内网小范围试用                                ← 越早越好
9. 跨会话自动记忆（决策三）                      ← 试用之后，且未必要做
```

**第 8 步不要再往后推。** 到目前为止，pulse7 的每一个功能决策——包括本文件——都建立在推测之上，没有一个真实用户用过。GUI 做完就交出去，哪怕只有两个人、只用三天。

---

## 遗留：必须立即处理

`artifacts/claude-handoff.md` 中**明文存有 Win7 虚拟机与 VMware 宿主机的 SSH 凭据**，而该文件正在被传递给 GPT、GLM 与外部评审方。凭据轮换自 M3 挂起至今，**已从"待办"变成"正在扩散"**。

轮换 VM 密码、宿主机密码、API Key，改 SSH 公钥，清理文档中的明文。**这不是开发任务，是现在就该做的事。**
