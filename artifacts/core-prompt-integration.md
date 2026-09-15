# 基础提示词接入

用户提供的 Pulse7 Core Coding Agent Prompt.md 已逐字节复制到 agent/prompts/core-coding-agent.md，通过 Go embed 编入可执行文件，baseSystemPrompt() 直接返回该文本。原有三条基础规则已替换，不重复叠加。

来源 SHA256：564eff29303d2b247d1aab11ffbf4ae87a8af7014f95dfca2d2bb7aa8c5b1128；大小 9332 字节。复制后哈希相同，amd64/386 二进制均检出完整原文。claude-fable-5.1.md 作为来源参考，未导入其产品身份、非编码规则、虚构工具环境或平台信息。

systemMessage 原有进程提示、工作区 AGENT.md 与技能元数据拼接保持不变。基础提示词不受工作区 AGENT.md 的 8 KiB 截断限制。附件提及阅读 CLAUDE.md / AGENTS.md 是给模型的行为要求，本次没有新增这些文件的自动加载机制，也没有新增模型主动压缩工具。

Go 1.20.14：amd64/386 产品构建通过；amd64 测试编译与 vet 通过。未运行 Win7 回归或真实模型行为测试，不把编译成功表述为行为验收。

产物：artifacts/large-content-evidence/core-prompt-amd64.exe、core-prompt-386.exe。当前运行进程未替换；需使用新二进制启动并真正新建会话才能获得新基础提示词。恢复旧会话仍可能使用其已保存的旧 system 消息，本轮未改写历史。

独立提示词文件便于后续维护，但采用编译期嵌入，修改 Markdown 后需要重新构建，不是热加载。未提交、打 tag 或 push，未修改前端或其他人的在途改动。
