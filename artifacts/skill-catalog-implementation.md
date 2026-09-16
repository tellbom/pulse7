# 技能目录预算与任务边界刷新实施记录

日期：2026-09-16。基线 HEAD：9bcdd92；本轮改动尚未提交。工作树：E:/codex-worktrees/win7-agent/pre-phase3。

## 实现范围

- 全局与工作区两层，不做会话私有技能，不做上传/删除管理接口，不改前端。
- 保留完整技能元数据，取消发现时 200 rune 裁剪和 20 项截断。工作区同名覆盖全局，并给出警告。
- skill_catalog_budget_bytes 默认 8192；现有配置分层、CLI 参数、GET/PUT /api/config、全局持久化均接入。API 有效范围 1024–1048576，任务忙时 409。
- 独立目录 system 消息按实际 JSON UTF-8 字节计量。full/shortened/names/index 逐级降级；UTF-8 安全裁剪带省略号。目录只有名称、描述、作用域、路径，不注入正文。
- 超限时将完整元数据写入内容寻址索引，复用 read/grep，不新增模型检索工具、不调用额外 LLM。
- CLI/HTTP 共用 streamTurn 入口，在用户任务开始刷新；任务工具循环中不刷新。替换当前请求目录，不持续追加，不改历史工具结果。恢复后也在任务边界重建；旧标准 system 目录后缀迁移。
- skill_catalog 事件公开版本、模式、数量、预算、实际字节、索引路径与警告。元数据版本不代表正文文件版本。

源码：agent/skills.go、skillcatalog.go、config.go、main.go、api_storage.go、api_server.go。测试：skills_test.go、skillcatalog_test.go。前端交接：skill-catalog-frontend-handoff.md。

## 验证

工具链 E:/win7-agent/tools/go/bin/go.exe（go1.20.14），CGO_ENABLED=0，GOOS=windows。

| 检查 | 结果 |
|---|---|
| amd64 产品构建 | 完成，exit 0 |
| amd64 go test -c | 编译完成，exit 0；不等于测试执行通过 |
| amd64 go vet ./... | 完成，exit 0 |
| 386 产品构建 | 完成，exit 0；未执行 386 测试 |
| Win7 针对性运行 | **实测通过**：WIN-65VKOKP8G13 / Windows 6.1.7601；36 个匹配测试，30 个 PASS（含子测试），0 FAIL，exit 0 |
| 真实模型选择/读取技能 | 未验证；不能从静态改动推出调用率改善 |
| GUI 配置与事件展示 | 未验证，前端尚未接入 |

新增测试覆盖完整元数据保留、超过 20 项、正文不注入、四种预算模式、索引完整性、UTF-8 截断、刷新/删除/工作区隔离、旧目录替换、同名覆盖、索引写失败、API 校验与持久化/重新应用配置、busy 拒绝、已有 read 对索引分页，以及上下文 accounting、压缩阈值、压缩失败回退、紧急重试、micro compact、GUI 工具历史。Win7 实测 36 个匹配测试（30 个 PASS，含子测试，0 FAIL，exit 0），原始输出见 `run-20260916-104537.json`。

证据目录 artifacts/skill-catalog-evidence：测试运行脚本、SSH 探测、构建哈希。其中 exe 与临时 Python 运行脚本匹配仓库既有忽略规则；README 与 JSON 证据保留在目录中，没有把二进制加入 Git。构建来自本轮未提交源码，不能标成干净 9bcdd92 构建。

历史阻断已解除。测试期间未重启宿主或 VM；`vm-identity.txt` 记录主机名、版本和本次启动时间。构建哈希与上传哈希一致（`run-20260916-104537.json` 中 `HASH_MATCH=True`）。此前 SSH 探测失败仍保留在 `ssh-probe.json`，不作为本次结果。

## 2026-09-16 Win7 真实 exe 运行态验证

`real-exe-win7.py` 在 Win7 上启动本轮构建的真实 `pulse7-amd64.exe`，本地 Python OpenAI 兼容 SSE 端点保存每次请求 JSON；没有使用真实模型，因此该项验证的是消息投影和恢复刷新，不是模型调用率。

- 两次真实 exe 均 exit 0；捕获 2 个请求，首轮和 `--resume` 第二轮各一个。
- 首轮 system 消息含 `old catalog description`，且不含 `PRIVATE_BODY_SHOULD_NOT_BE_IN_INITIAL_CATALOG`：模型请求只注入技能元数据，没有注入 SKILL.md 正文。
- 修改同一工作区技能描述后，以同一 session `--resume` 再启动；第二轮 system 消息含 `NEW catalog description after refresh` 且不再含旧描述：恢复会话在任务边界刷新工作区目录。
- 首轮的 `session_init.skills` 还显示同一工作区的元数据，`skill_catalog` 事件显示 `mode=full,count=2,budgetBytes=8192,listingBytes=970`。这两类事件分别表示发现快照和模型目录投影，不混称技能已使用。
- 真实 exe 哈希为 `48ed22d427a6da74ac427a65b182d923b862c2a1349cd190a91050126e7dc150`；远端返回的哈希和本地相同。

证据：`real-exe-win7-run.json`（完整首轮/恢复轮输出与断言）、`real-exe-win7.py`（可复现脚本）、`run-20260916-104537.json`（Win7 测试运行及哈希）、`vm-identity.txt`（身份/启动时间）。

## 保留边界

- 总上下文阈值、压缩策略与估算口径不变，不依赖 usage、不自动探测模型窗口。
- 目录稳定到任务结束，不锁定技能正文；任务内外部直接修改正文后，read 读到的是读取时的文件。这与元数据版本是两个事实。
- SKILL.md 发现阶段仍读取文件以解析 frontmatter，正文不进入初始模型目录。未更换 YAML 解析器。
- 索引按元数据内容寻址，同版复用；旧版暂不自动清理，以免破坏历史引用。未来清理策略需考虑仍被引用的索引。
- 未实现监听器、实时待刷新状态或已用正文版本冻结；前端不能把本轮描述成具备这些能力。
- 请求投影替换不改写历史会话记录；当前版本/预算通过任务事件展示，历史记录不会倒写成新版。
- 用户若配置目录预算大于可用整体窗口，既有整体预算保护仍可能拒绝请求；本轮未放宽该保护。
- 仍未验证：真实 DeepSeek/内网模型对目录降级的选择行为、前端 GUI 展示、386 Win7 运行、Sandboxie 路径、实时文件监听、上传/删除接口。未提交、未打 tag、未 push、未替换 dist 正式部署。
