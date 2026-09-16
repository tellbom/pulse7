# Win7 SkillHub 安装与 pulse7 技能使用实测

2026-09-15。目标 Win7 SP1：user@192.168.124.3:2222。只做部署、安装、独立夹具验证，未修改 pulse7/SkillHub/技能源文件。本轮未提交/tag/push。

## 结论

1. 安装通过。内网 registry http://192.168.124.2:18081 及 npm 外网均从 Win7 可访问。
2. 用户原命令安装成功，但默认目标为 C:/Users/user/skillhub-validation/.agents/skills/code-review，pulse7 不扫描该位置。
3. 使用 SkillHub 自带 --dir 安装到 C:/Users/user/.pulse7/skills/code-review；pulse7 session_init 实际列出此技能，系统提示注入技能元数据。
4. 自然任务一轮：模型没有主动读取技能，不能宣称会自动使用。
5. 明确点名技能对照一轮：read SKILL.md 成功且 skill_loaded 事件出现，证明调用链路可用；不是自然触发证据。

## 部署位置与复现

- 便携 Node：C:/Users/user/tools/node-v18.20.2-win-x64。npm 10.5.0。机器原先无 node/npm/npx。
- SkillHub npm latest 执行时为 0.1.12，engines.node >=18.0.0。
- Node 来源为 [Alex313031/node18-win7 v18.20.2](https://github.com/Alex313031/node18-win7/releases/tag/v18.20.2)，是第三方 Win7 backport，不是 Node 官方 Win7 支持声明。ZIP SHA256 fb3bbf30ce6c8e779c567588bc469a3e5548bdf47de90c93b7b670214c934830；下载后上传 Win7，解压前核对相同哈希。未安装系统 MSI、未修改永久 PATH。
- 已安装技能：C:/Users/user/.pulse7/skills/code-review/SKILL.md。当前 SSH user 的其他 pulse7 工作区也可扫描个人目录；其他 Windows 用户须安装到各自个人目录。
- 验证用最新版 pulse7：C:/Users/user/skillhub-validation/pulse7.exe，配有 runtime/git。未覆盖 C:/pulse7 中来源不明的旧部署。
- 快捷启动：C:/Users/user/skillhub-validation/start-pulse7.cmd（默认进入独立 review-project，可在 Web 换工作区）。API key 本次仅通过 stdin→环境变量传递，不落配置；要在 Web 继续真实模型任务，需要用户在页面配置连接。
- 再次安装：C:/Users/user/skillhub-validation/install-code-review.cmd。

命令等价于（先将便携 Node 加到当前终端 PATH）：

```bat
set "PATH=C:\Users\user\tools\node-v18.20.2-win-x64;%PATH%"
npx --yes @astron-team/skillhub@latest install code-review --namespace win7 --registry http://192.168.124.2:18081 --dir "C:\Users\user\.pulse7\skills"
```

本次自动化通过该 Node 执行附带 npm 的 npx-cli.js，参数与上式一致；--yes 仅接受 npx 下载包提示。npm 包来自外网 npm，技能内容来自指定内网 registry，两者不是同一地址用途。

技能版本 20260827.145540，versionId 340。SKILL.md SHA256 3abbdfe52540f9242e16589fae2cc423b3d98994c37683944462b29bab89495c。原始 SKILL.md、agents/openai.yaml、来源 metadata 均未改写，留档 captured/installed-skill/。

## 真实模型验收

端点 https://api.deepseek.com/v1，模型 deepseek-chat；不是内网模型。未通过伪造工具响应模拟调用。默认轮数 100、本地上下文预算保持产品默认；shell 检查在隔离 git 夹具内执行。此次使用 --yolo（open 档）以运行只读 git/Python 审查命令，不等同代码级 --read-only。用户任务要求不修改文件。

| 项目 | 自然任务 | 明确指定技能对照 |
|---|---|---|
| 提示词 | 请审查 HEAD~1 到 HEAD 的改动，按 docs/change-spec.md 和 CODING_STANDARDS.md 检查实际实现，指出问题和代码位置。不要修改任何文件。 | 在自然任务前加“请使用已安装的 code-review 技能进行审查。” |
| 是否新会话 | 是 | 是 |
| 主请求轮数 | 4 | 7 |
| 工具次数 | 6 | 10 |
| 完成状态 | success，exit 0 | success，exit 0 |
| read SKILL.md | 0 | 1 |
| skill_loaded | 0 | 1 |
| 输出 | 自行指出公式、验证和无用变量问题 | 按 Standards / Spec 分别给出审查 |
| git status 前/后 | 均干净 | 前干净，后新增 ?? __pycache__/ |

对照轮生成 __pycache__ 的命令是 python -c "import checkout; ..."。源文件未变，但这不是工作区零变化；模型末尾声称 No files were modified 不完整。保留该现场与日志，没有改测试以掩盖，也未修改 pulse7 实现。

## 兼容边界与建议

- 原技能明确要求并行启动 Standards/Spec 两个子代理。当前 pulse7 工具目录不含子代理能力；对照轮只有单 Agent 工具调用。因此只能认定内容被读取且审查结构被采用，不能认定完整并行流程得到执行。
- 技能 description 超过 pulse7 当前 200 字符限制，产生截断警告。它仍出现在元数据中，但尾部 Use when... 提示被截去；这是可能影响选择的因素，尚无因果对照，不能断言自然轮未使用就是它造成的。
- Skill 是按需读取的说明，不是安装后自动执行的插件。当前产品提示为“相关时用 read 工具读取完整内容”，模型仍可自行决定不读。若需要立即可靠选中，可以明确要求“使用 code-review 技能”，但这属于显式使用。
- 若后续要让该技能完整适配 pulse7，应由技能维护方提供允许单 Agent 顺序审查的版本，并精简 description；本轮未擅自重写发布技能，也未往 pulse7 增加子代理/强制调用逻辑。
- 自然任务仅实测 1 次，既不能外推“永远不会调用”，也不能写成“工作中会稳定自动调用”。安装成功、目录可见、明确读取成功、自然使用未出现须分别记账。

## 证据

artifacts/skillhub-win7-evidence/ 下：probe.json.log、install.log、catalog.log、model-run.log、explicit-run.log；captured/ 包含原始 npx 输出、两份任务、两轮会话/事件/摘要与安装文件，共 21 文件，逐文件哈希见 captured/index.json 和 build-hashes.json。记录未含密码或 API key。


## 2026-09-16：技能注入与上下文管理复核（仅评审）

当前源码 HEAD 9bcdd92。证据同时核对了当前实现与上轮 captured/sessions/sess-t0915-210608-249.jsonl、sess-t0915-210807-838.jsonl。

### 已证实的事实

- 两个会话均有 system 消息，正文 10439 UTF-8 字节；末尾技能目录为 332 字节，包含 code-review、完整个人 SKILL.md 路径和前 200 字符描述。不是只有 UI session_init 列出而没有写入 system。
- systemMessage 在 main.go 中追加 skillsSystemBlock；主请求 Messages 直接使用当前消息数组。上次没有保留 HTTP 原始请求抓包，这里证据链是持久化 system + 请求构建源码 + 实际事件，不冒充网络抓包。
- 两轮 compaction 事件均为零。自然轮初始 context_state usedTokens=4464、budget=64000、percentLeft=93.025，均为本地估算，不是 provider usage。没有预算不足的证据。
- microCompact 只处理工具结果，不处理 system。全量摘要保留 system；技能目录本身进入统一序列化字节预算。读取后的 SKILL 正文作为 read 工具结果，后续可按普通工具结果规则被 micro/摘要处理，目录仍可用于再次读取。
- 描述源长度 421 字符；scanner 截为 200 rune，实际以 `...Spec (does the code match wha` 结尾，语义不完整。后面的 `Use when ... review a branch ... review since X` 整段被截去。
- 当前 parser 按行切冒号并非完整 YAML 解析，带引号的 description 保留外层引号。此例成功解析，不是漏装或漏扫；多行 YAML 支持问题属于其他潜在输入兼容性，不是这次已证根因。

### 判断

本次“名称和描述未注入”排除；“压缩导致模型看不到目录”排除。模型在目录可见时绕过了读取，直接完成任务，但外部事件不能证明其内部认定“不需要”；提示强度、描述截断和模型采样均可能影响选择，未做各因素 A/B，不能指定单一心理原因。

技能要求子代理属于执行适配限制，但该部分描述在这次目录中也被截去，模型没有读取正文。因此不能把缺少子代理当作自然轮不读技能的已证原因。

### 上下文设计评价

元数据（名称、适用描述、路径）应进入 system；完整技能正文按需读取。目前总体方向正确。332 字节约 83 个本地估算 token，没必要为了省这点空间破坏主要适用说明；不建议因此扩大整个模型上下文或依赖 usage。

当前预算管理实际是：最多 20 技能，每项描述 200 字符，目录超过 2 KiB 只警告而不阻止注入。不能将 2 KiB 描述为硬上限；截短描述也没有保证整个目录不超 2 KiB。

发现独立问题：CLI/HTTP 仅在消息列表为空时调用 systemMessage。恢复非空旧会话时沿用历史 system，新安装/更新的 skill 不自动同步；而 session_init 会重新 discover，所以会出现界面列出新 skill、旧会话的模型上下文仍没有它的差异。本次两轮都是新会话，实际已有目录，此问题不是本次自然轮的直接原因。

### 建议执行顺序（未改代码）

1. 把技能目录作为独立可替换附件，在新用户轮次/恢复会话时刷新，不追加重复目录；同步目录版本与 UI 可见状态。
2. 描述保留完整适用语义：建议将此类 421 字符说明完整容纳（例如单项 512 字符起），配独立总目录预算（例如 16 KiB）。超限须可见，不能在半句话上静默截断后称作完整说明。确切配额需覆盖中英文与长路径用例。
3. 系统说明改为“任务明显匹配某项技能，先读取其 SKILL.md，再判断如何应用；不能执行的步骤明确披露”，用户明确点名技能必须先读取。只要求低成本取事实，不强制盲目执行整个技能，不编码任务关键词映射、不让系统替模型判断业务。
4. 保留自然任务和点名对照。先逐项验证目录刷新/描述完整，再以独立新会话重复相同自然任务对照；记录读取率和实际执行质量，不能挑一次成功宣称稳定自动调用。

本次只读评审与报告追加，未改产品提示词或实现。
