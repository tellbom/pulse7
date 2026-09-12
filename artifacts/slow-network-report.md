# slow-network 分支交付报告（rc-0.4）

- 分支：`slow-network`（自 tag `rc-0.3.3` 切出）
- 目标：让 Agent 在慢 LLM 端点上稳定跑完任务，不再被自己的超时掐死
- 一句话结论：**全部达成。10 分钟持续慢流完整跑完不超时；网络偶发错误自动重试；真端点 A/B 中旧版猝死、新版跑满。**

## 改动清单（每 T 独立 commit）

| # | commit | 内容 |
|---|---|---|
| T1 | `f099746` | 移除整轮 5 分钟 ctx，改为每请求"首字节 300s + 空闲 120s"双看门狗；只要流在吐字永不掐死 |
| test | `3127d5c` | mock 慢端点触发器（SLOWDRIP/SLOWQUEUE/SLOWSTALL/SLOWMID/FAIL500ONCE/FAIL401ALWAYS） |
| T2 | `2a09ad0` | 可重试网络错误退避重试（5s/15s，`llm_max_retries`，默认 2）；空闲超时/4xx/余额不足不重试 |
| T3 | `0da0e98` | 上下文压缩调用独立 180s 预算（`llm_compress_timeout_sec`），不再与主循环抢预算 |
| T4 | `140cd2a` | 等待期每 15s 心跳 `[等待模型响应... Ns]`（首块到达即停）+ 每轮 `[第 N 轮完成，耗时 XmYs]`；纯行追加，无 ANSI/VT |
| T5 | `a3eb6cd` | session 惰性落盘（0 消息不生成文件）；`--resume` 复用原文件不再 fork；EXEC-ERROR 打印可照抄续跑命令（含具体 id 与原任务）；Ctrl-C 提示带 id |
| cfg | `60eabae` | 模板带 `llm_*` 默认值与 `_doc_` 注释（JSON 无注释，用相邻键） |

约束遵守：`go.mod`/`go.sum` **零 diff**；未新增工具/功能；未改系统提示词与工具描述；未动 30 轮上限与压缩阈值；全部历史 tag 未移动。超范围问题记录于 `artifacts/slow-network-findings.md`（未顺手修）。

## 核心机制：计时方式从"总量预算"改为"活性判定"

旧：一个 5 分钟 ctx 罩住整轮（全部 LLM 调用 + 压缩 + 工具挂钟时间）→ 流还在吐字，计时器照样掐死。
新：turn ctx 只承载中断取消；每个请求两个独立计时——

| 计时器 | 覆盖 | 默认 | 重试 |
|---|---|---|---|
| 首字节 | 请求发出 → 第一个数据块（含等 SSE 头的排队阶段） | 300s（`llm_first_chunk_timeout_sec`） | 是 |
| 空闲 | 相邻数据块间隔，每块重置（keepalive 也算数据） | 120s（`llm_idle_timeout_sec`） | 否（流已建立后卡死，重试同样会卡） |

## 慢端点测试数据（mock 可控延迟）

| 用例 | 配置 | 结果 |
|---|---|---|
| **每 30s 一个 chunk，持续 10 分钟（验收判据）** | 默认看门狗 | **第 1 轮 10m0s 完整跑完，EXEC-DONE**（旧版 5 分钟必死） |
| 连接建立后永不吐数据（SLOWSTALL） | first=3s | 首字节超时，错误信息含配置项名 |
| 长排队后正常吐字（SLOWQUEUE 45s） | first=60s | 心跳 15/30/45s 三行后正常完成 |
| 流建立后卡死（SLOWMID） | idle=2s | 首块已收，空闲超时，**0 次重试** |
| 配置项生效 | config 文件 idle=1s | 1s 即判卡死（flag>config>默认三层验证） |
| 5xx 一次（FAIL500ONCE） | — | `[重试] ... 5s 后第 1 次重试` 后恢复，EXEC-DONE |
| 永久 401（FAIL401ALWAYS） | — | 立即失败，无重试 |

## 全量回归对照表

| 场景 | RC 0.3.3 基线 | RC 0.4（本分支，VM 实测） | 判定 |
|---|---|---|---|
| mini-suite（M1/M2-FILES/M2-GIT/M3/T4-NORM） | 5/5 | **5/5** | ✅ |
| **慢端点模拟 30s×20** | 必死于 5min | **10m0s 完整完成** | ✅ **本轮成败判据** |
| shell20 连续 | 20/20 | **20/20（100%）** | ✅ |
| S1 修 bug | 4-5 轮 | **6 轮 / 1 edit / 30.0 验证通过** | ✅ |
| S2 加功能 | 5 轮 / 2 写 | **7 轮 / 2 写 / hello 验证通过** | ✅ |
| S3 模糊 | 4 轮追问 | **4 轮 / 0 写 / 追问** | ✅ |
| R1 单文件翻译 | 收敛 | **4 轮 / 3 edits / 仅目标文件变更（git status 单文件）** | ✅ |
| R2 项目解说 | JobObject 3/3 收敛 | 0/3（1 追问 20 轮 + 2 触顶 30 轮），全程零网络错误 | ⚠️ 见 findings F-1 |
| Gate A（MinGit 全流程） | 通过 | **全部 MARK + DONE** | ✅ |
| Gate B（poc-b）/C（go-openai） | 通过 | **gateb-ok / gatec-http-ok** | ✅ |
| 真模型 E2E S1（bigmodel glm-4.5-flash，本地） | — | 修复正确、每轮耗时行正常、EXEC-DONE | ✅ |

### R2 A/B 对照（关键证据）

同日同端点（aigc789 当天响应明显变慢）：

| exe | R2 结果 |
|---|---|
| rc-0.3.3（旧） | `EXEC-ERROR: context deadline exceeded`（压缩调用同报 deadline）——**直接猝死** |
| rc-0.4（新） | 三次全部活到追问/30 轮上限，零网络猝死 |

R2 不收敛是模型轮次管理问题（read 无分页 → powershell 逐段读 600 行 Program.cs，见 findings F-1），与网络韧性改动无关。

## T5 效果验证

- EXEC-ERROR 实测输出：
  ```
  EXEC-ERROR: <原因>
  已完成的进度已保存。续跑命令：
    pulse7.exe --resume "e1.jsonl" "<原任务>"
  ```
- resume 后**复用同一文件**（"resumed 2 messages from e1.jsonl"），不再每次 fork 新 session；
- RESUME-ERROR / 早期死亡不再产生 0 消息空文件（lazy 落盘）。

## 交付物

| 项 | 值 |
|---|---|
| 包 | `dist/rc-0.4/pulse7-rc0.4.7z`（34,053,354 B） |
| 包 SHA256 | `03d9433b155d970e4cd3ff68e9ee009818fb730e6d7718ca66d3b066426d9b4a` |
| exe SHA256 | `b0e835e645757c4b6361a7111d4d9253d0a6f548bb8ccb0b24bf3e2819aff106`（go1.20.14, windows/amd64, CGO_ENABLED=0） |
| tag | `rc-0.4` |
| 试用期预期说明 | 包内 `快速上手.md` 新增"内网慢模型的预期（必读）"一节 |

## 本轮能/不能解决（复述确认）

- **能**：不再被自己的超时掐死；偶发网络错误自愈；用户看得到"还在跑"；失败给可照抄续跑命令。
- **不能**：任务总耗时仍由模型速度决定（内网每轮 3 分钟 × 8 轮 = 24 分钟是物理事实）。
