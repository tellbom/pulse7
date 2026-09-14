# pulse7 Web GUI — handoff 对接与 DeepSeek 正式测试报告（v3）

日期：2026-09-13。开发树 `E:\codex-worktrees\win7-agent\pre-phase3`。本轮依据 `artifacts/gui-backend-frontend-handoff.md`（2026-09-13 后端交接清单）完成前端对接，并以真实 DeepSeek 端点完成正式端到端测试。

## 一、handoff 五项对接（web/ 前端，全部完成并回归）

| 清单项 | 实现 | 验证 |
|---|---|---|
| §1 工具历史状态三态 | renderHistory 改为 unknown 起始；role=tool 的 `toolOutcome{ok,summary,errorCode}` 落定成败（hard_link_impact_unknown→拒绝态）；缺失 toolOutcome 保持"状态未知"，不猜测 | mock 历史含成功/失败(exit1)/无结果三类 + 真实会话恢复后 4 工具全"成功"与实时一致 |
| §2 推理流 | `assistant_reasoning_delta` 按 attempt 与正文分别累积；discard 连带清除；助手消息新增独立可折叠推理区；历史从 `reasoning_content` 恢复；正文/推理/工具调用同消息共存 | mock：推理→正文、discard 后推理+正文零残留；真实 deepseek-v4-pro：实时 2 个推理区 + 恢复后同样 2 个 |
| §3 工作区切换 | 串行守卫（switching 禁重复切换/发送）；代次校验（旧异步响应不覆盖新页面）；PUT 成功才更新路径，失败保持原状；409 区分 busy/background_running 双横幅（中断按钮 vs 查看后台任务）；全量状态清理（timeline/attempts/sessionId/pendingPermission/context/skills/checkpoints/taskOutputs 等） | mock 触发 background_running：横幅文案、按钮形态、路径保持全部正确 |
| §4 上下文估算 | 不硬编码；读 `GET /api/config.max_ctx`（设置页显示"256,000 字节 ≈ 64,000 估算 token"）；warningLevel 以后端 context_state 为准（缺失时按 65%/90% 推算） | 设置页字节/token 双单位标注；顶栏/输入卡/状态栏随事件变色 |
| §5 Skills 文案 | 目录来源/新开会话生效/无商店/loaded 含义 | 设置页文案更新 |

顺手修复：手动路径切换成功后下拉未自动关闭（TopBar applyManual）。

## 二、DeepSeek 正式端到端（真实端点，全证据链）

环境：宿主机 `pulse7-e2e.exe serve`（新后端+新前端，386，`http://127.0.0.1:65130`）；模型 `deepseek-v4-pro`；端点 `https://api.deepseek.com`；工作区 `%TEMP%\pulse7-ws-demo`。

| 步骤 | 结果 | 证据（文件/DOM 双层） |
|---|---|---|
| 真实连接测试 | `✓ 连接成功 · deepseek-v4-pro · 3396ms` | DOM |
| 发任务"列出文件→读 readme→写 summary.md" | **已完成（4 轮 10s）** | serve 日志"任务结束：success" |
| 真实工具链 | ls→glob→read→write 全成功（10ms/8ms/8ms/1.9s） | DOM 工具记录 + 会话 JSONL 的 tool_call/role=tool |
| 真实文件落盘 | `summary.md` 生成，中文要点正确 | 磁盘文件内容核对 |
| 推理流（真实） | deepseek-v4-pro 返回 reasoning_content，界面 2 个可折叠推理区（英文思维链如实呈现） | DOM + 截图 + 展开文本 |
| 刷新恢复一致性 | 恢复后工具状态/toolOutcome/推理区/最终回复与实时完全一致 | DOM 对比 |
| 会话/审计持久化 | `data\sessions\sess-t0913-*.jsonl`（含 toolOutcome）、`audit.jsonl` | 文件存在且内容核对 |

截图归档：`artifacts/phase3-web/real-machine/real-09-deepseek-e2e.png`（恢复视图：4 工具记录 + 推理区展开 + write 参数面板）。

## 三、本轮新发现的问题（转后端/后续）

1. **会话列表单文件放大故障（建议修复）**：`GET /api/sessions` 对任一会话 readHistory 失败即整表 500。实测一份含 10.9MB 单行记录的历史会话（超出 1MiB 单条上限，合规拒绝）导致全部会话不可见。建议列表对单条目失败降级（跳过或逐项报错）。该文件已移入 `data\sessions\quarantine\` 隔离（连同说明：文件本身 JSON 合法，仅单条超限）。
2. **serve 重启后工作区回退默认**（cwd"."）：恢复会话按契约校验工作区不匹配被拒——行为符合契约，但向导/设置里可考虑持久化"最近工作区"以减少手工重切。
3. 推理区标题当前为"推理过程（端点提供）"；真实 reasoning 为英文时如实展示，不做翻译（不装作知道原则）。

## 四、复现

```bash
cd web && npm run build && npm run deploy:agent && cd ../agent && go build -o ../pulse7-e2e.exe .
./pulse7-e2e.exe serve   # 读控制台端口 → Chrome 打开 → 向导/设置填 DeepSeek → 发任务
```

VM（192.168.140.128）侧：`C:\pulse7\pulse7.exe`（旧版构建）与 mock:8080/serve:49817 会话已过时，可按上节同法重建部署。
