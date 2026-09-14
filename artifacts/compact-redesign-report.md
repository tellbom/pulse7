# 两级上下文压缩实施与验收

日期：2026-09-14。开发树：E:/codex-worktrees/win7-agent/pre-phase3，main，基线 be8e130。构建来自未提交工作区，包含此前大内容与配置修复，不能声称来自干净提交。前端未改、未构建；未提交、未打 tag、未 push。

## 实现

1. 原有 requestContextChars（消息 JSON 字节 + 工具定义 JSON 字节）、cfg.maxCtx 和 0.65 触发判断保持不变。没有读取 usage，也没有模型上下文探测、cache 字段、stream_options 或时间触发。
2. 新增 micro_keep_recent，默认 8；CLI 为 --micro-keep-recent，GET/PUT /api/config 可见并持久化、任务间生效。API 接受 1–1000。内部零值配置按默认 8 处理。最新完整 assistant/tool 组始终保留，实际保留结果数可超过 N。摘要阶段原有最近 3 个工具轮次保持不变。
3. 白名单 read/ls/tree/grep/glob/shell/task_output。只改历史 tool 消息 Content，保留调用 ID、工具调用结构和近期内容。小于 1024 字节的结果不清理，占位符不小于原文时也不替换。write/edit/checkpoint/权限与任务控制结果不属于本轮白名单。
4. 结果通过现有会话索引单遍查找，按已存 toolOutcome 与原文生成本地占位符。路径/范围只来自可解析参数，不猜 shell 影响路径，不声称文件已理解或任务已完成。
5. 复用 lc1 附件；无附件时先保存原文，成功后再清正文。保存或完整性校验失败的条目保留正文并输出跳过原因。不覆盖历史 JSONL。内容寻址的稳定附件 ID 支持重复压缩与 resume 复用。
6. micro 达到原阈值直接继续主循环，不请求模型；仍超阈才进入原有摘要资格判断。摘要使用 micro 前快照，避免先清后忘。结构为目标约束、确认事实、文件符号与重要性、实际修改、验证、未解决问题、当前工作、下一步。
7. 摘要改为一次 /chat/completions 流式调用，沿用 llm_compress_timeout_sec 独立超时。无自动重试、不请求工具、不把摘要流混入 assistant_delta。空摘要、坏流、超时回退原有截断；用户取消不回退继续。工具返回、长度截断及正文超本地预算被拒绝。
8. 当前待压缩历史的工具结果索引单独外置（包括尚未 micro 的结果），摘要/截断时保留一个有界系统指针，内含按需读取入口；索引记录不依赖模型准确抄写。该指针也计入原有预算。历史索引可链式引用，当前上下文不重复嵌入所有旧记录。
9. compaction.method 新增 micro，CLI 单独显示“微压缩”；事件携带 originalBytes/discardedBytes。审计沿用 _compress 与 method，原有 truncate 组明细仍保留。

## 验证

所有运行均在 Windows 7 Ultimate，192.168.140.130:22；启动时间 20260913201629.932215+480。使用新进程直接抓取输出，没有复用旧端侧日志。amd64 产品与测试程序的本地/端侧 SHA256 一致，见 compact-redesign-evidence/build-hashes.json。

- 首次压缩专项：16 PASS / 0 FAIL / 0 SKIP，exit 0。
- 补充并行工具组、配置测试及 CLI 展示后，首次 amd64 全量：182 PASS / 0 FAIL / 0 SKIP，exit 0，78.347 秒。相对之前 175 项新增 7 项；原有协议与紧急单次重试断言保留，仅把摘要端点夹具改成 SSE。
- amd64 / 386：Go 1.20.14，CGO_ENABLED=0、GOOS=windows，build、test -c、vet 均 exit 0。386 未执行测试。
- Win7 实际 serve HTTP/SSE，端点仅接受 stream=true 且不返回 usage：CASE_MICRO 19 次主模型请求、4 次 micro、0 摘要；CASE_SUMMARY 19 次主模型请求、2 次摘要，并走预算不足下的截断，最终成功。这证明请求和回退链路，不证明真实模型摘要质量。
- HTTP 首跑：CASE_MICRO 通过，CASE_SUMMARY 第 14 次主请求后等待事件超时。脚本未持续读取产品 stdout 管道。补充排空后原二进制复跑通过；首次失败完整保留。证据支持管道阻塞解释，但未捕获首跑进程阻塞栈，不宣称已通过栈取证确诊。

原始日志与测试脚本在 artifacts/large-content-evidence/：compact-focused-first.log、compact-full-first.log、compact-http-first.log、compact-http-drained.log、compact-full-win7.py、compact-http-win7.py。它们是本地验收制品，哈希索引另存。没有删除或覆盖首跑日志。

## 未验证与限制

- router RBAC→Go 同任务、同模型、同预算的真实模型对照尚未执行；不能承诺重读次数、费用或耗时已下降。实录中的路径错误与模型行为仍可能独立造成重复探索。
- 不以纯模拟模型结果证明内网端点或 DeepSeek 行为。本轮未使用真实 API key。
- 摘要质量仍由模型决定，结构化提示不是结构化字段的强制正确性保证。模型可能未保留重要事实，原文引用用于恢复。
- 本地恢复索引覆盖当前上下文中可关联已存会话记录的工具结果，并非整个项目的权威文件目录；无法关联的历史条目不能编造状态。前次索引以引用链接保留。
- 附件留存、历史索引链回收沿用大内容模块现有策略，没有自动 GC。首次索引和哈希仍需扫描，不能声称零 I/O。
- 386 运行、Sandboxie、GUI 浏览器联调本轮未做。前端所需接入点见 compact-redesign-frontend-handoff.md。

本轮没有同时修改轮数上限策略，max_rounds 语义维持原状。

## 收尾复查补记

恢复索引补齐未 micro 的历史结果后，compact-final-full.log 实测 180 PASS / 2 FAIL，exit 1。失败项为 TestContextOverflowCompressesOnceAndRetriesCurrentRequest、TestContextOverflowStopsAfterOneEmergencyRetry，均报读取 session.jsonl 不存在。原夹具只有内存历史，而生产 pushMsg 会先写会话。仅在 runOverflowTurn 中补齐相同历史的 sess.record，不削弱原断言、不绕过存储错误；补齐后专项 18 PASS / 0 FAIL，exit 0。最终全量与 HTTP 另记录，不用初版通过替代最终版本。

compact-final2-full.log：182 PASS / 0 FAIL / 0 SKIP，exit 0，119.744 秒。随后增加角色限定，只替换 system 恢复索引，用户引用相同标记的消息保持原样，并在现有摘要测试中补断言。该最终候选命名 compact-release-*；文件名不代表已经发布或打 tag。

## 最终候选结果

- compact-release-full.log：182 PASS / 0 FAIL / 0 SKIP，exit 0，116.536 秒。测试 SHA256 为 6d78d71e4f58059572290c1ddc5dbdf86f34b01cdd134661abca1f8700e8ff68。
- compact-release-http.log：PASS。每个场景各 19 次主模型请求；micro_keep_recent=4 的 160000 字节场景触发 4 次 micro、0 次摘要；64000 字节场景累计 2 次摘要及截断，正常结束。默认 8 的保留行为由专项测试验证。这些是同一最终产品二进制的实测，端点不返回 usage，仅接受流式请求。
- 产品 compact-release-amd64.exe SHA256：9852c11bf32d162a45ff0e3eaa6de77cba7226494502b711fa188c28ddf54db2，本地与端侧一致。
- 上述结果不抹去 earlier compact-final-full.log 的两项夹具失败和 HTTP 首次超时；其表现、修正范围与复跑已分别记账。
- 当前用户前端在持续更新，候选 exe 包含构建当时的静态资源，不代表最新 GLM 前端已经联调。前端合包请基于当前前端与本轮后端源码，不用后端验收结论替代 GUI 验收。

后端实现、确定性测试和 Win7 产品进程链路验收完成。真实模型同任务对照、GUI、386 运行和 Sandboxie 仍未验证。工作树保持未提交，未打 tag 或 push。

启动窗口披露：全量脚本记录 LastBootUpTime=20260913201629.932215+480；续接后的 HTTP 验收核对记录为 20260913205016.748378+480（compact-release-vm-window.log），两次返回不同，不推测 VM 恢复或时钟原因。各运行时间均晚于对应记录的启动时间，HTTP 为续接后新发起进程并重新核对同一产品哈希；不宣称它们发生在同一个启动窗口。

