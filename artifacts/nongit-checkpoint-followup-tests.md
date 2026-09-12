# 非 git checkpoint 跟进测试：磁盘量化 / 中文长路径 / data 位置与摘要缺口（第二轮）

Date: 2026-09-10（第二轮，同日 23:18–23:20 执行）。执行树 `E:/codex-worktrees/win7-agent/pre-phase3`（未提交、未 tag、未 push；本文件为本轮唯一新增文件）。约束遵守情况：**一行代码未改、一个既有测试未改**，只跑、只看、只记。第一轮报告：`artifacts/nongit-checkpoint-probe.md`。

被测 exe 与第一轮同一身份：`C:/Users/user/f1f4-validation/pulse7-current.exe`，SHA256 `c87065a6b02c89ae1b421cc65eb5b7622b5fd7cd62c30526f407b0d3962296fd`（rc-0.11，源 `ed3731a`）；VM 同一开机窗口（boot 2026-09-10 00:58:42，本轮全部运行在 23:18–23:20）；真实模型 deepseek-v4-flash；全部运行 EXIT_CODE=0 且带 `=== EXEC-DONE ===` 终结记录。

## 测试 C：`data` 位置有没有配置项（代码核查，只读）

**没有。** `agentConfig` 的全部 JSON 键（config.go:18-45）为：`base_url / api_key / model / workspace / box / start_exe / sandbox_root / sandbox_preference / yolo / read_only / shell_timeout_sec / memory_limit_mb / max_ctx / max_rounds / process_warn_threshold / background_task_max_output_mb / background_task_warn_count / background_task_warn_sec / cleanup_on_exit / llm_first_chunk_timeout_sec / llm_idle_timeout_sec / llm_max_retries / llm_compress_timeout_sec`——**无任何 data/存储目录项**。

存储路径全部由 exe 自身位置推导：main.go:66-67 `exeDirStore()` = `filepath.Dir(os.Executable())`；sessions/logs/audit/permissions 均挂其下（main.go:287、318）；checkpoint 仓库 `gittools.go:75` 同样挂在 `<exeDir>\data\workspaces\<wsID>\checkpoint.git`。**结论：exe 放哪个盘，data（含快照）就在哪个盘；无用户可改项。**

**大小上限：没有。** gittools.go / tools.go 的 checkpoint 路径无任何容量限制；cleanup.go 唯一相关清理是 `purgeStaleIndexTemps`（data/sessions 下 `index-*.tmp` 临时索引），**checkpoint 的 refs、对象、metadata jsonl 均无清理、无上限、只增不减**（代码阅读结论；测试 A 提供实测增长数据）。

## 测试 B：中文长路径工作区（一次实测）

**构造**（探测脚本按 Go 的 `wsID` 语义计算：替换 `\ : / space .` 后按 **UTF-8 字节** 取尾 60）：

- 工作区：`C:\Users\user\nongit-probe2\部门共享文件夹\二零二六年第三季度工作总结与财务报表归档\市场部提交的最终版本材料`
- 全路径 UTF-8 长度 **147 字节**（>60，触发截断）
- 计算 wsID（60 字节）以 `0xBB 0x93` 开头——**断在多字节序列中间**，解码后为 2 个 U+FFFD + 正常中文：ascii 表示 `'\ufffd\ufffd\u4e0e\u8d22\u52a1\u62a5\u8868\u5f52\u6863_\u5e02\u573a\u90e8\u63d0\u4ea4\u7684\u6700\u7ec8\u7248\u672c\u6750\u6599'`
- 夹具 3 文件（ASCII 文件名，隔离"目录路径中文"这一单变量；中文**文件名**第一轮已测过）

**结果：checkpoint 与 rollback 全链路正常。**

修改任务（session `cn-a`，23:18:08→23:18:16，8s）checkpoint 行原文：

```
[checkpoint] AUTO checkpoint cn-a/1 (kind=auto, 163e00f, time=2026-09-10T15:18:13Z, mode=private-checkpoint-repo, tree=e9b2fc2, dirty-files=3)
```

数据目录**实际创建名**（探测脚本 `ascii()` 输出，含两个替换符）：

```
'\ufffd\ufffd\u4e0e\u8d22\u52a1\u62a5\u8868\u5f52\u6863_\u5e02\u573a\u90e8\u63d0\u4ea4\u7684\u6700\u7ec8\u7248\u672c\u6750\u6599'  （31,483 B，26 个文件，5 个 git 对象）
```

rollback（`--resume` 同会话，23:18:27→23:18:33，6s）原文：

```
[rollback] target: checkpoint cn-a/1 kind=auto created_at=2026-09-10T15:18:13Z commit=163e00f
[OK] rollback （1 行）
```

哈希比对：`FILES_SETUP=3 FILES_NOW=3 DIFFS=0 IDENTICAL_TO_SETUP=True`。

**定性**：与 M-4（AGENT.md 8KB 字节截断切断中文字符）同类的截断**确实发生**——wsID 目录名带 U+FFFD 乱码——但在 checkpoint/rollback 链路上**功能无损**（目录名只需稳定唯一，不需要可读）。碰撞场景维持第一轮降级定性（要求两工作区路径尾 60 字节完全相同，罕见；60 字节以内不截断）。

## 测试 A：磁盘量化（157MB / 355 文档树跑 checkpoint）

**夹具**（`ws-big`，共 355 文件、164,691,631 B ≈ 157MB）：
100 个唯一二进制（0.5–2MB 随机字节，模拟 docx/xlsx 类不可压缩内容）、40 个重复副本（2 个模板 × 各 20 份，模拟"同模板到处复制"）、200 个小文本、12 个 `tick-K.txt`、3 个 fixture 文件。

**① 首个 checkpoint（big-a/1，任务 15s，含 cp 的第 2 轮 8s）**——checkpoint 行原文：

```
[checkpoint] AUTO checkpoint big-a/1 (kind=auto, 22d3e92, time=2026-09-10T15:19:25Z, mode=private-checkpoint-repo, tree=4d8e5e1, dirty-files=355)
```

| 指标 | 数值 |
|---|---|
| 工作区 | 164,691,631 B / 355 文件 |
| checkpoint.git（cp1 后） | **104,825,456 B（≈100MB）** |
| git 对象数 | **319** = 317 blob + 1 tree + 1 commit |
| count-objects size | 102,314 KB |

**去重证据**：355 个文件只存 317 个 blob——40 个重复副本只占 2 个 blob（同内容 = 同对象）。压缩：157MB → 100MB（随机二进制基本不压，文本部分压掉）。

**② 12 次修改 + 2 个新 checkpoint（big-t，27s）**——checkpoint 行原文（两次相差 0.31s：模型把 12 个 write 批在同一轮，第 1 与第 11 次修改各触发一次，符合每 10 次修改的设计间隔）：

```
[checkpoint] AUTO checkpoint big-t/1 (kind=auto, 8fab07f, time=2026-09-10T15:20:14Z, mode=private-checkpoint-repo, tree=fd2f2a1, dirty-files=355)
[checkpoint] AUTO checkpoint big-t/2 (kind=auto, dd92feb, time=2026-09-10T15:20:14Z, mode=private-checkpoint-repo, tree=31704fc, dirty-files=355)
```

| 指标 | cp1 后 | big-t 后 | 增量 |
|---|---|---|---|
| checkpoint.git | 104,825,456 B | 104,843,047 B | **+17,591 B** |
| git 对象 | 319 | 335 | +16（12 个文件新版本 blob + tree + commit） |
| metadata jsonl | 281 B | 843 B | +562 B（2 行） |

其中 `big-t/1` 是**跨任务全树重扫**（新 task 首次修改必拍全量），对已入库内容零重复存储（内容寻址去重跨任务生效）；`big-t/2` 只新增修改过的文件版本。

**③ 成本模型（实测锚点，非推测）**：
- 首个 checkpoint ≈ 工作区**唯一内容**的 zlib 压缩后大小（本例 157MB→100MB，63%）；重复文件近乎免费。
- 后续每个 checkpoint ≈ 自上个 checkpoint 以来**修改过的文件**的新版本之和（本例 12 改 2 拍仅 +17.6KB）。
- 时间：157MB 树的单次全量 checkpoint 含在 8s 的工具轮内（含模型往返）；12 写 + 2 拍全树的任务 27s。Win7 VM、MinGit 2.46.2。
- 无上限、无回收（代码核查见测试 C）——工作区几个 GB 时按上表比例外推即得量级，但**外推数字未实测**。

**④ dirty-files 语义注意**：mode 2（非 git）下 `dirty-files` 恒等于工作区全部文件数（355、355、355），与实际改动量无关——第一轮已见（4），本量级更直观。该字段在非 git 下不表达"这次改了多少"。

## 测试 D：任务末摘要缺口（行为证据汇总，本轮未改代码）

同一产品、同一天的四次实测对照：

改了文件、**跑了 shell**（第一轮 nongit-s，非 git 工作区）：

```
任务结束：已完成（共 5 轮，耗时 9s）
本次任务执行的 shell 命令（不可通过 rollback 回退）：
  1. dir  (2026-09-10T22:49:44+08:00)
文件改动已存 checkpoint，可 rollback；以上命令的外部影响不可回退。
```

改了文件、**没跑 shell**（本轮 big-a，157MB 文档被覆盖 report.txt/notes.txt 后）：

```
任务结束：已完成（共 3 轮，耗时 15s）
```

代码位置（阅读结论）：session.go `endOfTaskSummaryLines`——"文件改动已存 checkpoint，可 rollback"只挂在 shell 命令块内；无 shell 且无工作区外写入时整块不打印。**刚被覆盖文件的用户反而得不到"有得救"的提示**。按裁决这属"一行改动"的产品缺口，本轮依约束未动代码，证据在此备查。

## 未验证（明确列出，防误引）

1. 分发包内 MinGit 完整性 + 干净机器解压即用（等打包阶段一起做）。
2. 大文档树的 rollback（本轮大树只测了增长；rollback 正确性已在 3 个小工作区验证）。
3. 超过 2 个 checkpoint 时的 `rollback {"to":N}`（N 为中间序号 / 不存在）。
4. wsID 碰撞（两个工作区尾 60 字节相同共享同一 checkpoint.git）——维持降级定性，未实测。
5. GB 级工作区的实际耗时/体积（只有 157MB 实测锚点）。

## 证据清单

- VM：`C:/Users/user/nongit-probe2/`（`state-setup.json` 含 exe 哈希与全部夹具 SHA256、`cn-a|cn-rb|big-a|big-t` 的 log/jsonl/meta/audit、`hash-cn.json`）；`nongit-probe2.py` 探针；checkpoint 数据在 `C:/Users/user/f1f4-validation/data/workspaces/`（`C__Users_user_nongit-probe2_ws-big` 与带 U+FFFD 的中文 wsID 目录）。
- Host 副本：`E:\agent-uat\nongit-staging\`（probe2.py 及上述 log/meta）。
- 第一轮证据未动：`C:/Users/user/nongit-probe/` 与 `E:\agent-uat\nongit-staging\`（第一轮文件）。
