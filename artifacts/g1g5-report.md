# G1–G5 执行报告

## 最终验收（2026-09-11，以本节覆盖下方历史阶段状态）

**G1–G5各项验收：实测通过，证据如下。最终Win7 amd64/386各138项通过、0失败、0跳过、exit0；真实模型R1/S1完成。** 不作发布或交付决定。证据文件名均相对 `artifacts/g1g5-evidence/`。

最终源码来自干净提交 `170945cfe67a16c980d4dbf698c0387ab6feb740`，agent tree为 `b6031b6bf999c56ea311f6666a3842285ba1bd79`。最终归档只改文档。构建全过程及实际完整命令见 `build-record.md`，产物哈希见 `build-hashes.json`，VM在执行前重新计算测试exe哈希；产品哈希见模型meta，均与本地一致。`go version -m`没有vcs.revision，不宣称有；源码溯源依靠干净树核对和commit/tree一致性。

### 提交与v2归档

| 内容 | commit |
|---|---|
| lifecycle文档增量收尾，docs-only | 003b759 |
| 任务书v1原样归档，docs-only | d88eb56 |
| v2更正：G4裁决，docs-only | 273af97 |
| G1首版序号方案 | 6dcb84f |
| G2来源文案 | 31a2d42 |
| G3仅测试转义 | fb40b65 |
| G4普通status与标签 | 1277383 |
| G5规则测试及用户文档 | a29e00b |
| G1最终补正，原生LUID覆盖管理器重建 | 9e73063 |

G项各自独立，G1另有一个补正提交；禁止改写历史，因此没有amend/squash首版。v1归档SHA256为 `c5c8540bdeaf38426d9360131ddd0cfa4bff89a35e05fe22012f68dea693f85a`；v2为 `ecd94286c9f5ebb1221e1d93d47eaebdd8a4b017a333ad312d9333767e366226`，均与只读历史源文件当时字节一致。v2纠正G4定义、展示和8项验收：计数是采集时普通status，包含已有改动、首次AUTO为0正常，任务归属清单另列阶段三契约。历史原件未修改。

### G1

最终在创建任何任务文件之前调用Windows AllocateLocallyUniqueId预留身份；ID采用可读时间前缀+base36 LUID，唯一性不靠时间戳。调用失败明确返回，无新增包级状态、依赖、配置、retry/fallback；首版计数字段已删除。Windows保证本机重启前唯一，覆盖一个进程内所有管理器。最小客户端Windows XP、Advapi32.dll，Win7满足要求；[微软API依据](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-allocatelocallyuniqueid)。不保证跨系统重启的历史ID。

| 验收 | 状态与证据 |
|---|---|
| 快速启动≥50个且输出互不覆盖 | 实测通过：g1-native-win7.txt；50个不同ID、50条map记录，各输出对应各任务；额外50次重建管理器也不重复 |
| 原摘要用例20次 | 实测通过：g1-native-summary20-win7.txt，20/20、exit0 |
| 两条STARTED不同 | 实测通过：上述20次共40条STARTED、40个不同ID |
| tasks无退化 | 实测通过：g1-native-win7.txt及最终全量；原断言未改 |

原始时间戳方案第2次快速启动即重复（g1-red-win7.txt）。首版管理器计数虽单次50通过，但同进程20次摘要只有30个不同ID/40条STARTED；新增重建测试第2次复现（g1-recreation-red-win7.txt）。首版不能判G1完成，最终LUID补正另行验收。下方保留发现过程。

### G2

仅在已有紧急压缩失败分支区分errLocalContextBudget与真实端点超限，没有重构错误层级、压缩、截断或重试流程。

| 验收 | 状态与证据 |
|---|---|
| 本地预算不足不冒充端点响应 | 实测通过：g2-green-win7.txt，本地文案含retained_bytes=1361、budget_bytes=63，endpoint_requests=0，无“端点报告” |
| HTTP mock端点超限仍标端点 | 实测通过：同文件，HTTP400/context_length_exceeded、endpoint_requests=1，保留“端点报告上下文超限” |
| 网络韧性无退化 | 实测通过：同文件16项及最终双架构全量，既有断言未改 |

g2-red-win7.txt保留旧版0请求仍声称端点报告的失败。

### G3

原命令 `start "" /b ping -n 4 127.0.0.1 ^>nul` 改为 `start "" /b ping -n 4 127.0.0.1 >nul`，仅删除一个转义字符，断言不动。

| 验收 | 状态与证据 |
|---|---|
| 子进程存活数秒 | 实测通过：g3-command-v2-win7.txt，wrapper约0.0115秒返回，ping PID2272仍存活，约3.0853秒后退出；out.txt为空、ec.txt=0 |
| 原用例20次 | 实测通过：g3-20-win7.txt，20/20、exit0；最终双架构再各通过一次 |
| jobobject.go零diff | 实测通过：global-static-checks.json、product-diff.txt |

首个外部探针等到子进程继承管道关闭才枚举，未抓到存活PID而exit1（g3-command-win7.txt）；仅修外部采集方式后补证，失败记录保留。

### G4 v2

计数读取普通索引，保留用户Git配置，使用 `status --porcelain=v1 -z --untracked-files=all`，逐NUL记录解析；rename/copy的源路径不另计一条，路径换行不增加计数。stdout/stderr分离，status错误及不完整记录明确报错。计数在存储快照前采集，失败不继续静默产出数字。观察设置GIT_OPTIONAL_LOCKS=0，不刷新普通索引。

展示由 `dirty-files=N` 改为 `工作区相对 HEAD 的变更文件数=N`。内部临时索引add/write-tree/commit-tree与autocrlf=false字节保全不变。此字段包含已有改动，不是任务归属清单。

| v2验收 | 状态与证据 |
|---|---|
| 1. R1首次AUTO为0 | 实测通过：g1g5-r1-final.log；对应基线g1g5-r1-baseline.log为2524，两边初始2622个文件哈希一致 |
| 2. 模型改动后为1、基线成对 | 实测通过：post-model-verified-win7.json.txt，同一R1-final工作区旧版2524、新版1，普通status只有指定cs文件 |
| 3. 包含任务前N项 | 实测通过：g4-final-win7.txt，首次checkpoint前修改+暂存重命名+未跟踪文件共3项，计数3 |
| 4. 标签如实 | 实测通过：上述日志/工具结果和product-diff.txt均显示“工作区相对 HEAD” |
| 5. 存储字节不变 | 实测通过：同一修改后工作区两版tree均为5a5ccc5ad2304a36b37929e7f03e6a8d5d3396ad，普通索引哈希不变；首次AUTO tree均为6f7a35d8e5ea58600594c44cae60518e78dbeda5；另有CRLF blob与文件逐字节断言 |
| 6. status失败明确报错 | 实测通过：g4-final-win7.txt，普通索引损坏报git status/exit128/index file smaller than expected，无计数结果 |
| 7. 不按CombinedOutput行数计数 | 实测通过：product-diff.txt；NUL、rename/copy、换行及不完整格式用例通过 |
| 8. checkpoint/rollback无退化 | 实测通过：g4-final-win7.txt的19项及最终全量 |

修改后成对checkpoint由外部HTTP mock请求两版实际产品的checkpoint工具，输入是同一份真实模型已改完的工作区，**不是额外真实模型R1**。产品两次均exit0，避免两次模型译文差异干扰存储比较。VM持久会话为g1g5-20260910/post-baseline.jsonl与post-current.jsonl；原始请求与工具结果在本地post-model-checkpoints-win7.json.txt。

后置采集器首次按session文件名猜基线ref而查错，采集器exit1；3303809早于F3，实际task_id为t0911-005519-883。随后只读提取已保存工具结果中的实际ref并核对tree，未重跑模型或checkpoint挑结果。原错误记录保留。CRLF首夹具普通status自身报2、diff内容为空，未满足干净克隆前提（g4-diagnose-win7.txt）；改为真实克隆并先断言status为空，保留计数0断言后通过。cmd引号导致的exit255也单独保留，不算测试运行。

### G5

实测通过：g5-win7.txt四个子用例，write/edit按最终目标deny均拒绝且文件未变；按junction别名deny均不命中并实际改到最终文件。permissions.go/pathpolicy_windows.go零diff。DELIVERY-DOC.md与api-contract.md说明 `C:\work\shared` junction指向 `D:\shared` 时，write/edit应配置D:/shared/*，别名pattern不命中。没有新增别名匹配行为。

### 全局与真实模型

最终双架构从干净170945c构建，Go1.20.14/CGO_ENABLED=0/GOOS=windows。final-full-amd64-win7.txt与final-full-386-win7.txt各138项、exit0，0失败/0跳过。最终所有日志在VM本次启动2026-09-10 00:58:42.109999+08:00之后；使用新日志、新会话，未拿旧日志充数。宿主机go vet两架构exit0是静态检查，不冒充端侧执行。

| 真实模型运行 | 结果与证据 |
|---|---|
| R1基线3303809 | 实测通过：6轮、read2/edit3、exit0，仅指定GlobalExceptionFilter.cs改变；g1g5-r1-baseline.log/.jsonl及meta/spec/audit |
| R1最终170945c | 实测通过：3轮、read1/edit4、exit0，同样仅指定文件变化；g1g5-r1-final.log/.jsonl及meta/spec/audit |
| S1最终170945c | 实测通过：4轮、ls1/read1/edit1/shell1、exit0，仅calc.py变化；工具输出each pays: 30.0，独立Win7执行亦exit0；g1g5-s1-final各文件及s1-independent-win7.txt |

两份R1初始2622个文件哈希逐项相同（summary.json）；基线源码通过git worktree add --detach建立baseline-3303809，从干净3303809构建，随后git worktree remove移除。没有新分支，没有复用旧TEMP源码归档。构建命令与哈希见build-record.md。

g1g5-r1-current.*是G1补正前f784733产物的额外中间运行，3轮exit0，完整保留；最终源码变化后重跑R1，不是挑好结果。前一轮137项全量也保留，但不作为最终138项的替代。两次模型翻译措辞不同，不用轮次差异宣称性能提升。端点均为aigc789.top/v1、deepseek-v4-flash，提示词沿用历史，没有调整预算或轮次上限。

**R2未重跑：** 按任务书免除；G1只改身份，G2只改文案，G4只改checkpoint计数/展示，ctxcompress.go及netresilience.go零diff，截断/压缩策略未变。历史窄配置不收敛结论不变。

全局静态核对实测通过：global-static-checks.json显示go.mod/go.sum/jobobject.go零diff，既有测试仅jobobject_test.go命令一字符变化；原tag快照完全一致，rc-0.11仍指ed3731a。所有新增行为对应G1/G2/G4要求，G3只测试，G5只文档和测试。未实现阶段三HTTP/SSE/静态资源或分发包。

两份外来未跟踪nongit-checkpoint文档未提交，构建时短暂改名到现有忽略路径并在finally恢复，内容哈希完全一致（build-file-preservation.json）。本批次使用新annotated rc-0.12，指向本报告的最终归档提交，annotation列出未满足项；不移动原tag、不push。创建后核对tag目标、agent tree及全部旧tag对象。

---

## 历史执行记录（以下“暂停/待验收”是当时状态，不代表最终状态）

## 2026-09-10：归档完成，规格核对暂停

唯一开发树：`E:/codex-worktrees/win7-agent/pre-phase3`，分支 `pre-phase3`。本次未修改产品源码或测试，未运行新测试，未创建tag，未push。

| 项目 | 状态与证据 |
|---|---|
| 上轮 lifecycle 增量收尾 | 实测通过：commit `003b759`，仅13个 artifacts 文档/诊断证据文件，无源码；`git show --stat 003b759` 可复核 |
| G1–G5 任务书原样归档 | 实测通过：commit `d88eb56`，仅 `artifacts/decisions/pulse7-G1-G5-Task-Node.md`；源文件与复制文件 SHA256 均为 `c5c8540bdeaf38426d9360131ddd0cfa4bff89a35e05fe22012f68dea693f85a` |
| G1–G5 实现与各项验收 | 未验证：规格核对发现下述 G4 冲突，按任务书 §0.5 暂停，未开始源码修改 |
| 全量双架构、真实模型 R1/S1、rc-0.12 | 未验证：尚未执行，不提前打tag |

## 待裁决：G4 的统计含义与时间点

任务书同时要求：

1. dirty-files 表达“本次任务在工作区内改动的文件数”；
2. 计数使用普通索引的 status，即用户 git status 的基准；
3. R1 验收期望 dirty-files=1。

现有调用链实读：`agent/tools.go:1081` 在可变操作前调用 ensureAutomaticCheckpoint；`:252` 调用 CheckpointAs，`:258` 输出 AUTO checkpoint。`agent/gittools.go:214–223` 在 CheckpointAs 内计算并返回计数。因此，首次修改前的计数不可能知道随后会改几个文件。

历史证据 `artifacts/f1f4-report.md:215` 及 `f1f4-evidence/r1-current-b.log`、`r1-baseline-b.log`：两个R1在执行前普通status为空，首次自动checkpoint报2524，任务结束后只有一个文件改动。将计数直接换为普通status且保持调用时点，则第一次AUTO应报0；修改后再次读取普通status才为1。这与“同一条首次AUTO应为1”的理解不同。

普通status还包含任务开始前已有的staged/unstaged/untracked改动，不能普遍等价于“本次任务改动”。若坚持任务归属计数，需要定义任务开始基线及差异规则；这些行为不在任务书当前最小改法中，不能自行加入。此处只是规格分析，未实现任何新基线或后置采集机制。

建议裁决为：dirty-files表示“采集时工作区按普通索引status显示的变化文件数”，允许包含任务开始前已有改动；保持checkpoint时点，R1首次AUTO期望0，修改后另行验收计数为1。若仍要求任务归属计数或首次AUTO输出1，请补充具体采集时点与基线规则。等待裁决前不执行后续源码修改。

## 仍未满足项

- G1–G5 尚未实现及验收，rc-0.12尚未创建。
- S3精确问题数仍为既有判据问题。
- 默认CLI与rc-0.7存在30 vs 100轮次上限差异。
- R2窄配置不收敛；本轮未重跑。
- 已有真实模型证据来自aigc789.top，不是内网端点。

## v2裁决后恢复执行

用户已裁决，前述G4阻断解除。归档v2 commit `273af97`，SHA256 `ecd94286c9f5ebb1221e1d93d47eaebdd8a4b017a333ad312d9333767e366226`；v1保留于d88eb56。变更点：普通索引status是采集时工作区相对HEAD的变化计数，包含已有改动；首次AUTO为0正常；展示文案同步更正；任务归属清单留作阶段三契约。

G1 `6dcb84f`，G2 `31a2d42`，G3 `fb40b65`，G4 `1277383`，G5见后续最终提交清单。针对性Win7证据已保存在g1g5-evidence；全量双架构及真实模型回归尚待执行，本段不声称最终完成。

中间构建二进制保留为g1g5-evidence/*.exe.tmp（现有*.tmp忽略规则），仅改本地制品名称，不删除内容；受测VM文件仍为原exe名。最终构建也使用忽略路径，确保构建前git status为空。

### G1最终核对补正（2026-09-11）

首版6dcb84f仅保证单管理器内序号唯一。复核同一进程的20次摘要日志，40条STARTED仅30个不同ID；新增管理器重建用例在第2次启动复现重复（g1-recreation-red-win7.txt）。因此首版不能判G1完成。
最终改为在创建文件前调用Windows AllocateLocallyUniqueId分配LUID，以时间前缀+base36 LUID构成文件名。系统负责原子分配，本机重启前唯一，涵盖一个进程的所有管理器；不新增包级变量、依赖、配置或retry/fallback。失败明确返回。原递增字段已删除。
依据：https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-allocatelocallyuniqueid （最小客户端Windows XP，Advapi32.dll；本机重启前唯一）。本节点不承诺跨重启的历史ID唯一性。
补正后的快速启动50个、重建管理器50个、tasks回归通过；摘要20/20且40条STARTED均不同（g1-native-win7.txt、g1-native-summary20-win7.txt）。由于不得改写历史，G1补正以额外独立G1 commit保留，未amend原提交。前一轮f784733双架构137通过保留，但不冒充最终补正后的全量证据，接下来重新构建验收。

## 最终仍未满足项（本报告收尾）

- 未验证：S3精确问题数判据，保持既有判据问题结论。
- 未验证：默认CLI与rc-0.7逐行一致，30 vs 100轮次差异仍在。
- 未验证：R2窄配置收敛，历史仍不收敛，本节点按任务书不重跑。
- 未验证：内网速度与端点行为；真实模型证据全部来自aigc789.top，不是内网端点。
- 未验证：默认预算保底组超限阈值、hard link专项checkpoint/rollback影响。
- 本次任务文件改动归属清单为阶段三契约待办，未实现；其余未修项见g1g5-findings.md及api-contract.md。
