# 技能解析器与 A 类问题标记（仅定位，不含修正方案）

- 仓库：`E:\codex-worktrees\win7-agent\pre-phase3`
- 定稿时 HEAD：`d8bcf55 feat: add skill installation roots contract and evidence`（分支 `main`，工作区干净）
- 目标位置（建议）：`artifacts/skill-parser-and-a-class-issue-markers.md`（`.gitignore` 白名单允许 `artifacts/**/*.md`）
- 范围：**只标出代码问题所在的位置与性质**；本文不包含修正方案、不提出实现路线、不改动任何源码。
- 复核方式：所有引用均为 `文件:行号`；可用纯只读命令逐条确认（见第 4 节）。

---

## 1. YAML 解析器：自研 vs 现成工具类

### 标记 ①　解析器本体（自研，未使用 YAML 工具类）

`agent/skills.go:101-126`（`parseSkillFrontmatter`）

```go
func parseSkillFrontmatter(content string) (string, string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) < 4 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" { closed = true; break }
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return "", "", false     // ← 任何一行没有冒号即整条失败
		}
		...
	}
	return name, description, closed && name != "" && description != ""
}
```

问题点：

- 判定规则是"`---` 之间的**每一行都必须含冒号**"，因此**不接受**：`#` 注释行、空行、`- ` 列表项、多行块（`|` / `>`）、嵌套缩进键；文件少于 4 行直接失败。
- 取值只做 `strings.TrimSpace(value)`，不是 YAML 语义：`description: "..."` 会把**外层引号当作描述内容**保留。
- 影响面：真实第三方 SKILL.md（含注释/空行/列表）会被**整条丢弃**。

### 标记 ②　依赖现状（佐证"没有引入解析库"）

- `agent/go.mod`：`require` 段**只有** `github.com/sashabaranov/go-openai v1.42.0`
- `agent/vendor/modules.txt`：无 yaml / toml 类库（vendor 内仅 `github.com/sashabaranov/...` 及其 `jsonschema` 子包）

### 标记 ③　契约与实现不一致（同源问题）

- `agent/skillcatalog.go:36`（下发给模型的安装说明原文）：
  > `frontmatter must begin/end with --- and contain nonempty single-line name and description accepted by pulse7.`
- 对照标记 ① 的实际判定：**说明比实现宽松**。结果是"模型按说明判定已合规安装 / 系统按实现判为跳过"。

### 标记 ④　被跳过的分支（只有"成功或丢弃"两态）

`agent/skills.go:78-80`

```go
name, description, ok := parseSkillFrontmatter(string(b))
if !ok {
	catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("跳过 frontmatter 缺失或格式错误的 skill：%s", path))
	continue
}
```

问题点：不存在"**不合规但存在**"的状态；该技能随后在目录、界面列表、使用判定中**全部缺席**。

### 标记 ⑤　前端同类的自研渲染器（同批讨论项）

- `web/src/lib/md.js:1-70`（`mdToHtml`）：自研正则渲染，注释自述支持"标题/加粗/斜体/行内码/列表/引用/分隔线/段落"，**无表格规则**；`| 列 | 列 |` 会落到普通段落分支（`md.js:66`）。
- `web/package.json`：`dependencies` 仅 `@element-plus/icons-vue` / `element-plus` / `vue`，**无 Markdown 库**。
- 相关：工具结果区以 `<pre>` 展示原文、不做 Markdown 渲染（`web/src/components/ToolRecord.vue:133`）。

---

## 2. A 类故障

### A1 —— 个人目录失败 → 技能全丢 + 整轮失败

#### 标记 ⑥　发现阶段"整体返回"

`agent/skills.go:43-49`

```go
func discoverSkills(workspace string) skillCatalog {
	home, err := os.UserHomeDir()
	if err != nil {
		return skillCatalog{Warnings: []string{fmt.Sprintf("无法确定个人目录，跳过个人 skills：%v", err)}}
	}
	return scanSkills(workspace, home)
}
```

问题点：个人目录不可解析时**整个函数返回**，连与个人目录无关的**工作区技能**也不再扫描。对照 `agent/skills.go:56-63`，`scanSkills` 对每个 root 本已具备逐项容错（`os.IsNotExist` → `continue`）。

#### 标记 ⑦　同一失败被升级为 error

`agent/skillcatalog.go:19-23`

```go
func skillInstallationInstructions(workspace string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("skill installation user directory: %w", err)
	}
```

#### 标记 ⑧　error 上抛至回合（模型尚未被调用）

`agent/skillcatalog.go:192-195`

```go
installation, err := skillInstallationInstructions(cfg.workspace)
if err != nil {
	return err
}
```

`agent/main.go:982-984`（`streamTurn` 首行）

```go
func streamTurn(client *openai.Client, reg *Registry, cfg *config, msgs *[]openai.ChatCompletionMessage, stats *turnStats) (string, error) {
	if err := refreshSkillCatalog(cfg, msgs); err != nil {
		return "", err
	}
```

错误出口（界面得到 `turn_result: error`）：`agent/main.go:737` / `769` / `957` / `964`、`agent/api_runtime.go:308`。

---

### A2 —— "使用"判定依赖"发现"结果

#### 标记 ⑨　判定依据选错（在"已发现列表"里找匹配）

`agent/skills.go:153-181`（`loadedSkillForRead`），核心在 `:171`

```go
for _, skill := range discoverSkills(workspace).Skills {   // ← 只在"已发现列表"里匹配
	path := skill.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspace, path)
	}
	path, err = filepath.Abs(path)
	if err == nil && strings.EqualFold(filepath.Clean(requested), filepath.Clean(path)) {
		return skill, true
	}
}
return skillInfo{}, false
```

问题点：被跳过的技能不在该列表中，因此**永远返回 false**——即使模型确实 `read` 了它的 SKILL.md。

#### 标记 ⑩　事件只在判定为真时发出

`agent/main.go:1103-1107`

```go
res := reg.Execute(c.Function.Name, c.Function.Arguments)
emitToolResult(c, res)
if !strings.HasPrefix(res, "error:") {
	if skill, ok := loadedSkillForRead(cfg.workspace, c.Function.Name, c.Function.Arguments); ok {
		emitRuntimeEvent("skill_loaded", skillLoadedEvent{Name: skill.Name, Path: skill.Path})
	}
}
```

连带消费方：

- `web/src/store/store.js:1255-1257`（`skillsUsed` 仅由 `skill_loaded` 填充）
- `web/src/components/SettingsDialog.vue:327`（"本轮已读取正文"标记）

#### 标记 ⑪　权威 root 定义已存在，但未用于该判定

`agent/skills.go:36-41`（`skillRoots`：`<workspace>/.pulse7/skills` = workspace、`<home>/.pulse7/skills` = global）

---

### A3 —— 同一轮内技能目录被扫描 3 次（无复用）

| 标记 | 位置 | 时机 | 产物去向 |
|---|---|---|---|
| ⑫ | `agent/events.go:203`（`emitSessionInit`） | 会话初始化 | `session_init` 事件 → 界面技能列表 |
| ⑬ | `agent/skillcatalog.go:196`（`refreshSkillCatalog`） | **每个用户回合** | 目录 system 消息（进模型）+ `skill_catalog` 事件 |
| ⑭ | `agent/skills.go:171`（`loadedSkillForRead`） | **每次 `read` 工具调用** | 仅用于 `skill_loaded` 判定 |

调用入口：`agent/main.go:1103`（每次工具结果后触发 ⑭）、`agent/main.go:982`（每回合触发 ⑬）。

问题点：

- 三处各自读盘 + 解析，**无复用**；同轮内目录变化时三份结果可能不一致。
- 相关不一致：`session_init.skills`（会话初始化时计算，入口 `agent/api_runtime.go:190-201`）与 `skill_catalog`（每轮计算）是**两份不同时刻的扫描结果**；界面"技能列表"与"本次任务发现 N 个技能"因此可能相互矛盾。

---

### A4 —— 警告双通道 + 每轮重复 + 前端两处渲染

#### 标记 ⑮　后端打印通道

`agent/skills.go:140-144`

```go
func printSkillWarnings(catalog skillCatalog) {
	for _, warning := range catalog.Warnings {
		out("[警告] %s\n", warning)
	}
}
```

→ `out()` → `teeOut`，其定义为 "stdout + agent.log when active"（`agent/main.go:81-86`）。

#### 标记 ⑯　每轮无条件打印 + 发事件（无与上一轮比较/去重）

`agent/skillcatalog.go:245-246`

```go
printSkillWarnings(catalog)
emitRuntimeEvent("skill_catalog", state)
```

`state.Warnings` 的来源：`agent/skillcatalog.go:48`（结构体字段）、`:101`（赋值）。

#### 标记 ⑰　前端两处渲染同一份 warnings

- `web/src/components/InlineRecord.vue:67` → `v-for="warning in it.warnings"` → `⚠ {{ warning }}`（聊天时间线，每轮新增一条）
- `web/src/components/SettingsDialog.vue:306` → `v-for="warning in store.skillCatalog.warnings"` → `⚠ {{ warning }}`（设置面板 Skills 页）

---

## 3. 汇总表（按位置索引）

| # | 位置 | 问题类别 |
|---|---|---|
| ① | `agent/skills.go:101-126` | YAML 自研解析（逐行切冒号，拒注释/空行/列表/多行；引号原样保留） |
| ② | `agent/go.mod`、`agent/vendor/modules.txt` | 无 YAML 依赖（仅 go-openai） |
| ③ | `agent/skillcatalog.go:36` | 契约与实现不一致（说明宽松、实现严格） |
| ④ | `agent/skills.go:78-80` | 无"不合规但存在"状态，直接丢弃 |
| ⑤ | `web/src/lib/md.js:1-70`、`web/package.json` | 前端自研渲染，无表格规则 |
| ⑥ | `agent/skills.go:43-49` | A1：个人目录失败 → 工作区技能一并丢失 |
| ⑦ | `agent/skillcatalog.go:19-23` | A1：同一失败被升级为 error |
| ⑧ | `agent/skillcatalog.go:192-195`、`agent/main.go:982-984` | A1：模型未被调用即整轮失败 |
| ⑨ | `agent/skills.go:153-181`（尤 `:171`） | A2：使用判定依赖发现结果 |
| ⑩ | `agent/main.go:1103-1107` | A2：`skill_loaded` 发不出 → 使用证据缺失 |
| ⑪ | `agent/skills.go:36-41` | A2：权威 root 定义已存在但未被判定使用 |
| ⑫⑬⑭ | `agent/events.go:203`、`agent/skillcatalog.go:196`、`agent/skills.go:171` | A3：一轮内三次独立全量扫描 |
| ⑮⑯ | `agent/skills.go:140-144`、`agent/skillcatalog.go:245-246` | A4：双通道 + 每轮无条件重复 |
| ⑰ | `web/src/components/InlineRecord.vue:67`、`web/src/components/SettingsDialog.vue:306` | A4：前端两处渲染同一份数据 |

---

## 4. 只读复核命令

```powershell
$repo = 'E:\codex-worktrees\win7-agent\pre-phase3'

# 提交与工作区状态
git -C $repo log --oneline -3
git -C $repo status --porcelain

# YAML 解析器（标记 ①③）
Select-String -Path "$repo\agent\skills.go" -Pattern 'func parseSkillFrontmatter' -Context 0,26
Select-String -Path "$repo\agent\skillcatalog.go" -Pattern 'frontmatter must begin/end'

# 依赖现状（标记 ②）
Get-Content "$repo\agent\go.mod"
Select-String -Path "$repo\agent\vendor\modules.txt" -Pattern 'yaml'

# A1（标记 ⑥⑦⑧）
Select-String -Path "$repo\agent\skills.go" -Pattern 'func discoverSkills' -Context 0,7
Select-String -Path "$repo\agent\skillcatalog.go" -Pattern 'UserHomeDir'
Select-String -Path "$repo\agent\main.go" -Pattern 'refreshSkillCatalog\(cfg, msgs\)'

# A2（标记 ⑨⑩⑪）
Select-String -Path "$repo\agent\skills.go" -Pattern 'func loadedSkillForRead' -Context 0,28
Select-String -Path "$repo\agent\main.go" -Pattern 'loadedSkillForRead'
Select-String -Path "$repo\agent\skills.go" -Pattern 'func skillRoots' -Context 0,6

# A3（标记 ⑫⑬⑭）
Select-String -Path "$repo\agent\events.go","$repo\agent\skillcatalog.go","$repo\agent\skills.go" -Pattern 'discoverSkills\('

# A4（标记 ⑮⑯⑰）
Select-String -Path "$repo\agent\skills.go" -Pattern 'func printSkillWarnings' -Context 0,5
Select-String -Path "$repo\agent\skillcatalog.go" -Pattern 'printSkillWarnings\(catalog\)|emitRuntimeEvent\("skill_catalog"'
Select-String -Path "$repo\web\src\components\InlineRecord.vue","$repo\web\src\components\SettingsDialog.vue" -Pattern 'warnings'

# 全套回归（8 项失败，见附录）
cd "$repo\agent"; go test -mod=vendor ./...
```

---

## 5. 附录：全套回归失败项清单（8 项）与定性

来源（本机、未纳入版本控制，`.gitignore` 含 `/dist/`）：
`dist/build-evidence-20260916/go-test-full-rerun.log`（40,400 字节，2026-09-16 20:47:18）

| 失败测试 | 断言位置 / 报错原文 | 定性 |
|---|---|---|
| `TestF1MinimumBudgetNeverSendsRequest` | `agent/f1_remediation_test.go:51` → `protected group lost` | **断言过期**：`streamTurn` 首行 `refreshSkillCatalog` 会无条件插入"技能安装策略" system 消息（`agent/skillcatalog.go:237-243`），`msgs` 由 4 条变 5 条；`:50` 的 `len(msgs) != 4 \|\| msgs[2]...` 因此失败。该测试的核心保护（`:47-49` 的 `err != nil && requests == 0`）未失败 |
| `TestEditGBKPreservesUnrelatedBytes` | `agent/fileenc_test.go:69` → `file changed but manifest persistence failed: open : ...` | **夹具缺字段**：`newTestRegistry`（`agent/fileenc_test.go:26-31`）未设 `Registry.workspace` 与 `Registry.auditPath`；`recordFileChange`（`agent/tools.go:1125-1139`）据此判为"工作区外写入"并写空审计路径 |
| `TestWriteKeepsEncoding` | `agent/fileenc_test.go:174` → 同上 | 同上 |
| `TestWriteKeepsBOM` | `agent/fileenc_test.go:195` → 同上 | 同上 |
| `TestEditKeepsBOM` | `agent/fileenc_test.go:211` → 同上 | 同上 |
| `TestH13TwentyNativeStartStopCycles` | `agent/h13_cycles_test.go:14` → `H_PRODUCT required` | **环境前置**：缺 `H_PRODUCT` 时 `t.Fatal`（而非 `t.Skip`），使默认 `go test ./...` 无法全绿 |
| `TestH5FourExitPathsHarvestProcessTable` | `agent/h5_exit_test.go:105` → `H_PRODUCT must identify the Win7 product binary for exit integration` | 同上 |
| `TestH2TimeoutConfirmsProcessTable` | `agent/h2_termination_test.go:55` → `root=6588 targets=map[6588:true 10076:true]` | **时序敏感**：`:54` 的 `len(targets) < 3` 在 400 ms 采样点判定；日志显示该进程树随后被正常终止（`terminated 6588` / `terminated 10076`） |

共同点（与本文件 A 类问题的关联）：多处**按"整体形状"断言**（消息数量、消息下标、环境变量必须就位），因此主线一旦新增一条 system 消息或环境未配置，就会连带失败。

---

## 6. 本文边界声明

- 本文**只做问题定位**：给出位置、代码片段、问题性质与影响面。
- 本文**不包含修正方案**，不推荐实现路线，不判定修复优先级以外的取舍。
- 本文**未改动任何源码**；定稿时工作区为干净状态（`git status --porcelain` 为空）。
- 标记 ⑤（前端 Markdown 渲染）属**展示层**问题，与应用功能可用性无关，列在此处仅因同批讨论。
