# 证据索引（2026-09-15 补验）

产生树：`E:\codex-worktrees\win7-agent\pre-phase3`，源码状态见 `build-hashes.json`。运行机器：Win7 SP1 `192.168.124.3:2222`，独立目录 `C:\Users\user\plan-recall-validation-20260914`。

| 文件 | 用途 / 结果 |
|---|---|
| `build-hashes.json` | 原候选构建输入与输出哈希；新增 fixture_correction 记录测试夹具修正；win7_validation 与 evidence_sha256 索引 |
| `plan-recall-amd64.exe` | 实际 Win7 受测产品，SHA256 9053b4db1e2beb523711e648ec564e53055ce21f7bda7f62edef75e3541f880f |
| `plan-recall-amd64.test.exe` | 原测试程序；首次 6 过 1 失败，不再冒充最终测试程序 |
| `plan-recall-final-amd64.test.exe` | 修正夹具后的受测程序，SHA256 80a5e381481a32841753ce1d97c96809759347279d799d43fbf66a148369abeb |
| `plan-recall-386.exe` | 仅构建，未运行 |
| `captured/focused-first.json` | 首次专项完整失败现场；exit 1，6 过 1 失败 |
| `captured/focused-final.json` | 最终专项完整输出；exit 0，7 过 |
| `captured/full-final.json` | 全量一次完整输出；exit 0，189 过、0 失败、0 跳过 |
| `captured/plan-http-1789399996/` | 第一版 HTTP 脚本把 SSE resultRef 误作内联 result 的失败证据 |
| `captured/plan-http-1789400067/` | 第二版 HTTP 脚本误要求窄预算必须 summary 的失败证据；实际为 truncate |
| `captured/plan-http-1789416895/` | 最终 HTTP/SSE 四组通过；本地流式模拟端点，不是真实模型效果验证 |
| `captured/data/sessions/` | 实际持久化会话、模式记录、审计、lc1 附件 |
| `captured/index.json` | 152 个收回文件逐一列路径、字节数、SHA256 |
| `vm-probe.json` | 原地址 SSH 失败历史，保留；不用于描述当前地址状态 |
| `win7-prepare.log` / `win7-boot.log` | 新地址系统启动时间、Python/Git runtime、旧 worker 盘点 |
| `cleanup.log` | 用户授权后终止旧 worker PID 3152，exit 0；没有删除首次证据 |

维护脚本：`win7-regression.py` 校验上传哈希、记录 boot、捕获真实测试 exit。`runtime-http-verified-win7.py` 为最终运行时脚本；前两版脚本原样保留。`collect-win7.py` 通过 SFTP 收回原始文件，`cleanup-old-workers.py` 只处理已核实路径/命令形态的旧 worker。

每次 HTTP 运行新建 workspace/home，每个 case 新建会话；无真实 key。每份测试结果的本次时间戳均晚于所记录启动时间。完整落实、首次失败和剩余范围见 `../plan-recall-report.md`、`../plan-recall-findings.md`。


## 2026-09-15 planning decision validation

- decision-build.json / decision-source-snapshot.zip: 未提交工作树源码输入与二进制哈希，不能当作干净 HEAD 构建。
- decision-captured/index.json: 新增证据逐文件 SHA256，含两次运行时请求、会话和日志。
- decision-focused-first.json: 首次 4 项通过；decision-full-final.json: 最终全量 192 通过、H2 一项失败；decision-h2-isolated.json: 隔离 H2 一项通过，首败仍保留。上述文件位于 decision-captured/。
- decision-http-1789435498: 首次候选后端链路通过；decision-http-1789435660: 最终候选链路通过、8 请求，均位于 decision-captured/。
- runtime-decision-win7.py: 流式模拟协议验收，不是真实模型收敛评估。
- 详细解释见 ../plan-recall-report.md 的计划状态与显式问答章节。
