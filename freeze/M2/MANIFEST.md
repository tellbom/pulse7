# M2 正式冻结（2026-09-05）

> 冻结后 M3 原则上只动：installer / bootstrap / 环境探测 / 沙盒 adapter 配置 / 清理 / 产品配置。
> **禁止顺手重构 M2 已 PASS 的：Git 工具、文件工具、session、LLM loop。**

## 冻结制品（SHA256 见同目录 SHA256SUMS.txt）

| 制品 | 位置（工作区相对路径） | SHA256（前 16） |
|---|---|---|
| win7-agent.exe（M2 最终二进制） | `dist/win7-agent/win7-agent.exe` | `4a3616eb04b78180` |
| MinGit 2.46.2 x64（产品 runtime\git 来源） | `tools/dl/MinGit-2.46.2-64-bit.zip` | `0dca60869825ceb8` |
| Sandboxie Classic v5.73.2 x64 安装包 | `tools/dl/Sandboxie-Classic-x64-v5.73.2.exe` | `18239310d6ad247e` |
| Go 1.20.14 工具链（仅开发机） | `tools/dl/go1.20.14.windows-amd64.zip` | `0e0d0190406ead89` |
| go-openai | `v1.42.0`（agent/go.sum + agent/vendor 固化） | — |
| Sandboxie.ini 基线 | `freeze/M2/Sandboxie.ini`（副本；真机 C:\Windows\Sandboxie.ini） | — |

产品目录运行形态（dist/win7-agent/）：`win7-agent.exe + runtime\git\`（MinGit 解包 111MB）。

## M2 覆盖的能力（全部真机 PASS，详见报告 v3.2）

Agent loop（流式 + tool-call 装配）/ Tool Registry（10 工具）/ Workspace Policy（含相对路径工作区根解析）/ 极薄 Context / 行式 REPL + exec(headless) + 内嵌 mock / checkpoint/rollback（用户仓库私有 ref + restore；非 git 独立 checkpoint.git + reset --hard；manifest 精确清理）/ 会话 .jsonl 记录与 resume / Sandboxie adapter（wrapper 文件捕获）/ JobObject 自动降级。

## 验证日志（artifacts/，均真机）

M0.5：env / git-poc / gate-a(-supplement) / gate-bcd / gate-e-* / M0.5-freeze-manifest
M1：m1-smoke-win7
M2：m2-smoke-win7-r3（全量）/ m2-degrade-win7（零 Sandboxie 降级实测）

## 版本基线

- 本仓库 git tag：**`m2-freeze`**
- 构建：`GOOS=windows GOARCH=amd64 CGO_ENABLED=0`，Go 1.20.14，vendor 模式
