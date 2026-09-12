# 用户裁决：Win7 兼容性优先（2026-09-11）

裁决方案：兼容性优先于完整进程树控制。pulse7 在 Win7 的 Session Job 启用 `JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK`，仅对主动加入 Session Job 的直接进程提供确定性生命周期管理；第三方程序创建的后代允许自由脱离并使用自身 Job Object。脱离后的进程不属于 pulse7 的确定性清理范围，不视为 Agent 异常。pulse7 可进行低成本尽力清理，但不得为了回收完整性再次限制第三方程序的原生进程行为。
