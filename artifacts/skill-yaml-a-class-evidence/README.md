# SKILL.md YAML 解析与 A1–A4、前端 ⑤/⑰ 的 Win7 真机证据

对应提交：`626af5d`（①②③ YAML 解析）、`9aa384c`（A1）、`4671708`（A2）、`2bcdf7a`（A3）、`bc88522`（A4）、`7b05790`（⑤）、`81eff42`（⑰）、`1baed0a`（嵌入重建后的 Web 包）。

## 被测对象与环境

- 机器：`WIN-65VKOKP8G13`，`Microsoft Windows 版本 6.1.7601`，LastBootUpTime `20260916102121.125599+480`（本次两轮均在该启动窗口内，未重启、未做任何宿主机/客户机电源操作）。
- 产品二进制：`pulse7-amd64.exe`，sha256 `d309294d651b3e71d3f7b7a8f0a1e1f3747fa9f4efc100d2d63e7622ea9e7b63`，与 `dist/pulse7.exe`、`dist/pulse7-1baed0a-amd64.exe` 逐字节相同（上传前后哈希比对 `HASH_MATCH True`）。
- 模型：**本地受控 SSE 夹具，不是真实 LLM**。夹具只负责发起一次 `read` 工具调用并结束回合；不能据此声称模型行为或内网端点已验收。
- 运行前删除 `data/`；`agent.log` 中的 `=== session start ===` 时间戳为 `2026-09-17T10:13:27/27/28+08:00`，落在本次运行窗口内（`log_fresh: true`）。

## 两次运行

- `run-20260917-100133.json`：**PRODUCT_PASS False**。原因是本地夹具缺陷——工具调用块后发 `finish_reason=stop`，产品按 `agent/netresilience.go:117` 正确判为「finish_reason=stop received with an unfinished tool call」并中止回合，`read` 因此没有执行。不是产品缺陷，保留该文件作为过程留痕。
- `run-20260917-101329.json`：夹具改发 `finish_reason=tool_calls` 后重跑，**PRODUCT_PASS True**，18 项检查全部为真，`requests == 3`。

## 真机确认的事实（第二次运行）

| 项 | 证据 |
| --- | --- |
| ① YAML frontmatter | CRLF + 注释行 + 空行 + 引号 name + 折叠 `>` 多行 description + 列表 + 额外键的 `SKILL.md` 解析为 `{"name":"yaml-review","description":"Review code carefully."}` |
| 全局根目录 | `global-db` 以 `scope":"global"` 出现在目录中 |
| ② 跳过原因 | 告警为 `跳过 skill …\broken\SKILL.md：frontmatter 缺失：首行必须是 ---` |
| 目录不含坏包 | `broken` 不在 `session_init.skills` / catalog 列表中 |
| 正文不入上下文 | 三次请求的 JSON 中均无 `BODY_NOT_FOR_CATALOG` / `GLOBAL_BODY` |
| 安装契约 | `[pulse7:skill-installation:v1]` 含两个根目录、并写明 YAML frontmatter 契约 |
| A2 | 模型 read 被跳过的包时仍发出 `skill_loaded {"name":"broken","path":".pulse7\\skills\\broken\\SKILL.md"}`，判定来自安装根目录而非发现列表 |
| A3 | 一个回合内只有一次 `skill_catalog` 事件（`count: 2`），read 不再触发二次扫描 |
| A4 | 文本模式续跑中 `[警告] 跳过 skill` 恰好出现一次且带原因；stream-json 模式下告警只在人类文本流出现，事件流里是 `skill_catalog.warnings` |
| ⑤ / ⑰ | `serve` 返回的 `assets/index-1427a3c6.js` 含 markdown-it（`Wrong \`markdown-it\` preset`）与告警去重文案（`条告警与上一轮相同`），说明真机跑的就是重建后的嵌入包；页面交互本身未人工验收 |

## 测试二进制结果

`skill-yaml-amd64.test.exe -test.run 'Test(Skill|MalformedSkills|ReadAllowsPersonalSkill|SessionInitUsesEmptySkillsArray|EventConsumers|StreamJSON)'`：21 个顶层用例 + 4 个子用例通过，退出码 1，唯一失败为 `TestSessionInitUsesEmptySkillsArray`。

该用例只在本机 Win7 失败：它不隔离 `USERPROFILE`，Win7 真实个人目录下存在 `C:\Users\user\.pulse7\skills\code-review\SKILL.md`，于是 `skills` 非空。属测试隔离缺陷，不是产品行为变化；按本轮裁决不动测试夹具，已记入 `artifacts/h-findings.md`。

## 未验证 / 不声称

- 真实 LLM、内网端点、生产灰度均未跑；本目录只覆盖夹具驱动的回合。
- Web 页面未做人工交互验收，只核对了嵌入包内容与 `/` 可服务。
- 386 二进制（`dist/pulse7-1baed0a-386.exe`）未在真机上跑过，本次真机只跑 amd64。
- A1（个人目录不可解析）只有单元测试 `TestSkillDiscoveryContinuesWithoutHomeDirectory` 覆盖：产品在 `agent/config.go:142` 启动阶段就要求 `os.UserHomeDir()` 成功，没有产品级路径能走到该分支。用户 2026-09-17 裁决该问题不再调整。
- 未做凭据轮换，未在任何文件中写入 VM 口令；口令经环境变量传入 `run-win7.py`。

## 目录内容

- `README.md`、`build-hashes.json`：本文件与构建溯源（随仓库提交）。
- `run-20260917-100133.json`、`run-20260917-101329.json`：两次真机原始报告（含全部事件与请求体）。
- `run-win7.py`、`verify-win7.py`、`pulse7-amd64.exe`、`skill-yaml-amd64.test.exe`：harness 与二进制，按既有 `.gitignore` 规则不入库，留在本机工作树。
