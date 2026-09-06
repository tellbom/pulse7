# shell 中文编码根因报告（T1，只读排查）

- 日期：2026-09-06｜分支：encoding-pagination（rc-0.4 基线）｜对象：pulse7.exe = rc-0.4（`b0e835e6…f106`）
- 环境：Win7 专业版 SP1 x64（VM 192.168.124.3:2222），系统区域中文，控制台代码页 936（GBK）

## 结论（先说答案）

**中文 shell 输出乱码：确认，两种沙盒模式都坏；且命令方向（中文进命令行）也坏。**

| 方向 | 现象 | 证据 |
|---|---|---|
| **输出**（GBK→读作 UTF-8） | cmd/程序输出的中文（chcp 提示、dir 中文文件名、`type` 中文报错"系统找不到指定的文件。"）以 GBK 字节写入 out.txt，`readResult` 用 `string(outB)` 按字节转字符串（[runner.go:45-49](../agent/runner.go)），无任何转码 → 进入模型上下文即乱码；写入 session 时经 Go json.Marshal 把非法 UTF-8 字节替换为 U+FFFD，**信息不可逆丢失** | `enc-t1-plain.jsonl` tool 消息：`'\ufffd\uedaf\ufffd…: 936'`、dir 行 `C:\uat \ufffd\u013f\xbc`；GBK 回转只能得到问号 |
| **命令**（UTF-8 命令→GBK 解析） | `buildRunFiles` 把模型下发的命令（UTF-8）原字节写进 inner.cmd（[runner.go:31](../agent/runner.go)），cmd 按当前代码页 936 解析 → 含中文路径的命令变成 mojibake 路径 → `dir C:\uat\08-中文项目` 解析失败（exitcode 0 变 1，列出的是父目录/报错） | 同上 session：dir 结果 exitcode=1 且内容错乱；对照组（console CP=65001）同命令 exitcode=0 且正确列出中文目录 |

**`echo 中文测试ABC` 看似正常是侥幸**：UTF-8 字节按 GBK 成对解析后再按 GBK 输出，字节对双射使原始字节原样往返（E4B8AD E69687… → 涓枃… → 同样字节），readResult 读回"恰好正确"。不适用于任何输出由 cmd/程序按自身逻辑编码的场景（dir、错误消息、编译器输出）。

## 为什么一直没发现：测试台架掩盖了 bug

本项目所有真机测试（UAT、各轮回归）都用 PowerShell 启动器调 pulse7，其中一行
`[Console]::OutputEncoding=[Text.Encoding]::UTF8` 会把所在控制台代码页改为 65001，
wrapper 的 cmd 子进程继承 65001 → inner.cmd 的 UTF-8 命令被正确解析、out.txt 按 UTF-8 写出 → 全链路"正常"。
真实用户从纯 cmd（默认 936）启动则必现乱码。**对照组实测**：

| 启动方式 | console CP | chcp 输出 | dir 中文目录 | session 内容 |
|---|---|---|---|---|
| PS 启动器 + OutputEncoding=UTF8（旧台架） | 65001 | 干净 UTF-8 | exitcode=0 正确列出 | 全部合法 UTF-8 |
| 同启动器去掉该行（本次受控组） | 936 | U+FFFD 乱码 | exitcode=1 路径解析失败 | 乱码+有损 |

## 取证过程（字节级）

1. JobObject（SSH，受控启动器，`enc-t1-plain.jsonl`）：
   - `chcp` → `'exitcode=0\n[sandbox=JobObject]\n\ufffd\uedaf\ufffd\ufffd\ufffd\ufffd\u04b3: 936'`（"活动代码页: 936"的 GBK 字节）
   - `echo 中文测试ABC` → `'\u4e2d\u6587\u6d4b\u8bd5ABC'`（侥幸往返，见上）
   - `dir C:\uat\08-中文项目` → exitcode=1 + 全乱码（命令方向坏导致列错）
   - `type C:\uat\不存在的中文文件.txt` → 中文错误消息全乱码
2. Sandboxie（schtasks /IT 交互会话，`enc-t1-sbx.jsonl`）：chcp/dir 同样乱码（`[sandbox=Sandboxie]` 行伴随 U+FFFD）——**两种模式同一 `readResult` 路径，输出侧结论一致**。
3. wrapper 临时目录（.pulse7/run/<id>/）用后即删（runner defer），持久证据即 session jsonl；audit.jsonl 与 agent.log 同样记录乱码（同源）。
4. 裸 cmd 基线（`cmd /c echo … > file`）曾得到双重转换字节 `E6 B6 93 EE 85 9F …`，系 PS2.0 按 ANSI 读无 BOM 脚本 + Console CP 混杂所致，仅用于理解机制，不作为产品结论。

## 影响面

- 中文目录的 `dir`、中文错误消息（`type`/`xcopy`/`net` 等）、Go/编译器在中文路径下的报错——模型收到 U+FFFD 有损文本，无法判断、易猜错重试（与 UAT 观察到的 type 回读乱码引发 re-edit 循环同类，只是位置更根本）。
- 模型下发含中文路径/参数的命令时（dir/type/copy 目标为中文路径），命令本身解析失败——**即使输出修好，命令方向不修照样用不了中文路径**。T2 必须双向修复。

## 附带发现（记 findings，不在本轮修）

- Sandboxie 模式偶发 `start.exe failed: exit status 0x40010004`（本次 echo/type 两条未跑起来，chcp/dir 正常）——与 RC0.3.3 报告的 Sandboxie R2 1/3 收敛不稳定可能是同一现象，待专项。
- `type 不存在的中文文件` 在部分运行中错误消息为英文（"The system cannot find the file specified."）另一轮为中文——VM 的 MUI/区域设置混合，不影响结论（两轮乱码判定一致）。
