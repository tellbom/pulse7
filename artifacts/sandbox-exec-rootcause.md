# T1 根因报告：Sandboxie 沙盒内 python 无法执行

## 确定的根因

**T4 重命名（win7-agent → pulse7）漏改了 `runner.go`**，导致 wrapper 写入路径与结果读取路径不匹配。

### 证据链

**runner.go 第 24 行（buildRunFiles）—— 写入侧仍用旧名：**
```go
dir := filepath.Join(home, ".win7-agent", "run", id)  // ← .win7-agent
rel := `.win7-agent\run\` + id                         // ← .win7-agent
```

**sbx.go 第 63 行（Run 的容器读取）—— 读取侧已用新名：**
```go
ctnDir := filepath.Join(s.SandboxRoot, userName(), s.Box, "user", "current", ".pulse7", "run", rf.id)
//                                                                              ↑ .pulse7
```

### 因果链

```
buildRunFiles 写 wrapper 到 %USERPROFILE%\.win7-agent\run\<id>\
→ wrapper 的 out.txt/ec.txt 写入沙盒容器的 .win7-agent\ 路径
→ agent 从容器的 .pulse7\ 路径读取
→ 找不到文件 → readResult 返回 ec=-1
→ 模型看到 exitcode=-1 → 以为命令失败 → 重试 → 循环 21+ 次
```

### 排除过程（先查后改，未猜）

| 实验 | 结果 | 结论 |
|---|---|---|
| SSH 直调 Start.exe + python | 全部超时 | SSH = 会话 0，Sandboxie 需要桌面会话，该路径无效 |
| /it 任务 + 直调 python --version | `Python 3.8.1` ✅ | python 本身**可以在沙盒内运行** |
| /it 任务 + 手工 wrapper（写 .pulse7 + 读 .pulse7） | `each pays: 30.0`, `ec=0` ✅ | wrapper 机制在沙盒内完全正常 |
| /it 任务 + Go 二进制复刻 agent 路径（写 .pulse7 + 读 .pulse7） | `each pays: 30.0`, `ec=0` ✅ | Go 调用 Start.exe 没有问题 |
| 源码审查 | runner.go 用 .win7-agent，sbx.go 用 .pulse7 | **路径不匹配 = 根因** |

### 为什么此前测试通过

M3 Gate A / RC 0.1 / RC 0.2 时期，`runner.go` 和 `sbx.go` **都**用 `.win7-agent`，路径匹配，一切正常。T4 重命名时 `sed` 只改了 `cleanup.go sbx.go sandboxiecfg.go`，漏了 `runner.go`，从此写入和读取分裂到两个不同路径。

### 影响

- Sandboxie 模式下**所有 shell 调用**都返回 exitcode=-1（不只 python）
- JobObject 模式**不受影响**（不走容器路径映射，wrapper 直接在真实文件系统读写）
- 这解释了为什么 R2（探索任务）能收敛——R2 的 shell 调用在部分场景走了降级模式或只用了 read 工具

## 修复

一行改动：`runner.go` 中 `.win7-agent` → `.pulse7`（两处）。

**不需要** Sandboxie 程序例外（方案 A）、不需要验证命令走 JobObject（方案 B）、不需要仅提示（方案 C）——环境本身没有问题，是路径 bug。

## 对此前判断的修正

- RC 0.3 findings 说"python 在 Sandboxie 沙盒内无法执行"——**结论错误**。python 可以在沙盒内运行，只是 agent 读不到结果。
- T5（retrieval findings）说"S2 异常是 type 乱码 + 验证循环"——**部分错误**。真正的因果链是：agent 的 shell 返回 exitcode=-1 → 模型以为改错了 → 尝试用 type 回读验证 → 看到乱码 → 更确信改错 → 循环。type 乱码是次要症状，shell 路径 bug 才是起因。
