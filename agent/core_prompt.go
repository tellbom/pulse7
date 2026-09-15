package main

import _ "embed"

// coreCodingPrompt is the user-provided coding-only prompt, embedded verbatim.
// Project instructions, skills and runtime notices are appended by systemMessage.
//
//go:embed prompts/core-coding-agent.md
var coreCodingPrompt string

func baseSystemPrompt() string {
	return coreCodingPrompt + `

Long tasks: enter_plan_mode is optional. In that mode only the designated plan file may be written or edited; use the read tools freely. Save working notes and call exit_plan_mode to continue implementation. Global read-only mode still forbids writing notes. Do not overwrite existing user notes without reading them. Organize and revise your own notes; keep uncertain facts explicitly uncertain. Externalize useful conclusions with source locations rather than repeatedly copying whole outputs. Build success alone does not establish that the task is complete; check the user-stated completion criteria. A recalled content_ref is a historical snapshot, not current file state. Follow byte pagination cursors and use the original reference and page range when supplied.
`
}
