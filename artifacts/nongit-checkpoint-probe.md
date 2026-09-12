# 非 git 工作区 checkpoint / rollback 实测报告（nongit-checkpoint-probe）

Date: 2026-09-10. 执行树：`E:/codex-worktrees/win7-agent/pre-phase3`（未提交、未打 tag、未 push、未建分支；本文件为本次唯一新增文件）。`E:/win7-agent` 全程只读。一行代码未改、一个测试未改——只跑、只看、只记。

## 0. 结论

**工作区不是 git 仓库时，用户仍然有回滚兜底。答案：有。**

rc-0.11 构建在非 git 工作区下自动切换到"私有 checkpoint 仓库"模式（输出中 `mode=private-checkpoint-repo`）：第一次 write/edit/shell 前 AUTO checkpoint 成功落盘（快照存放在**exe 安装目录**下的 bare 仓库），事后通过 `rollback {"to":1}` 把两个被改文件（含整个工作区其余未触碰文件）逐字节恢复到任务前状态（SHA256 全对上，`IDENTICAL_TO_SETUP = True`）。任务末摘要中"文件改动已存 checkpoint，可 rollback"这句承诺在非 git 工作区同样打印，且经实测为真。

部分边界未验证（见第 6 节，均逐条标注"未验证"），其中最值得注意的一条：整套兜底依赖 exe 旁的 `runtime\git`（本机经 junction 指向回归部署的 MinGit）；该依赖缺失时的行为本轮未实测（代码阅读结论：首个变更工具会直接报错）。

## 1. 环境与身份链

- VM：Win7 SP1 x64（`ver` = 6.1.7601），`ssh -p 2222 user@192.168.124.3`。
- 本次开机：`LastBootUpTime 20260910005842+480`（2026-09-10 00:58:42）。全部运行与日志时间戳在 22:46:05–22:49:46（+08:00），落在本次启动窗口内。
- 被测 exe：`C:/Users/user/f1f4-validation/pulse7-current.exe`（任务书所指 `pulse7-amd64.exe` 在 VM 上的实际文件名），
  SHA256 `c87065a6b02c89ae1b421cc65eb5b7622b5fd7cd62c30526f407b0d3962296fd`，9,325,568 字节，
  与 `f1f4-evidence/build-hashes.json` 中 `pulse7-amd64.exe`（源 `ed3731a`，rc-0.11）完全一致。
- bundled git：`C:/Users/user/f1f4-validation/runtime/git/cmd/git.exe`，`git version 2.46.2.windows.1`。
  注意：`runtime` 是指向 `C:\Users\user\win7-agent\runtime` 的 **junction**。
- 驱动方式：真实模型（`--base-url https://aigc789.top/v1 --model deepseek-v4-flash`），与 g1g5 实测同款命令行；无参硬编码 python 探针（`probe.py`，证据清单见第 7 节）驱动，逐字捕获 stdout。
- 台架纪律：每个工作区为全新目录（setup 阶段 rmtree 重建）；每份日志含 `RUN_START/RUN_END` 时间戳与 `EXIT_CODE=N`；产品侧终结记录 `=== EXIT DONE code=0 ... ===` 四次运行全部在位。

## 2. 非 git 组实测（ws-nongit，无 .git）

夹具（setup 时 SHA256 已入 `state-setup.json`）：`report.txt`（3 行原文）、`notes.txt`（1 行）、`会议纪要.txt`（中文文件名，UTF-8 内容）、`data.bin`（4096 字节随机二进制）。

运行 1（修改任务，session `nongit-a`，2026-09-10T22:46:05+08:00 → 22:46:20，EXIT_CODE=0）。
任务：把 report.txt 整体覆盖为 `AGENT-OVERWRITE-1`、向 notes.txt 追加 `AGENT-NOTE-1`。结果两文件均被改（SHA256 变化入 `nongit-a-meta.json`）。

**自动 checkpoint：成功。** 控制台原文（log 第 17 行，出现在第一次 write 尝试的权限行之后、工具结果之前）：

```
[checkpoint] AUTO checkpoint nongit-a/1 (kind=auto, a09dae5, time=2026-09-10T14:46:12Z, mode=private-checkpoint-repo, tree=09ffd94, dirty-files=4)
```

- `mode=private-checkpoint-repo` = 非 git 专用路径（对照组为 `user-repo+private-ref`）。
- **dirty-files 报了 4**（工作区全部 4 个文件；该字段语义为相对 checkpoint 索引/HEAD 的 git status 记录数，代码侧见 gittools.go，未做字节级对账）。

**产生的 checkpoint 记录 / 私有 ref / 数据文件**（全部在 exe 目录下，不在工作区内）：

```
C:\Users\user\f1f4-validation\data\workspaces\C__Users_user_nongit-probe_ws-nongit\
  checkpoint-metadata.jsonl   289 B
  checkpoint.git\             （bare 仓库）
    refs\pulse7\checkpoints\nongit-a\1     41 B
    objects\ 共 6 个松散对象（4 blob + 1 tree + 1 commit）
    index                     369 B
```

`checkpoint-metadata.jsonl` 原文：

```
{"workspace":"C:\\Users\\user\\nongit-probe\\ws-nongit","task_id":"nongit-a","kind":"auto","seq":1,"created_at":"2026-09-10T14:46:12.5180992Z","commit":"a09dae54fd255843962165eeb41cd59016e24a2c","tree":"09ffd941d89c1557406ea7f783ef5b80784dc20e","ref":"refs/pulse7/checkpoints/nongit-a/1"}
```

`git --git-dir=checkpoint.git for-each-ref` 原文：

```
refs/pulse7/checkpoints/nongit-a/1 a09dae54fd255843962165eeb41cd59016e24a2c
```

`git log --all --oneline` 原文：`a09dae5 win7-agent checkpoint refs/pulse7/checkpoints/nongit-a/1`；`count-objects -v`：`count: 6`。

**任务结束摘要关于回滚说了什么**（本运行无 shell 命令、无工作区外写入，摘要未含任何回滚相关文字）：

```
========================================
任务结束：已完成（共 5 轮，耗时 15s）
========================================
```

运行 2（rollback，`--resume` 同一会话，session task 仍为 `nongit-a`，22:47:13 → 22:47:17，EXIT_CODE=0）。
prompt 指示模型仅调用 `rollback {"to": 1}`。控制台原文：

```
resumed 13 messages from C:\Users\user\nongit-probe\nongit-a.jsonl (workspace=C:\Users\user\nongit-probe\ws-nongit task=nongit-a)
...
-> rollback
[permission] AUTO-ALLOWED rollback  (preset:open)
[rollback] target: checkpoint nongit-a/1 kind=auto created_at=2026-09-10T14:46:12Z commit=a09dae5
[OK] rollback （1 行）
```

rollback 工具完整结果串（自会话 jsonl 提取，197 字节，与审计 `{"ok":true,"result_bytes":197}` 一致）：

```
rolled back to checkpoint nongit-a/1 kind=auto created_at=2026-09-10T14:46:12Z commit=a09dae5; worktree restored (checkpoints available: 1); manifest cleanup removed 0 post-checkpoint Agent file(s)
```

**回滚后工作区哈希比对（vs setup 基线）**：

| 文件 | 恢复? | setup SHA256 = 回滚后 SHA256 |
|---|---|---|
| report.txt | ✅ | c41e7816a41294a51bc8a648c6d7c0cf41d8a9a6bfc9a82cd8323d8119fdadce |
| notes.txt | ✅ | a63292eb896c2958bd350a8e23d1cc2227a538d6a643190fe96780d8394a1f10 |
| 会议纪要.txt | ✅ | 9f71cd1dcd917e28cef9b02f59574c0a7b7f8f1dea27b2fc0e1b5b313e849a14 |
| data.bin | ✅（未被动过） | 432cc02ae2e22517e15569221309f6eb8f3f0f4d4ab5f73210651c2a85ddd381 |

探针输出原文：`IDENTICAL_TO_SETUP = True`（回滚后工作区与任务前逐字节一致）。

## 3. git 对照组实测（ws-git，git init + baseline commit 2fbffa4）

同样夹具、同样任务、同样流程（session `git-a` 22:47:44 → 22:47:54；rollback resume 22:48:06 → 22:48:11；均 EXIT_CODE=0）。

checkpoint 控制台原文：

```
[checkpoint] AUTO checkpoint git-a/1 (kind=auto, 096a75b, time=2026-09-10T14:47:52Z, mode=user-repo+private-ref, tree=1657a11, dirty-files=3)
```

- 私有 ref 在**用户仓库内**：`ws-git\.git\refs\pulse7\checkpoints\git-a\1`（`refs/heads/master` 仍指向 baseline `2fbffa4`，未动）。
- 元数据在 exe 目录：`data\workspaces\C__Users_user_nongit-probe_ws-git\checkpoint-metadata.jsonl`（280 B，内容格式同上）。
- 模式差异产物：`data\sessions\index-git-a-1.tmp`（369 B，临时索引）；非 git 组无此文件。

rollback 结果串原文：

```
rolled back to checkpoint git-a/1 kind=auto created_at=2026-09-10T14:47:52Z commit=096a75b; worktree restored (checkpoints available: 1); manifest cleanup removed 0 post-checkpoint Agent file(s)
```

回滚后哈希：4 个工作文件全部恢复（`match?=True`）；`IDENTICAL_TO_SETUP = False` 仅为 `.git` 内新增 checkpoint ref/objects——检查点记录本身，非用户数据差异。

## 4. 并列对照

| 维度 | 非 git 工作区 | git 工作区（对照） |
|---|---|---|
| AUTO checkpoint | 成功 | 成功 |
| 输出 mode= | `private-checkpoint-repo` | `user-repo+private-ref` |
| dirty-files= | 4（全部文件） | 3（3 个文本文件相对 baseline 的差异） |
| 快照存哪 | exe 目录 `data\workspaces\<wsID>\checkpoint.git`（bare，6 对象） | 用户仓库私有 ref + exe 目录元数据 |
| 用户仓库是否被碰 | 否（工作区无 .git） | 私有 ref 写入，master/index 未动，临时索引在 exe 目录 |
| rollback {"to":1} | 成功，结果串 197 B，`ok:true` | 成功，结果串同格式 |
| 回滚后 vs 任务前 | 逐字节一致（4/4 文件） | 逐字节一致（4/4 工作文件；.git 多出检查点 ref 属预期） |
| 任务末摘要（无 shell 时） | 仅"任务结束：已完成"，无回滚相关文字 | 同左 |

## 5. 带 shell 的补充观察（非 git 工作区，session `nongit-s`）

办公场景必然用 shell；该运行让模型 write 覆盖 report.txt 后执行一次 `dir`。checkpoint 行同样成功（`nongit-s/1 (kind=auto, b03f02a, ..., mode=private-checkpoint-repo, tree=09ffd94, dirty-files=4)`；tree 与 nongit-a/1 相同，因回滚后内容一致，可交叉印证快照确定性）。任务末摘要原文：

```
========================================
任务结束：已完成（共 5 轮，耗时 9s）
本次任务执行的 shell 命令（不可通过 rollback 回退）：
  1. dir  (2026-09-10T22:49:44+08:00)
文件改动已存 checkpoint，可 rollback；以上命令的外部影响不可回退。
========================================
```

即：非 git 工作区下产品照样承诺"文件改动已存 checkpoint，可 rollback"，且该承诺与第 2 节实测一致。（此运行后 report.txt 处于被覆盖状态，属探针工作区，非用户数据。）

## 6. 未验证 / 边界（均未运行，标注来源）

1. **`runtime\git` 缺失时的行为**：未验证。代码阅读（gittools.go:65-69，`newGitOps` 找不到 bundled git 即报错）显示该错误会经 `ensureAutomaticCheckpoint` 使首个 write/edit/shell 直接失败——未运行验证。
2. **快照寿命与位置**：快照在 exe 安装目录下（本轮实测位置事实）。安装目录被删/清盘后兜底是否随之消失、以及对 `D:\部门文档` 这类跨卷工作区的磁盘占用——未验证、未量化。
3. **wsID 路径截断碰撞**：`wsID` 取路径尾 60 字符（gittools.go:103-110，代码阅读），超长路径可能碰撞共享同一 checkpoint.git——未验证。
4. 工作区位于某个 git 仓库的**子目录**（自身无 .git、父目录有）时走哪条模式：未验证。
5. `rollback {"to":N}` 中 N 不存在、以及 >1 个检查点时回滚到中间检查点：未验证（仅实测 to:1、单检查点）。
6. 真人 REPL 手动触发 rollback 的入口与体验：未验证（本轮 rollback 均由模型工具调用触发）。
7. dirty-files 的字节级成因对账（git 组为何是 3 而非 0）：仅记录数字与代码侧计数语义，未做 blob 级对账；参照既有 dirty-files=2524 定性（换行差噪音类）。

## 7. 证据清单

VM（保留原位，可复核）：

- `C:/Users/user/nongit-probe/`：`state-setup.json`、`boot-time.txt`、`nongit-a.log|jsonl|-meta.json|-audit.jsonl`、`nongit-rb.log|-audit.jsonl`、`git-a.log|jsonl|-meta.json|-audit.jsonl`、`git-rb.log|-audit.jsonl`、`nongit-s.log|jsonl|-meta.json|-audit.jsonl`、`hash-ws-nongit.json`、`hash-ws-git.json`、`prompt-*.txt`、各 `-home` 隔离 USERPROFILE。
- `C:/Users/user/nongit-probe.py`（探针，与 host 副本同内容）。
- checkpoint 数据：`C:/Users/user/f1f4-validation/data/workspaces/C__Users_user_nongit-probe_ws-nongit/`（含 `checkpoint.git`、`checkpoint-metadata.jsonl`）与 `.../C__Users_user_nongit-probe_ws-git/checkpoint-metadata.jsonl`；`data/sessions/index-git-a-1.tmp`。

Host 副本（`E:\agent-uat\nongit-staging\`，pscp 逐文件拉回，规避 pscp 中文名坑——中文名文件以 SHA256 入 meta json 为准）：上述全部 log/jsonl/meta/audit + `probe.py`。

## 8. 对后续决策的输入（不含建议，仅事实）

- "办公场景安全网是假的"这一担忧在 rc-0.11 构建上**不成立**：非 git 工作区有完整的 checkpoint→rollback 链路，且对外承诺与实际一致。
- 该兜底的**真实依赖**是 exe 旁 bundled MinGit 与 exe 目录可写；快照不在工作区内、不在 `.pulse7` 下，而在安装目录 `data\workspaces\`。任何"改前备份到工作区 `.pulse7\backups`"的替代/补充方案，需与现状（备份与工作区同盘、同删除命运）对照权衡。
