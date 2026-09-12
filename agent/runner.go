package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// runFiles builds wrapper scripts for sandboxed/direct command execution.
// inner.cmd holds the raw command (its own redirections work verbatim);
// run.bat calls it with output/exit-code capture files. This fixes the M1
// limitation where a command's own redirect was overridden by the capture.
type runFiles struct {
	dir     string
	inner   string
	batPath string
	id      string
}

func buildRunFiles(home, workspace, command string) (*runFiles, error) {
	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	dir := filepath.Join(home, ".pulse7", "run", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	inner := filepath.Join(dir, "inner.cmd")
	bat := filepath.Join(dir, "run.bat")
	rel := `.pulse7\run\` + id
	// T2 (encoding): cmd.exe parses inner.cmd under the CONSOLE codepage
	// (936 on a Chinese system, 437 on English), so the UTF-8 command must
	// be transcoded to that codepage or Chinese paths/arguments in it
	// resolve to garbage. A1: console CP, not CP_ACP.
	if err := os.WriteFile(inner, append(utf8ToCodepage(command, consoleCodepage()), '\r', '\n'), 0644); err != nil {
		return nil, err
	}
	content := "@echo off\r\n" +
		`cd /d "` + workspace + `"` + "\r\n" +
		`call "` + inner + `" > "%USERPROFILE%\` + rel + `\out.txt" 2>&1` + "\r\n" +
		`echo %errorlevel% > "%USERPROFILE%\` + rel + `\ec.txt"` + "\r\n"
	if err := os.WriteFile(bat, []byte(content), 0644); err != nil {
		return nil, err
	}
	return &runFiles{dir: dir, inner: inner, batPath: bat, id: id}, nil
}

func readResult(dir string) (string, int) {
	outB, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	ecB, _ := os.ReadFile(filepath.Join(dir, "ec.txt"))
	ec := -1
	fmt.Sscanf(string(ecB), "%d", &ec)
	// T2 (encoding): out.txt carries the CONSOLE codepage's bytes (GBK on a
	// Chinese console) for anything cmd/programs printed in the local
	// language. Already-valid UTF-8 passes through untouched. A1: decode
	// with the console codepage, falling back to CP_ACP headless.
	return decodeShellOutput(outB), ec
}
