# H0–H7 执行报告

## 最终收尾与 rc-0.13（2026-09-12）

以用户最终裁决 decisions/H-Final-Acceptance-Deferrals.md 为本轮验收范围。下面是最终状态；后文原报告、失败日志、曾经的“未打 tag”进度描述保留为历史，不能当作最终状态。

**amd64 最终全量：实测通过。** 从干净提交 `9edc6ed6d9e1856291542872542257609608277f` 构建，仅在新 Win7 VM 执行一次，无筛选参数：149 个顶层用例通过，0 失败、0 跳过，exit 0。启动窗口 boot 20260911231151.111597+480；运行 2026-09-12 04:21:18 至 04:22:22 +08:00。证据 rc013-amd64-full-win7.txt、rc013-final-summary.json。

构建工具 `E:/win7-agent/tools/go/bin/go.exe`（go1.20.14），工作目录唯一开发树的 agent/，CGO_ENABLED=0、GOOS=windows。完整命令、构建前后空 git status、提交号及保全/恢复记录见 h-evidence/rc013-clean-build.json。两份用户未跟踪 nongit 文档临时改名至同目录忽略路径，构建完成后原位恢复，逐字节 SHA256 一致；未修改用户内容。仅构建完成后写入证据文件。

| 构建产物 | SHA256 |
| --- | --- |
| amd64 测试程序 | 9e975d2a866807be76faa7672a1f1a16ee9451aa626afeb6ade152ba9ba3c110 |
| amd64 产品 | 818fa3a793c24fd69e91a44727f8747f13ba787365724197230cb821c23e9b30 |
| 386 产品（仅构建） | 9d35c6dd869a5082572934e7a04929dbbab4d185ca4c7096a0099827099cb3ae |

VM 测试程序运行前 SHA256 与本地一致；产品上传 SHA256 由 certutil 核对，见 rc013-product-upload-hash.txt。最终报告提交仅改文档/证据，agent/ 与干净构建提交零差异；rc-0.13 是该源码加收尾记录的快照。go.mod/go.sum 相对 rc-0.12 零差异，已有 tag 保持不动，不 push。

### 未满足项与延期项（必须随 tag 披露）

- **386 实测：推迟至真实用户实测。** Win32/32 位构建与支持继续保留，本次产品构建成功；不运行 386 全量或专项复测。历史时序失败与隔离通过照原样保留，不因延期改判失败或通过。
- **Sandboxie 分支实测：推迟至真实用户实测。** 本轮不运行沙盒链路，不将 JobObject 的结果外推给 Sandboxie；保留两种模式的语义差异。
- H 节点没有补齐新官方模型的 S1/S2/S3/R1/R2 整组回归，仅有真实 shell 冒烟与确定性工具/CDP 场景。最终单元/集成测试程序全量结果不能替代未跑的真实模型场景。
- Job 分配失败的受限模式目前保留注入失败测试证据，不冒称新 VM 实际处于拒绝分配的外层 Job 环境。
- F/G 历史 R2 窄预算不收敛、S3 判据、CLI 默认 30/100 差异等未被本节点重新验证或消除，沿用原报告；内网速度与内网端点行为仍未实测，当前真实模型来源为 DeepSeek 官方。
- Win7 后代脱离是最新裁决下的预期行为：只对直接加入 Job 的成员提供确定性清理。后台+detached 由用户决定关闭；单独 detached 可能持续等待输出管道，本轮保留该语义，不保证穷尽登记/回收所有后代。

rc-0.13 annotation 同步列明上述限制；不将此快照等同于未执行场景的验证结论。未开启阶段三，不制作分发包。

## 2026-09-12 用户裁决落实：background + detached

最新裁决见 decisions/H-Background-Detached-User-Control.md。后代脱离是预期行为；长期保留程序采用 background=true、detached=true，立即返回任务标识，把查看、关闭选择交给用户。不为输出管道仍被持有而强制终止后代或增加任意 WaitDelay；单独 detached 的前台等待语义保留。386 时序波动暂停追加复测，保留旧日志，后续 UI 阶段由用户实测观察。

本次仅修改 shell 工具说明以明确两种参数组合，不自动重写参数，不修改进程实现或 Sandboxie。本地 Go1.20.14 amd64 构建成功；以下真机执行用的是既有后端产品 SHA256 0781b439a9a5c17412fc8ea0d29da7407054c170148b29cebe1e4523f8759613，不冒称新措辞已由真实模型验证。

**#2b 按新裁决实测通过**：新 VM 原始证据 new-vm-background-detached-1.txt 包含三次产品运行（均 exit 0）：

1. 第一条 shell 显式 background=true、detached=true，返回 task_id；后续两条独立 shell 取得 Chrome 目标列表并完成 CDP 鼠标点击，浏览器使用默认沙箱。测试第二条命令包含固定 readiness 等待，不能将其算作产品启动延迟或产品重试。
2. pulse7 退出后 Chrome 仍存活；重启读取 detached 清单，CLI 与模型上下文均显示“未经核实”，没有自动终止。
3. 测试显式模拟用户关闭，通过 task_kill 的 PID/creation_time/image_path 三元组核对并返回 process terminated。只据此证明匹配的 Chrome 进程被终止，不声称全部后代都被回收。

**脚本失败完整披露**：原测试脚本最后在精简 CLI 文案中寻找完整的 process terminated 字符串，断言失败，脚本 exit 1。完整工具消息已经记录成功返回。复核 new-vm-background-detached-1-review.json 从同一份原始记录检查三个产品 exit、task_id、后续工具结果、鼠标交互、历史上下文及终止响应；这是证据复核，不是另一次运行。已修正脚本查验工具消息的位置，未重跑挑选结果。

按用户裁决，H-F07 不再作为上述后台组合的阻断项；前台 detached 可能持续等待仍如实保留。H 节点其他未完成验收不由本条外推；本次不打新 tag、不 push。

## 最新续跑：新 Win7 VM，JobObject 专项（2026-09-11 夜间）

本节覆盖旧表中本次已补测项目的状态，但不删除旧失败证据。用户已授权新 VM `192.168.140.128:22`；本轮仅 JobObject，Sandboxie 链路延后，不安装/修改 Sandboxie、不改系统补丁。Win7 Ultimate SP1 x64，boot `20260911231151.111597+480`，Chrome 109.0.5414.120；证据 new-vm-inventory.txt。验证目录 `C:/Users/user/h-validation`。

最新语义以 decisions/H-Win7-Silent-Breakaway-Decision.md 为准（归档 0e234d3，实现 71f59df）：仅直接加入会话 Job 的成员保证清理，第三方后代自由脱离，不视为异常。原任务书 #4 的整树保证、#2 退出后必须关闭 Chrome 的旧附加判断不再适用；detached 的显式意图不变。

| 本次项目 | 状态 | 新 VM 结果 / 证据 |
| --- | --- | --- |
| amd64 H 定向组 | 实测通过 | new-vm-h-focused-amd64.txt，12 个顶层用例，exit 0；含 20 次启停、四条退出路径、身份核对、输出事件与告警 |
| 386 H 定向组 | 实测通过 | new-vm-h-focused-386.txt，12 个顶层用例，exit 0；四退出路径按实际 SessionManaged 成员查进程表，非成员不冒报被收割 |
| amd64 JobObject 组 | 实测通过 | new-vm-jobobject-amd64.txt，7 项，exit 0 |
| 386 JobObject 组 | 未验证 | new-vm-jobobject-386.txt，6 通过、1 失败，exit 1；固定 1.5 秒等待后缺少 sentinel，见下文 |
| 上述失败用例隔离复跑 | 实测通过 | new-vm-jobobject-386-isolated.txt，仅 1 次，exit 0；不能覆盖原组失败，不修改等待时间 |
| amd64 / 386 tasks 定向组 | 实测通过 | new-vm-tasks-amd64.txt / new-vm-tasks-386.txt，各 10 项，exit 0；含输出硬限、启动握手、退出摘要、后台增量输出与终止等既有用例 |
| #2 Chrome 默认沙箱三独立 shell | 实测通过 | new-vm-cdp-silent-2.txt，curl 列表、CDP 鼠标点击后 dataset.clicked=yes；没有 --no-sandbox；pulse7 退出后 Chrome 存活且 sessionManaged=false，随后仅由测试脚本 Browser.close 收尾 |
| #2b detached 完整场景 | 未验证 | 旧 VM detached-silent-1 超时保留；新 VM 最小复现确认 worker 仍可能等后代管道，不把普通 Chrome 场景外推为 detached 通过 |
| 新 VM DeepSeek 官方真实 shell | 实测通过 | new-vm-real-shell-1.log / .jsonl / -meta.json：deepseek-flash，echo JOB-OK，工具 exitcode=0、产品 exit 0，工作区零文件变化；不是 S1/S2/S3/R1/R2 全量模型回归 |
| #16 干净提交双架构全量 | 未验证 | 本次是定向组；测试构建时存在未提交制品，不称为干净提交全量。未打 rc-0.13、不 push |

### detached 超时定位

new-vm-worker-pipe-probe.txt：直接命令在 2 秒采样时已完成标记、写出 parent-output，但 worker 尚未退出，status 文件不存在；约 8.35 秒后（ping 后代结束）worker 才 exit 0。第二份 new-vm-worker-pipe-probe2.txt 再核对 WMIC：2 秒采样时 worker 已无直接子进程，约 8.45 秒才完成。

源码 agent/task_worker.go 使用非文件 writer 接收 stdout/stderr，再调用 cmd.Run()；Go os/exec 在直接子进程退出后仍等待复制输出的 goroutine 到 EOF。实测和源码一致，支持后代继承输出管道导致 worker 等待，不能再归因于 Session Job 禁止 Chrome 沙箱。本轮先记录，不自行加任意 WaitDelay、吞掉输出错误或改 Sandboxie 的共享 worker 行为。后续修复须明确直接命令完成与后代输出采集的生命周期，不能以强制关管道破坏第三方行为换取假通过。

### 386 首次失败

TestJobObjectForegroundChildSurvivesUntilSessionClose 的原断言未动。组运行在 1.66 秒报告 session-child-finished.txt 不存在；隔离运行 1.70 秒通过。此结果支持时序波动线索，不能证明根因，也不能将整个 386 组改判通过。原日志与隔离日志分别留档。

### 环境与构建证据

Python 3.8.10 嵌入版能运行普通脚本，但导入 _socket 报 DLL load failed；保留 new-vm-cdp-silent-1.txt。隔离部署 Python 3.7.9 后 socket/ssl/http.server 导入成功；无系统补丁变更。Git runtime 来自只读历史树 dist/rc-0.7/pulse7/runtime/git 的只读副本，版本 2.46.2。curl 官方 Windows 8.22.0_1 明示支持 Win7，下载 ZIP SHA256 与官方匹配：7f23b039f6ea4197362d4468e1a0e71428201222e1bef3b680d5ef7b2aefb714，来源 https://curl.se/windows/；仅用于测试。

Python HTTPS 探针曾报本地 issuer 缺失，保留 new-vm-official-models.json；随后 Go TLS 探针验证成功返回未认证 HTTP 401，真实产品携 key 调用亦成功。因此不将 Python 探针失败称为产品 TLS 失败，不关闭 TLS 校验、不修改系统证书。key 仅经 getpass / stdin / 子进程环境传入，不入制品。

VM 记录的产品 SHA256：0781b439a9a5c17412fc8ea0d29da7407054c170148b29cebe1e4523f8759613；amd64 测试：9d6b902c1eeb410402f572bc49625cd4ac14490836da1a3efd207328be616954；386 测试：20c96f09210ee4bc3e88cd4de1de17ce0ddcca29d13857dd28783e22477744791。源码实现为 71f59df；amd64 产物延续上一轮构建，386 本轮构建；以上不冒称全量干净构建验收。

2026-09-11；基线 rc-0.12 / `2b0a0c774087882862bf519d666f0bb6102d1484`；唯一开发树 `E:/codex-worktrees/win7-agent/pre-phase3`，分支 pre-phase3。

## H0

实测通过（静态源码清点）：任务书全文已读取，逐项源文件核查见 `h-inventory.md`。JobObject 会话归属、挂起分配恢复、后台表/工具及输出上限已存在，不能重复实现。缺口包括原生进程身份、普通前台/后代登记、detached 跨会话清单和身份核对、输出主动事件等。清点本身不是运行验收。

## 实施边界（2026-09-11 用户裁决）

H1/H2/H5 仅作用于 JobObject 路径；Sandboxie 保留现有 box 管理、超时整箱杀和 cleanupOnExit 行为。差异记入 `h-findings.md` H-F01，待真实使用反馈后再评估对齐。按 H1→H7 恢复实施；不擅自关闭 Sandboxie 或削弱既有断言。

## 验收状态

| 项 | 状态 | 证据 |
| --- | --- | --- |
| #1 H0 清单 | 实测通过 | h-inventory.md 各行源码位置与清点基线 |
| #3 超时整树终止 | 实测通过 | h-evidence/h2-process-table-win7.txt：实际进程表；后续干净提交全量另计 #16 |
| #4 四条退出路径 | 实测通过 | h-evidence/h5-four-exits-fixed-win7.txt：normal / exec-error / ctrl-c / panic，进程表无残留，后台结束事件为 killed |
| #13 连续 20 次启停 | 实测通过 | h-evidence/h13-cycles-win7.txt：20 次前台 + 20 次后台，每次终止后 Job 计数 0，exit 0 |
| #15 两个 lifecycle 用例 | 实测通过 | h-evidence/h15-process-count-20-win7.txt 与 h15-exit-summary-20-win7.txt，各 20 次，失败 0/20，均 exit 0；未改两个原测试 |
| #2、#2b、#5、#6、#7、#8、#9、#10、#11、#12、#14、#16 | 未验证 | 已有部分实现和定向证据，不等于整项验收完成；#2 见本次诊断 |

H1–H7 尚未完成，不创建 rc-0.13，不移动已有 tag，不 push。未开启阶段三。

## 2026-09-11 续跑：实现状态与证据边界

H1–H7 已分别提交实现（35df6a6、0d5fb63、c0e8a3e、99dd7d6、75a6cf3、02d7657、e5fe45b）；48aa9e7 补 H5 收割后后台结束事件及四种退出路径验收。实现提交不等于验收完成。

- H1：JobObject 受限状态进入、后续轮次及任务列表持续显示。
- H2：原生 taskkill 失败可见，超时用实际进程表确认整树终止。
- H3：原生 PID / 创建时间 / 映像路径登记及生命周期事件。
- H4：detached 持久化、跨会话未经核实提示、终止前同一进程句柄核对三元组。
- H5：后台输出偏移主动通知；会话收割后等待后台结束事件，避免把 killed 报成 exited。
- H6：任务数量、时长及 Job 进程数量告警事件，保留非阻断语义。
- H7：api-contract.md 追加数据、操作和事件约定；没有实现阶段三。

本次 h-cycles.exe 的 VM SHA256 为 `12c739547239bc6f4e389b6ee6b470337b7983ae974a99fbb24c787a59850dbc`，在 HEAD 48aa9e7 加未提交 h13_cycles_test.go 的状态编译，不能作为 #16 的干净提交证据。20 次循环与两个 lifecycle 20 次结果均在本次启动窗口内（VM boot 20260910005842.109999+480），每次由 win7-run.py 删除对应旧日志后运行。旧二进制与新二进制未混称。

### CDP 验收 #2 诊断

1. cdp-session-1：测试目录漏装 bundled Git，自动 checkpoint 拒绝三次 shell，浏览器未执行。测试装配错误，原日志保留。
2. cdp-session-2：补齐 Git 后 Chrome 确实启动，但浏览器主进程提前退出，curl exit 7。事件表显示剩余 Chrome 子进程在会话退出时收割。
3. cdp-session-3：同条件保留 Chrome stderr，六次 `GPU process launch failed: error_code=18` 后 `GPU process isn't usable. Goodbye.`。
4. chrome-baseline-1：独立于 pulse7 Job 的 Chrome（headless、disable-gpu、专用 profile）5 秒后仍存活，CDP 返回页面；随后仅终止该测试进程树，exit 0。该对照页面为 about:blank，而产品场景为本地按钮页，不冒称所有参数逐字相同。
5. cdp-inprocessgpu-1：仅测试命令增加 --in-process-gpu，浏览器存活且 curl exit 0，但 /json 返回空列表，实际交互未完成。

以上证据表明当前启动方式存在 Job 内外差异，但 `error_code=18` 本身不足以定性为 AssignProcessToJobObject 失败。Chromium 源码将 18 定义为 CREATE_PROCESS 错误，20 才是 ASSIGN_PROCESS_TO_JOB_OBJECT 错误：[sandbox_types.h](https://chromium.googlesource.com/chromium/src/+/2bd75c72c147bbd6acbb9d541a2959f5b06a9c06/sandbox/win/src/sandbox_types.h)。尚未取得该 VM Chrome 对应版本的内部 Windows 错误码，不声称已经证明精确失败 API。

正式 #2 仍为未验证。未放宽 pulse7 Job 的 breakaway 属性、未改 Chrome 产品默认参数、未用 detached 代替普通会话进程验收。

6. cdp-nosandbox-diagnostic-1：只在测试命令增加 Chrome --no-sandbox，三条独立 shell 均 exit 0，CDP 执行 mousePressed/mouseReleased 后页面 dataset.clicked=yes；pulse7 退出后 CDP 端点不再可达，脚本 exit 0。产品 SHA256 `d7434b9d17850dbca73871954bff45230dceb64977d8aa54c9ae4b98c817b11d`。该运行是诊断，不将其 passed=true 字段等同于正式 #2 通过。

**待裁决的具体验收差异**：是否允许 #2/#2b 使用明确记录 --no-sandbox 的本地测试页面场景？这保留 pulse7 会话 Job 的收割要求，但关闭 Chrome 自身沙箱，与默认 Chrome 行为不同。若不允许，应保留 #2 未验证并单独确定 Win7 Chrome 沙箱与会话 Job 的兼容方案。不能自行放宽 Job 的 breakaway / SILENT_BREAKAWAY 来通过，否则后代可能脱离会话收割范围。依据任务书末尾“需要改动本文件未列出的产品行为才能继续时，停下报告”，在裁决前不做此类产品修改，也不打 rc-0.13。

真实模型配置按用户新指令改用 DeepSeek 官方 https://api.deepseek.com；仅已有 /models HTTP 200 连通证据（official-models.json），选择响应内的 deepseek-flash。尚无官方端点 S1/S2/S3/R1/R2 完整运行证据；历史 aigc789 结果仍保留其原始来源，也不能顶替内网实测。密钥不写入制品。

### 用户裁决与 Chrome 109 对应源码核对

用户明确暂不允许 --no-sandbox 直接作为 #2/#2b 正式通过条件。两项保持未验证，以上待裁决问题已有此答复，不再将关闭 Chrome 沙箱作为验收捷径。

VM 实测 chrome.exe 文件版本为 109.0.5414.120（chrome-version-win7.txt）。核对同版本 Chromium [target_process.cc](https://chromium.googlesource.com/chromium/src/+/refs/tags/109.0.5414.120/sandbox/win/src/target_process.cc)：141–145 行在存在沙箱 Job 且系统早于 Win8 时，给创建标志增加 CREATE_BREAKAWAY_FROM_JOB；150–159 行调用 CreateProcessAsUserW，失败则保存 GetLastError 并返回 SBOX_ERROR_CREATE_PROCESS。对应 sandbox_types.h 定义该错误为 18。原版源文件留存于 h-evidence/chrome109-*.txt。

pulse7 的 StartSession 当前只设置 KILL_ON_JOB_CLOSE 和可选 JOB_MEMORY，未设置 BREAKAWAY_OK / SILENT_BREAKAWAY_OK。微软 [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects) 明确 Win7 不支持嵌套 Job，并说明禁止 breakaway 与子程序自建 Job 可能不兼容。

因此当前证据强烈支持：冲突是 Chrome 自身沙箱需要脱离父 Job、自建 Job，而 pulse7 为保证统一收割禁止脱离；不是 Chrome 被 shell 返回时提前收割。错误可能发生在创建子进程阶段，不能称为已经实测到 AssignProcessToJobObject 失败。尚未捕获 Chrome 本次运行保存的原始 GetLastError 数值，准确 API/错误码链条仍属于版本源码结合运行对照的推断，而非 API 跟踪实测。此次不改实现、不放宽 Job 策略。
