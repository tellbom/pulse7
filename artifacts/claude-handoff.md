# pulse7（win7-agent）交接文档 — 给 Claude

> 写于 2026-09-06。目标：让一个新的 AI 会话零上下文接手本项目。
> 仓库：`E:\win7-agent`（Win11 开发机，git bash）。产品：pulse7.exe，Win7 可用的 CLI 编程 Agent（Go 1.20，零第三方运行时依赖，vendor 完整）。

## 1. 项目是什么

- 单二进制 CLI Agent：`exec "任务"` 单发 / REPL / `mock` 内置测试端点 / `doctor` 自检 / `--list` / `--resume <id>`。
- 工具集：read/write/edit/grep/glob/ls/tree/shell/get_time/checkpoint/rollback（`agent/tools.go`）。
- 沙盒：Sandboxie 优先，无桌面会话/未装时自动降级 JobObject（`sandbox.go`/`jobobject.go`），产品原则是**永不要求用户打补丁**。
- Git 安全：checkpoint 存私有 ref（`refs/pulse7/checkpoints/...`），rollback 只按 manifest 清 Agent 自建文件，绝不用 `git clean`（`gittools.go`/`session.go`）。
- 会话：append-only JSONL（`data/sessions/sess-*.jsonl`），审计 `audit.jsonl`，日志 `data/logs/agent.log`（最可靠）。
- 主循环：`agent/main.go` `streamTurn()`（30 轮上限）；上下文压缩 `agent/ctxcompress.go`（75% 阈值摘要，失败回退截断）；网络韧性 `agent/netresilience.go`。

## 2. 当前 git 状态

- 分支 `slow-network`（自 tag `rc-0.3.3` 切出），HEAD = `853f1eb`，已打 **tag `rc-0.4`**，工作区干净。
- 历史 tag：m2-freeze / m3-complete / rc-0.1 ~ rc-0.4（**任何情况下不要移动已有 tag**）。
- 其他分支：`main`、`final-fix`（rc-0.3.3 时代）。
- rc-0.4 的 8 个 commit：T1 看门狗 `f099746`、mock 触发器 `3127d5c`、T2 重试 `2a09ad0`、T3 压缩独立预算 `0da0e98`、T4 心跳 `140cd2a`、T5 现场/指引 `a3eb6cd`、config 模板 `60eabae`、报告+打包 `853f1eb`。

## 3. 最近两轮工作（背景）

### 3.1 正式用户验收（UAT，2026-09-06 凌晨）
- 10 项 E2E 全在真 Win7 VM 上跑：**CONDITIONAL PASS，无 P0**。报告：`E:\agent-uat\Win7-CLI-Agent-正式验收报告.md`。
- 结果：TEST-01~05、07、08 PASS；TEST-06 FAIL（预埋包级编译错误未发现）。
- 遗留 P1（**均未修**）：①LLM 无重试/超时不可配（→ 已被 rc-0.4 修复）；②TEST-03 中 Agent 擅自把数据文件 `data/tasks.json` 改为 `tasks.json` 致旧数据不可见；③只用单文件 `go run` 验证、不做包级 `go build ./...`（TEST-06 失败根因）；④目标文件缺失时自造输入数据宣称完成；⑤多文件组织失控致包级不可构建。
- 遗留 P2：**read 工具无 offset/lines 分页**（schema 只有 path，模型传了被忽略 → 长文件反复整读、轮次暴涨，也是 R2 场景不稳的根因）、30 轮上限易触顶、排序语义反转等，详见报告第五节。

### 3.2 slow-network 分支（rc-0.4，2026-09-06 上午）
- 动机：内网模型慢且不可改善，Agent 不能被自己的超时掐死。
- 核心改动：**去掉整轮 5 分钟 ctx**，改为每请求双看门狗（首字节 300s/空闲 120s，`llm_first_chunk_timeout_sec`/`llm_idle_timeout_sec`）+ 退避重试（5s/15s，`llm_max_retries`，默认 2）+ 压缩独立 180s（`llm_compress_timeout_sec`）+ 心跳/每轮耗时输出（无 ANSI，纯行追加）+ session 惰性落盘与 resume 复用 + EXEC-ERROR 打印可照抄续跑命令。
- 验收：**10 分钟慢流（30s/chunk×20）完整跑完**；mini-suite 5/5、shell20 20/20、S1/S2/S3/R1/Gate A/B 全过；R2 三次未收敛但 A/B 证明非本分支回归（同日旧 exe 直接死于 deadline）。
- 报告：`artifacts/slow-network-report.md`；超范围发现（未修）：`artifacts/slow-network-findings.md`。

## 4. 建议的下一步（按优先级）

1. **read 工具加分页**（offset/lines 进 schema 与实现）——一举缓解 R2 不稳、轮次浪费、触顶，多个报告反复指向它。
2. 验证策略默认含包级 `go build ./...`（改系统提示词或在 shell 验证后追加）——解 UAT P1-3/P1-5。
3. 需求变更时保持既有 CLI/数据契约（可写进 baseSystemPrompt 或文档约定）——解 UAT P1-2。
4. 目标文件缺失应报告而非编造（baseSystemPrompt 补一条边界）——解 UAT P1-4。
5. findings F-2：`streamTurn` 每轮连调两次 `maybeCompressContext`，第二次必空转，删一行即可。
6. 人工事项（不要替用户做）：凭据轮换；内网网关实测（两个关键数据：单轮响应耗时、**是否支持 tool call**——不支持的话整个架构跑不起来）；2-3 位 IT 同事真实试用。

## 5. 构建与测试速查

```bash
# 构建（与发布一致）
cd agent && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o pulse7.exe .
# 约束惯例：go.mod/go.sum 必须零 diff；不引第三方库；vendor 里已有 go-openai
go vet ./...

# 本地 mock 测试（端点 127.0.0.1:8080/v1，默认 base-url 即它）
./pulse7.exe mock 300 &                     # 起内置端点
./pulse7.exe --workspace <ws> --yolo exec "M1-SMOKE round"   # 触发词驱动
# mini-suite 五件套：M1-SMOKE / M2-FILES / M2-GIT / M3-SMOKE / T4-NORM（M1/M2-FILES 用绝对路径 C:\Users\user\ws，只在 VM 上有意义）
# 慢端点触发器（rc-0.4 新增）：SLOWDRIP delay=30s count=20 / SLOWQUEUE delay=45s / SLOWSTALL / SLOWMID / FAIL500ONCE / FAIL401ALWAYS
# 验收口径：30s×20 drip 必须跑完不超时；SLOWMID（空闲超时）不得重试；401 不得重试

# 真模型回归（在 VM 上，历史脚本在 poc/scripts/：ff-e2e.cmd、shell20.cmd、gate-a.cmd、gate-bcd.cmd）
# 端点 https://aigc789.top/v1 + deepseek-v4-flash（key 在脚本里）；R1/R2 前先跑 reset-project.cmd
# rc-0.4 轮用的编码安全启动器：C:\Users\user\win7-agent\snrun.cmd / snrun-r2.cmd（--max-ctx 12000 版）
```

## 6. Win7 目标机环境（已搭好，勿重复搭建）

| 项 | 值 |
|---|---|
| 目标机 SSH | `ssh -p 2222 user@192.168.124.3`（密码 123456，Win7 专业版 SP1 x64）——**敏感，凭据轮换待办** |
| 宿主机 | `ssh Administrator@192.168.124.3`（密码 sk85-xxjsz）**仅用于 VM 崩溃恢复**（vmrun，`E:\VM_os\...\Windows_7_x64_SAS.vmx`），平时不要碰 |
| 本机无 sshpass | 用 `E:\agent-uat\tools\plink.exe`/`pscp.exe`，hostkey 固定 `SHA256:oGqmL3Pp0YEzZE2onCLvkpWVd5Z289tEtbgby4fxvMM` |
| VM 上的 Agent 安装 | `C:\Users\user\win7-agent\pulse7.exe`（现为 rc-0.4，旧版备份 `pulse7-rc033.exe.bak`）；UAT 专用部署在 `C:\uat\pulse7` |
| VM 上的 Go | `C:\uat\go`（go1.20.14 amd64，Win7 最后支持版本） |
| UAT 测试目录 | `C:\uat\01-minitask`、`02-calculator`、`04-nongit`、`05-crash`、`05b`、`06-health`、`07-cleanup`、`08-中文项目` |
| 回归夹具 | `C:\Users\user\real-e2e\S1/S2/S3`、`C:\Users\user\T7\process-copy`（R1/R2，git 仓，`reset-project.cmd` 重置） |
| 关键坑 | VM 根证书缺 Certum Trusted Network CA → Go TLS 到 bigmodel 卡死；已 `certutil -addstore root` 修过一次。若重装系统需再来一遍 |

## 7. 编码陷阱（本机+VM 都踩过，务必照做）

1. **中文经 cmd/plink 会乱码**：cmd 按 GBK 读脚本。给 pulse7 传中文 prompt 的可靠模式 = prompt 存 UTF-8 文件 + PowerShell 读取后启动（模板：`C:\Users\user\win7-agent\snrun.cmd`，本地版 `E:\agent-uat\vm\runagent2.cmd`）。
2. **PowerShell 2.0（Win7）把无 BOM 的 UTF-8 .ps1 当 ANSI 读**：ps1 里的中文字面量必坏。ps1 只写 ASCII，中文用 `[char]0xXXXX` 拼或 base64 内嵌。
3. **bash printf 会吃 `\u` `\t` 等转义**：写含反斜杠/中文的文件用 python 脚本（`open(...,'wb').write(...)`）或编辑器直写，别用 printf/heredoc 内联。
4. **zip 经 Shell COM 解压会把 UTF-8 文件名变 mojibake**：中文名文件用 base64 内嵌的 ps1 在目标机生成；整目录传输用 `pscp -r`。
5. VM 上中文目录操作：先 `powershell -File list2.ps1`（`E:\agent-uat\vm\list2.ps1` 模式）拿真实 Unicode 名再引用。
6. `pulse7` 每次 exec（含 --resume）都会建新 session 文件的历史行为在 rc-0.4 已改为 resume 复用原文件；EXEC-ERROR 会留空文件的问题也已修（惰性落盘）。

## 7.5 台架纪律：台架不得比目标环境更宽容

rc-0.5 的教训：中文 shell 输出乱码是**必现**问题，但历次回归全被测试台架的 `[Console]::OutputEncoding=UTF8` 掩盖，纯 cmd 用户必踩——测试环境比目标环境更宽容时，必现 bug 可以十几轮回归都不暴露。

原则：
- 任何为便于测试而做的环境调整（设置编码、追加 PATH、安装证书、预置工具链、schtasks 交互会话等）都必须记录在案，并注明「真实用户机器上是否具备」；
- 不具备的调整，必须另有一条**不依赖该调整**的验证路径（本轮起 S1 等场景在纯净 cmd 路径下跑：无 OutputEncoding、无 PATH 预置、无编码调整）；
- 场景脚本在 pulse7 调用后必须 `echo EXITCODE=%errorlevel%` 并检查 `=== EXIT ... ===` 终结记录，无记录即判 FAIL（rc-0.6 起产品保证任何退出路径都留记录）。

## 8. 行为红线（历次任务沿用，建议保持）

- 不新增第三方 Go 依赖（go.mod 零 diff）；不改 30 轮上限与压缩阈值，除非任务明确要求。
- 不移动任何已有 tag；每个子任务独立 commit，提交信息写清动机+验证。
- 测试失败如实记录，不许"修到通过"；超范围问题记 `artifacts/*-findings.md`，不顺手修。
- 对 VM：不硬复位（只软重启）；宿主机只在 VM 崩溃时用于恢复。
- 用户数据安全高于一切：rollback 永不 `git clean`，checkpoint 永不污染普通 `git log`（这是 UAT 零 P0 的根基，动 gittools.go 前先读 `M2.5-crash-window-matrix.md`）。

## 9. 关键文档索引

| 文档 | 位置 |
|---|---|
| 正式验收报告（10 项 E2E + P0/P1/P2 全清单） | `E:\agent-uat\Win7-CLI-Agent-正式验收报告.md` |
| rc-0.4 交付报告（改动+慢端点数据+回归对照） | `artifacts/slow-network-report.md` |
| rc-0.4 超范围发现 | `artifacts/slow-network-findings.md` |
| 历轮报告 | `artifacts/RC0.1~0.3.3-report.md`、`M4-report.md` 等 |
| 交付说明 | `DELIVERY-DOC.md` |
| 本轮 UAT 全部原始输出/审计 | `E:\agent-uat\vm\*-output*.txt`、`E:\agent-uat\vm\audit.jsonl`（358 条工具调用） |
| rc-0.4 包 | `dist/rc-0.4/pulse7-rc0.4.7z`（SHA256 `03d9433b155d970e4cd3ff68e9ee009818fb730e6d7718ca66d3b066426d9b4a`，exe `b0e835e645757c4b6361a7111d4d9253d0a6f548bb8ccb0b24bf3e2819aff106`） |
