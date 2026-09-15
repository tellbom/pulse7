# 计划问题与用户退出：前端接入说明

2026-09-15。后端新增能力；本轮不修改前端源码。原有普通聊天、权限确认接口保持原语义。

## 状态与问题

- 新工具 `ask_planning_decision({question})` 仅在计划模式下可用。问题持久化到当前会话计划日志，生成独立 decision ID，然后结束本轮为 `need_answer`。同批后续工具仍回填失败结果，避免产生缺失 tool response 的请求。
- SSE `planning_decision`：`{sessionId, decision:{id, question, awaitingReply:true}}`。按 sessionId 隔离并显示完整问题，不使用问号或关键词推断这一类问题。
- `GET /api/plan`：`{sessionId, initialized, state}`；state 为 `{active,path,decision?}`。无选择会话时 state=null。已 resume 但尚未启动 runtime 也可读回持久化状态，不启动模型或 Job。运行中返回 409，先使用已有运行状态与事件；结束后读取。initialized 表示状态已可读取，不表示进程已启动。
- decision 可包含 `replyUuid`、`replyPreview`、`cancelled`。`awaitingReply=false` 只表示不再等待；有 replyUuid 表示已收到回复，cancelled 表示用户退出取消等待。均不表示问题已解决或计划已获批准。完整答复由既有 messages 接口读取，preview 最多 2048 字节。

## 回复

`POST /api/answer`，JSON：`{"sessionId":"当前会话","decisionId":"当前问题ID","answer":"用户原文"}`。

成功 202 后通过原 SSE 渲染下一轮；后端原样保存用户文字，单独生成运行时事实附件。错会话返回 409 session_mismatch；过期、已回复、已取消或未提供正确 ID 返回 409 decision_mismatch。存在待答计划问题时直接 `/api/turns` 返回 409 plan_decision_pending，不把普通输入猜成回答。旧普通聊天问答不要求 decisionId。

前端必须在 need_answer 后展示关联答复入口。请求失败保留草稿并刷新状态，不能仅凭点击把问题显示为解决。用户可以回复“不知道”或追问；是否还需澄清由后续模型判断，系统不代判。

## 用户直接退出

`POST /api/plan/exit`，JSON：`{"sessionId":"当前会话"}`。仅空闲时执行，运行中先由用户调用已有 `/api/interrupt` 并等待本轮结束。成功 200 返回 `{sessionId,state}`，同时 SSE `plan_state` 返回 `{sessionId,state,source:"user"}`。

动作只解除阶段限制并取消待答问题，不自动发起模型请求，不批准计划质量，不解除全局只读和其他校验。维持现有 exit_plan_mode 的文件存在与合法路径要求；计划文件尚未落盘等返回 409 plan_mode，展示原错误。用户下一条任务由原输入框提交。未成功前不可乐观切换状态。

CLI 对应 `/plan`、`/plan-answer <decision-id> <回复>`、`/plan-exit`；非交互执行遇到此问题输出 need_answer 并以 AWAIT-USER-ANSWER 退出，交互 resume 后可继续回答。

## 验收边界

后端 Win7 HTTP/SSE 模拟端点验证与 GUI 用户操作是两项证据。前端接入后需实际验证：刷新恢复待答问题、切换会话不串答、错误回复仍待答、取消不冒充回答、运行中先中断再退出、恢复会话可重新显示历史文字。本文件不宣称这些 GUI 行为已完成。
