# Retrieval Task 报告（2026-09-05）

## 成败判据：R2 探索任务收敛 —— ✅ 达成

| 指标 | R2 基线（RC0.2） | R2 本轮（tree + rg） | 变化 |
|---|---|---|---|
| 轮次 | **30（撞上限）** | **19（自然收敛）** | **-37%** |
| ls 调用 | **105** | **2** | **-98%** |
| tree 调用 | 0（不存在） | 8 | 新工具被模型主动使用 |
| read 调用 | 26 | 22 | 基本持平 |
| 压缩次数 | **22** | **0** | **-100%** |
| 是否收敛 | ❌ 空终答 | ✅ 完整项目结构描述 | **关键突破** |
| 工作区改动 | 零 | 零 | 保持 |

**核心结论**：tree 工具直接解决了 R2 失败模式——模型不再逐目录遍历（105 次 ls → 2 次），上下文不再碎片化（22 次压缩 → 0 次），因此能在轮次内收敛并给出完整答案。

## T1 tree 工具

- 紧凑树形输出，目录显示直接子项数量
- 默认跳过 .git/node_modules/vendor/dist/build/target/__pycache__/.venv/bin/obj/隐藏目录
- max_depth=3 / max_entries=300 默认值，超出报告省略数
- 工具描述明确"了解项目结构优先用 tree，不要用 ls 逐层遍历"

## T2 grep → ripgrep 集成

- **ripgrep 13.0.0**（冻结版，MIT，4.6MB，Win7 x64 验证可用）放入 `runtime\rg\`
- SHA256: `ab5595a4f7a6b918cece0e7e22ebc883ead6163948571419a1dd5cd3c7f37972`
- 调用参数：`--no-heading --line-number --smart-case --max-count 200 -g <glob>`
- 探测不到 rg.exe 或调用失败 → 自动回退原 Go 实现（返回标注 `[ripgrep]` / `[go-fallback]`）
- 输出上限从 50 行提升到 200 行（实测常见函数名命中数百处，50 行过苛）

## T3 R1 回归

R1 文件在前次测试中已被修改为目标状态（中文），模型正确识别"已是目标状态，无需改动"（1 轮，0 写）。任务执行逻辑完整。

## T4 更名 pulse7

- 可执行文件：`pulse7.exe`（SHA256 `ec5b81348a11ae71`）
- 环境变量：`PULSE7_API_KEY`（旧 `WIN7_AGENT_API_KEY` 保持兼容回退）
- git refs：新 `refs/pulse7/checkpoints/` + 旧 `refs/win7-agent/checkpoints/` 双命名空间扫描（跨命名空间 rollback 已验证）
- profile 目录：`.win7-agent` → `.pulse7`（修复 `.claude/` 污染问题）
- Sandboxie box：保持 `Win7Agent`（改名会导致已安装 box 孤立）

## T5 S2 数据异常

**结论**：提示词调优副作用。调优后的"改动后验证"指令使模型过度执行——edit 后 `type` 检查看到 GBK 乱码（cmd 读 UTF-8 文件必然如此），以为改错，re-edit 循环 4 轮后放弃 edit 改为全文件 write。最终结果正确但多花 6 轮。详见 `retrieval-findings.md`。

## ripgrep 性能对比

| 关键词 | 旧 Go grep | ripgrep（Win7 真机） | 提速 |
|---|---|---|---|
| error | ~300ms（dev 估） | 59ms | ~5x |
| ConnectionString | ~300ms | 52ms | ~6x |
| zzzqqqxyzzy（零命中） | ~300ms | 45ms | ~7x |

## 交付

- 分支 `retrieval`，4 commits（T5→T1+T2→T3→T4）
- go.mod / go.sum 零 diff（rg 为外部 exe）
- 四旧 tag 未移动
- 冻结清单新增：ripgrep 13.0.0（见上文 SHA256）
