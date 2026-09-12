# encoding-pagination 交付报告（rc-0.5）

- 分支：`encoding-pagination`（自 tag `rc-0.4` 切出）
- 三件事：查清并修复 shell 中文编码、`--prompt-file`、`read` 分页——**全部完成**
- 一句话结论：**中文 shell 双向乱码确认并修复（两种沙盒模式 0 U+FFFD）；read 分页把 R2 从"三轮全崩"变成"6/6 跑出 5 次收敛、轮次减半"。**

## 改动清单（每 T 独立 commit）

| # | commit | 内容 |
|---|---|---|
| T1 | `b760aa5` | 只读排查 + 字节级根因文档 `artifacts/encoding-rootcause.md`（仅文档，无代码） |
| T2 | `71c9538` | Win32 `MultiByteToWideChar`/`WideCharToMultiByte`（CP_ACP，零依赖）双向转码：out.txt 非 UTF-8 输出→UTF-8；inner.cmd 命令→ANSI 使中文路径可解析 |
| T3 | `ea04a8c` | read 分页：`offset`（1 起）/`limit`，结果标注 `[第 X-Y 行，共 N 行]` 与续读 offset；越界友好提示；limit 上限 2000；7 个单元测试 |
| T4 | `0d2b211` | `--prompt-file`：UTF-8 读取、容忍 BOM、与位置参数互斥、exec/--resume 通用 |

约束遵守：`go.mod`/`go.sum` 零 diff；未动 30 轮上限/压缩阈值/系统提示词/`gittools.go`；全部 tag 未移动；超范围问题记 `artifacts/encoding-findings.md`（5 条，未顺手修）。

## T1 根因结论（详见 encoding-rootcause.md）

- **输出方向**：中文系统控制台 936 下，cmd/程序输出的中文以 GBK 写入 out.txt，`readResult` 裸 `string(outB)` → 模型上下文乱码，json.Marshal 再变 U+FFFD 有损。
- **命令方向**：inner.cmd 存 UTF-8 命令、cmd 按 936 解析 → 中文路径变 mojibake 路径（`dir 中文目录` exitcode 0→1）。
- **长期未暴露的原因**：历次测试台架的 PowerShell 启动器设了 `[Console]::OutputEncoding=UTF8`，把 console CP 变 65001 掩盖了 bug；纯 cmd 用户必现。

## T2 验收（VM，纯 cmd 启动器，无 OutputEncoding 依赖）

| 用例 | rc-0.4（修复前） | rc-0.5（修复后） |
|---|---|---|
| `chcp` 输出 | `\ufffd\uedaf…: 936` | `活动代码页: 936` ✅ |
| `echo 中文测试ABC` | 侥幸正确（GBK 字节对双射） | 正确（不再靠运气） ✅ |
| `dir C:\uat\08-中文项目` | exitcode=1 + 乱码 | **exitcode=0 + 正确列出中文名** ✅ |
| `type 不存在的中文文件` | 乱码 | `系统找不到指定的文件。` ✅ |
| 纯 ASCII | 不变 | 不变 ✅（utf8.Valid 直通） |
| JobObject / Sandboxie | 都乱 | **两种模式全部 0 U+FFFD** ✅ |

## T3 验收（单元测试 + 实跑）

7 个单测全过：默认窗口向后兼容、显式 offset/limit 带续读提示、精确尾页、越界提示、limit 钳制 2000、空文件、小文件整读。
实跑见 R2：模型自发使用 `read offset=90/182/267` 分页读 600 行 Program.cs，**powershell 逐段行循环从 6 次降到 0**。

## T4 验收

带 BOM / 不带 BOM 的 UTF-8 中文 prompt 文件均正确执行（本地 mock M1-SMOKE/M3-SMOKE 双触发通过）；与位置参数同时给出时明确报错。本轮 VM 全部真模型场景即以 `--prompt-file` 驱动（启动器从 PowerShell 体操简化为纯 ASCII cmd）。

## T5 全量回归对照表

| 场景 | rc-0.4 基线 | rc-0.5（本轮 VM 实测） | 判定 |
|---|---|---|---|
| mini-suite（M1/M2-FILES/M2-GIT/M3/T4-NORM） | 5/5 | **5/5** | ✅ |
| 慢端点触发器（SLOWDRIP/SLOWQUEUE/SLOWMID/FAIL500ONCE/FAIL401） | 全过 | 全过（本地复测） | ✅ 维持 |
| shell20 | 20/20 | **20/20** | ✅ |
| S1 修 bug | 6 轮 | **4 轮 / 30.0 验证** | ✅ |
| S2 加功能 | 7 轮 / 2 写 | **5 轮 / 2 写 / hello 验证** | ✅ |
| S3 模糊 | 4 轮追问 | **3 轮追问 / 0 写** | ✅ |
| R1 单文件翻译 | 4 轮 / 3 edits | **8 轮 / 4 edits / 仅目标文件（git status 单文件）** | ✅ 可比 |
| **R2 JobObject ×3** | **0/3 收敛**（20 追问 + 2×30 触顶） | **3/3 收敛，9/7/7 轮** | ✅ **显著改善** |
| **R2 Sandboxie ×3** | （未测；rc-0.3.3 基线 1/3） | **2/3 收敛（8/8 轮）+ 1 次第 13 轮无声退出** | ⚠️ 优于基线，残留见 findings F-2 |
| TEST-08 中文项目 | PASS（台架掩盖编码） | **PASS**（纯 cmd 链路，统计结果正确、中文名完整） | ✅ |
| Gate A / Gate B(C) | 通过 | **全 MARK / gateb-ok / gatec-http-ok** | ✅ |

R2 轮次对比（同一端点同模型）：rc-0.3.3 JobObject 15/15/18 → rc-0.4 0/3（20/30/30）→ **rc-0.5 9/7/7 全收敛**。根因链：read 无分页 → 模型用 powershell 逐段读大文件 → 轮次爆炸 → 触顶/追问。

## 交付物

| 项 | 值 |
|---|---|
| 包 | `dist/rc-0.5/pulse7-rc0.5.7z`（34,067,156 B，SHA256 见下行） |
| 包 SHA256 | `077e0a0a66b672d031c722dc4749ce31506914f6a7c67088aa0a199c0e1044d0` |
| exe SHA256 | `98fa12f0dc96c5e3ee2e42b6bfd38f94c5ff353e83ad939c5b20eceb95337747`（go1.20.14, windows/amd64, CGO_ENABLED=0） |
| tag | `rc-0.5` |
| 快速上手更新 | 新增 `--prompt-file` 用法与"中文任务描述推荐用此方式"说明 |
