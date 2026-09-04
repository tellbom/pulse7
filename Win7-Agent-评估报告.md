# Win7 版 CLI Agent 工具评估报告（实施基线）

| 项目 | 内容 |
|---|---|
| 文档状态 | **v3.2 —— 实施基线（M0.5/M1/M2 全部真机验证完成，含降级实测）** |
| 日期 | 2026-09-05 |
| 测试机 | Win7 专业版 SP1 x64 虚拟机（宿主机 192.168.124.3:22 VMware Workstation 17.6.3，VM 经 NAT 端口转发 2222；VM 状态已恢复，SbieSvc/SbieDrv RUNNING） |
| 方向 | **A 路线**：Go 1.20.14 编排层 + 冻结复用第三方组件，Agent 本体运行于 Win7 本机 |
| 生产硬约束 | **① KB4474419 仅为开发/测试验证条件，禁止转化为生产安装前置；② 生产环境 Sandboxie 驱动不可用时必须自动降级（Job Object），绝不要求用户补系统补丁** —— 已在代码层实现（启动探测 + 横幅声明 + 降级运行），见 §10 |

### 变更记录

| 版本 | 变更 |
|---|---|
| v1.0–v2.1 | 评估与两轮评审收敛（详见历史） |
| v3.0 | M0.5 五 Gate 真机执行全 PASS + 冻结清单；M1 最短链路真机 E2E PASS |
| v3.1 | M2 完成：文件工具、git checkpoint/rollback、会话/恢复、wrapper 修复、沙盒自动降级实现 |
| **v3.2** | **降级实测真机 PASS**（零 Sandboxie 启动状态下 JobObject 自动接管，会话 0 亦可用）；测试机资源故障经宿主机 vmrun 硬复位解决；新增运维发现：SbieSvc 停止而驱动仍加载会拖垮全系统进程创建（§12.3）；宿主机访问信息更新 |

---

## 1. 执行摘要

1. **M0.5 全 Gate PASS**（A MinGit / B Go / C go-openai / D TLS / E Sandboxie，E 带已吸收约束），第三方底座冻结：Go 1.20.14、MinGit 2.46.2 x64、go-openai v1.42.0（vendor 固化）、Sandboxie Classic 5.73.2 x64。
2. **M1 最短链路真机 PASS**：问题→LLM→tool call→本地执行→回传→终答，全程流式+审计。
3. **M2 真机全量 PASS**（§11）：write/edit/grep/glob/ls 五文件工具、git checkpoint→write→rollback 序列（manifest 精确清理，用户文件零误删）、会话 resume（21 条消息恢复）、shell 命令自带重定向修复实证（`REDIRECT-FIX` 落盘真实工作区）。
4. **生产降级已实现**：启动时探测 Start.exe 与服务/驱动健康（`/silent /listpids` 探针，8 秒超时），不可用即自动切 Job Object（进程树击杀/超时/内存上限，运行期免管理员），横幅明示 "no system patch required"。本地（无 Sandboxie 环境）降级运行实证；Win7 停服务降级实测被测试机资源故障阻断（§12.3），复测脚本就绪。

## 2–7. 架构基线（v3.0 内容不变，摘要）

- 生态：grok-1=权重无 Agent；grok-build 不可移植（Rust≥1.78/TUI/不收 PR）；模型一律远程 API（x.ai / 内网 OpenAI 兼容网关）。
- Win7 运行时矩阵唯一选择 Go 1.20.14；TLS 实证（Schannel TLS1.0 被拒 vs Go TLS1.3）；驱动签名实证（无 KB4474419 → 错误 577）。
- 产品目录：`win7-agent\{win7-agent.exe, runtime\git, data\sessions}`，拷贝即用；开发机交叉编译（`GOOS=windows GOARCH=amd64 CGO_ENABLED=0`）。
- Git 策略（最终裁决版）：用户仓库=私有 ref `refs/win7-agent/checkpoints/<task>/<seq>` + 临时索引（GIT_INDEX_FILE，不动用户暂存区）+ 回滚 `git restore --source`（不移动分支）；非 git 工作区=独立 `checkpoint.git`（--git-dir/--work-tree）+ `reset --hard`；manifest 只清 Agent 创建物，禁 `git clean -fdx`。
- 沙盒约束（全部被 adapter 吸收）：仅交互会话可用；非管理员令牌可用；退出码奇数→0x40010004（wrapper 文件回传）；stdout 文件捕获（容器 `user\current\` / `drive\C\` 双布局）；`OpenFilePath`+`/reload` 工作区直写；`/terminate`、`delete_sandbox_silent` 验证通过。
- 应用级防护：路径白名单（相对路径以工作区根解析）、shell 确认门（REPL y/N；exec 需 --yolo）、append-only 审计 jsonl。

## 8. M0.5 执行记录（真机，全 PASS）

见 v3.0 与 `artifacts/M0.5-freeze-manifest.md`（版本/SHA256/路径/排障全记录）。关键排障：Plus IFW 会话 0 静默失败→Classic NSIS `/S`；wusa ssh E5→expand+DISM CAB；错误 577→KB4474419+重启（**仅开发机**）；Start.exe 会话限制→`/it` 交互任务；奇偶退出码与容器布局→wrapper 文件捕获。

## 9. M1 结果（真机 PASS）

read/get_time/shell(Sandboxie) 三工具链路 + 流式终答 + 审计（`artifacts/m1-smoke-win7.log`）。

## 10. 生产部署约束与降级矩阵（本轮裁决落实）

**裁决原文**：KB4474419 仅属开发/测试环境验证条件，不得转化为生产 Win7 安装前置；生产 Sandboxie 驱动不可用时必须自动降级，不得要求用户补补丁。

**实现**（agent/sandbox.go + jobobject.go）：
```
启动探测: Start.exe 存在? → /silent /box:<box> /listpids (8s 超时) 探针
  通过  → sandbox: Sandboxie (box=Win7Agent)
  失败  → sandbox: JobObject (auto-degraded: <原因>) - no system patch required
JobObject: CreateJobObject + KILL_ON_JOB_CLOSE + JOB_MEMORY(2GB) + 超时 TerminateJobObject（整树击杀）
          运行期零提权、零驱动、零补丁依赖
```
- 降级后安全边界：路径白名单 + 确认门 + 审计 + git 回滚仍全量生效（无文件系统隔离，仅进程收容）。
- 本地实证：开发机（无 Sandboxie）自动降级横幅 + JobObject 模式 shell 执行成功。
- **真机降级实测 PASS（v3.2，`artifacts/m2-degrade-win7.log`）**：将 SbieSvc/SbieDrv 双双 disabled 并重启 VM，得到与"未打补丁生产机"完全等价的零 Sandboxie 状态（服务 STOPPED、无进程、驱动未加载）→ agent 启动横幅 `sandbox: JobObject (auto-degraded: Sandboxie service/driver unavailable ...) - no system patch required` → 全链路 read/get_time/shell 正常（`exitcode=0 [sandbox=JobObject]`，重定向文件真实落盘）。**且本次在会话 0（无桌面、凭据计划任务）跑通——JobObject 模式连交互会话都不依赖，服务态部署亦可行**。实测后服务配置已恢复（SbieSvc=AUTO/RUNNING，SbieDrv=DEMAND/RUNNING）。

**离线交付**：全部组件内网制品库自带（exe+vendor 固化 / MinGit / Sandboxie 安装包+SHA256 / 内网 LLM 网关），无公网依赖。

## 11. M2 结果（真机全量 PASS，`artifacts/m2-smoke-win7-r3.log`）

| 场景 | 结果 |
|---|---|
| RUN1 M1 回归 + 重定向修复 | ✅ read/get_time/shell 全过；`echo REDIRECT-FIX > ws\redirect-test.txt` **真实落盘** + `exitcode=0 [sandbox=Sandboxie]`（M1 已知边界关闭，wrapper inner.cmd 方案生效） |
| RUN2 M2-FILES | ✅ write(created) / edit(replaced 1 occurrence) / grep(`m2\note2.txt:1: M2-LINE-1-EDITED`) / glob / ls(`- note2.txt (16 B)`) |
| RUN3 M2-GIT | ✅ checkpoint `t0904-201756-783/1 (private-checkpoint-repo)` → write m2extra.txt → rollback `worktree restored; manifest cleanup removed 1 agent-created files`；**M2GIT-OK-extra-removed**，note.txt 完好 |
| RUN4 resume | ✅ `resumed 21 messages` 上下文载入 |
| 审计 | ✅ 全部工具调用带 taskID 落 audit.jsonl |

过程中修复三个真 bug（均已回归验证）：工具参数结构体缺 json tag（`old_string` 匹配失败→edit 误报 ambiguous）；相对路径按进程 CWD 解析（ls/grep 越界拒绝）→ 改为工作区根解析；taskID 秒级碰撞（相邻 run 共用 manifest）→ 加毫秒后缀。

M2 交付物：`win7-agent.exe`（SHA256 `4a3616eb04b78180d16de764223c5550c379de8bcd5ead375a51889f527c1492`），代码 `E:\win7-agent\agent\`（main/tools/runner/sbx/sandbox/jobobject/gittools/session/mock，vendor 固化）。

## 12. 风险与事件

### 12.1 风险清单（增量）

| # | 风险 | 等级 | 缓解 |
|---|---|---|---|
| 1 | 生产机器缺 SHA-2 → Sandboxie 不可用 | 高 | §10 自动降级（已实现，生产不可要求补丁） |
| 2 | **Win7 desktop heap / 进程资源耗尽**（长会话大量进程后用户态服务失去响应，§12.3 实证） | 中 | Agent 运行纪律：wrapper 目录即用即删（已做）；box 定期 `delete_sandbox_silent`（M3 产品化）；避免进程常驻堆积；必要时定期重启 |
| 3 | Sandboxie GPL-3.0 | 中 | 独立进程调用；分发前许可评审 |
| 4 | 测试机凭据已在对话中出现 | 中 | 轮换密码+公钥（待执行） |

### 12.2 遗留待办

- 真实 API Key 的 M1/M2 真模型 E2E
- 桌面 REPL 人工验收（交互会话内）
- 自动登录恢复（硬复位后未自动登录，见 §12.3；不影响 JobObject 模式，影响 Sandboxie 模式的交互会话可用性）

### 12.3 测试机事件与运维发现（已解决）

**事件（09-04 晚）**：高密度进程操作后 Win7 用户态失去响应（SSH 握手即断、SMB 67），判定 desktop heap 类资源耗尽。**解决（09-05）**：经宿主机（192.168.124.3:22，Windows 11 + VMware Workstation 17.6.3，用户 `Administrator`）`vmrun reset hard` 复位 `Windows_7_x64_SAS.vmx`（ASCII 路径 `E:\VM_os\w7qtp\Windows7_x64_QTP\`；注意 vmrun 需精确匹配清单中的真实中文路径，junction 别名无效）。重启后一切自动恢复。

**运维发现（重要）**：
1. **SbieSvc 停止而 SbieDrv 仍加载时，会拖垮全系统新进程创建**（SSH 新会话立即失败，服务自动恢复后自愈）——模拟"驱动不可用"必须双 disabled + 重启，绝不能只停服务；生产环境因驱动压根未加载，无此问题。该现象同时意味着：Sandboxie 安装但服务死亡的机器上，agent 的 8 秒探针超时降级是正确且必要的行为。
2. 硬复位后自动登录未发生（quser 无会话）→ `/it` 交互任务不可用；降级测试改用**会话 0 凭据任务**（`/ru user /rp *** /rl HIGHEST`）完成——顺带证明 JobObject 模式无桌面依赖。
3. vmrun 路径身份为精确字符串：清单中文路径 ≠ junction 短路径。
4. 宿主机凭据同样已出现在对话中，与 VM 密码一并轮换。

## 13. 里程碑状态

| 阶段 | 状态 |
|---|---|
| M0 / M0.5（Gate A–E + 冻结） | ✅ 全 PASS |
| M1 最小 Agent Loop（真机 E2E） | ✅ |
| M2 文件工具 + git checkpoint/rollback + 会话/恢复 + headless(exec) + wrapper 修复 + **自动降级** | ✅ **全部完成（含零 Sandboxie 真机降级实测 PASS）** |
| M3 安装器（探测/降级矩阵产品化、box 定期清理、/reload 流程） | ⬜ |
| M4（可选）B 路线对比 | ⬜ |

## 14. 参考资料与仓库结构

同 v3.0（grok-1/grok-build/Go/GitFW/go-openai/Sandboxie/KB 直链来源 S2E 清单）。

```
E:\win7-agent\
├─ agent\     M2 源码（8 个 .go + vendor）      ├─ artifacts\  全部真机日志 + 冻结清单 + ini
├─ dist\win7-agent\  产品形态（exe + runtime\git）├─ poc\        Gate 脚本与验证代码
└─ tools\     Go 1.20.14 / 安装包（含 SHA256）
```
