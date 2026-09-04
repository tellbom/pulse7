# M3 冻结清单（2026-09-05）

## 最终二进制

| 制品 | SHA256 |
|---|---|
| win7-agent.exe（M3 最终，含全部修复） | `4e333b1e1c8a63cf64f6e93c480100791fff7369bb26dfaf4a1ceeadaa913be1` |

其余制品（Go 1.20.14 / MinGit 2.46.2 / Sandboxie Classic 5.73.2 / go-openai v1.42.0）沿用 M2 冻结（freeze/M2/）。

## M3 新增交付（agent/ 新文件，M2 核心零改动）

- `config.go`：config\agent.json（flag > config > 默认）；`init` 子命令生成模板
- `envdetect.go`：`doctor` 子命令 + selectSandboxMode（两种运行结果：Sandboxie / JobObject(auto-degraded)）
- `sandboxiecfg.go`：box/OpenFilePath 幂等生成（经 SbieIni.exe，非管理员可写）+ `/reload`；卸载时删 box 节
- `cleanup.go`：每次 shell 后 box terminate、会话结束 terminate+delete_sandbox_silent、启动清理 >1h 陈旧 wrapper
- `installer/install.cmd`、`installer/uninstall.cmd`：安装/卸载生命周期
- mock 增加 `M3-SMOKE` 触发器（read→write→checkpoint→shell→rollback，相对路径）

## M3 期间发现并修复的缺陷（全部真机回归）

| 缺陷 | 修复 |
|---|---|
| SESSIONNAME 在计划任务上下文为空 → 会话类型误判 | 改用 ProcessIdToSessionId（0=无桌面） |
| /listpids 探针在"已安装但服务死"下假阳性（/silent 吞错误且退出码撒谎） | 探针改走**生产 wrapper 机制**：容器侧必须出现 PROBE-OK 输出才算可用 |
| 探针对象只填了 StartExe/Box（Home/Workspace 空 → 相对路径落 System32 被拒） | 补全字段；doctor 增加 probe detail 诊断行 |
| Win7 无嵌套 job：sshd 会话内 Assign 被拒 | 降级为无 job 执行 + 超时杀直系子进程 + 输出告警 |
| 卸载脚本在服务死时调 Start.exe 会挂 | sc query RUNNING 守卫 |
| 111MB 目录删除 3 秒竞态残留 | 延迟删除循环重试 ×6 |
| 硬复位丢失未落盘注册表写入（自动登录失效） | 软重启策略 + 注册表复查 |
| 传输后的 .cmd 曾被清零（VM 磁盘疑似瞬时异常） | 部署后强制哈希/内容校验 |
| 挂死计划任务阻止同任务名新实例（读到旧日志误判） | 每轮换任务名+日志名，先清挂死实例 |

## Gate 结论（真机，日志 artifacts/m3-*）

| Gate | 环境 | 结果 |
|---|---|---|
| M3-A | 桌面会话 + 服务正常 | ✅ Sandboxie 全链路（A-EXIT=0） |
| M3-B | 双服务 disabled + 重启（=未打补丁等价态） | ✅ JobObject(auto-degraded) 全链路；doctor 同判；零补丁话术 |
| M3-C | 受限令牌 trustlevel 0x20000（普通用户等价） | ✅ Sandboxie 全链路（INNER-EXIT=0） |
| M3-D | 会话 0 凭据任务（headless） | ✅ JobObject 全链路（D-EXIT=0） |
| 生命周期 | 降级环境安装→运行→重启→再运行→卸载 | ✅ 全程通过；ws4/ws-life 用户数据完整；Sandboxie 本体保留；仅目录删除竞态已修复 |
