package main

// present.go: output layering (output-layering T1/T2/T3).
//
// Screen vs log vs model are now three different audiences:
//   - the MODEL still receives full tool results (unchanged - reg.Execute
//     return value goes into the conversation and the session file);
//   - agent.log still receives full tool results via logOnly (audit trail,
//     e.g. the shell20 harness greps for echoed markers there);
//   - the SCREEN receives one call line + one summary line per tool call,
//     plus up to 15 lines of detail ONLY when something failed.
//
// All markers are pure ASCII ([OK]/[!!]/-> and =/- separators): Win7 conhost
// with raster fonts cannot be trusted with U+2713-style glyphs, and no
// ANSI/VT sequences are used anywhere (project rule).

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// logOnly receives full-fidelity output that must not flood the screen but
// must stay in agent.log (nil when no log is active).
var logOnly io.Writer

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// toolArgSummary: one short phrase per tool, information over uniformity -
// read cares about the path, shell about the command, grep about the pattern.
func toolArgSummary(name, argsJSON string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(argsJSON), &m) != nil {
		return truncate(argsJSON, 60)
	}
	str := func(k string) string { s, _ := m[k].(string); return s }
	switch name {
	case "read", "write", "edit", "ls":
		return str("path")
	case "shell":
		return str("command")
	case "grep":
		s := str("pattern")
		if g := str("glob"); g != "" {
			s += "  glob=" + g
		}
		return s
	case "glob":
		return str("pattern")
	default: // checkpoint / rollback / get_time / tree / future tools
		return ""
	}
}

// printToolCall: the "doing" line, printed before execution.
func printToolCall(name, argsJSON string) {
	arg := toolArgSummary(name, argsJSON)
	if arg == "" {
		out("-> %s\n", name)
	} else {
		out("-> %s %s\n", name, truncate(arg, 80))
	}
}

// printToolResult: the "done" line. failed results additionally print up to
// 15 lines of detail - the user needs to see what broke.
func printToolResult(name, argsJSON, res string) {
	line, failed, detail := resultSummary(name, argsJSON, res)
	if failed {
		out("[!!] %s\n", line)
		for _, d := range detail {
			out("      %s\n", d)
		}
	} else {
		out("[OK] %s\n", line)
	}
	if logOnly != nil {
		fmt.Fprintf(logOnly, "[tool-result-full] %s\n", strings.ReplaceAll(res, "\n", " | "))
	}
}

// resultSummary builds the one-line summary per tool family.
func resultSummary(name, argsJSON, res string) (line string, failed bool, detail []string) {
	arg := truncate(toolArgSummary(name, argsJSON), 60)
	body := res
	shellEC := -1
	if strings.HasPrefix(res, "exitcode=") {
		first := res[:strings.IndexByte(res, '\n')]
		fmt.Sscanf(first, "exitcode=%d", &shellEC)
		body = strings.TrimPrefix(res, first+"\n")
	}
	outLines := strings.Split(strings.TrimRight(body, "\r\n"), "\n")
	nOut := len(outLines)
	detailLines := func(n int) []string {
		if n > nOut {
			n = nOut
		}
		return outLines[:n]
	}

	if strings.HasPrefix(res, "error:") {
		return name + " " + arg + "（失败：" + truncate(strings.TrimSpace(strings.TrimPrefix(res, "error:")), 100) + "）", true, detailLines(6)
	}
	if shellEC >= 0 {
		if shellEC != 0 {
			return fmt.Sprintf("%s %s（exitcode=%d，输出 %d 行）", name, arg, shellEC, nOut), true, detailLines(15)
		}
		return fmt.Sprintf("%s %s（exitcode=0，输出 %d 行）", name, arg, nOut), false, nil
	}
	switch name {
	case "read":
		if strings.HasPrefix(res, "[无法按文本读取") {
			return name + " " + arg + "（拒绝读取）", true, nil
		}
		if f := strings.SplitN(res, "\n", 2); len(f) > 0 && strings.HasPrefix(f[0], "[第 ") {
			return name + " " + arg + "（" + strings.Trim(f[0], "[]") + "）", false, nil
		}
		return name + " " + arg + "（" + strconv.Itoa(nOut) + " 行）", false, nil
	case "edit":
		if strings.Contains(res, "replaced 1 occurrence") {
			return name + " " + arg + "（替换 1 处）", false, nil
		}
	case "write":
		if strings.HasPrefix(res, "created ") {
			return name + " " + arg + "（新建）", false, nil
		}
		if strings.HasPrefix(res, "modified ") {
			return name + " " + arg + "（已覆盖）", false, nil
		}
	}
	return fmt.Sprintf("%s %s（%d 行）", name, arg, nOut), false, nil
}

// linePrefixer streams model narrative one line at a time with a marker, so
// "what the model says while working" is visually distinct from tool lines
// and the final framed answer.
type linePrefixer struct {
	prefix string
	buf    strings.Builder
}

func (p *linePrefixer) write(chunk string) {
	p.buf.WriteString(chunk)
	for {
		s := p.buf.String()
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return
		}
		line := strings.TrimRight(s[:i], "\r")
		p.buf.Reset()
		p.buf.WriteString(s[i+1:])
		if line != "" {
			out("%s%s\n", p.prefix, line)
		}
	}
}

// flush emits the trailing partial line (no newline yet from the model).
func (p *linePrefixer) flush() {
	s := p.buf.String()
	p.buf.Reset()
	if s = strings.TrimRight(s, "\r\n"); s != "" {
		out("%s%s\n", p.prefix, s)
	}
}

// frameAnswer prints the final answer inside ASCII separators (T2): the one
// block the user reads when the task ends.
func frameAnswer(content string) {
	out("========================================\n")
	for _, l := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		out("%s\n", l)
	}
	out("========================================\n")
}
