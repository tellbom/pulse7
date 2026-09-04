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
	dir := filepath.Join(home, ".win7-agent", "run", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	inner := filepath.Join(dir, "inner.cmd")
	bat := filepath.Join(dir, "run.bat")
	rel := `.win7-agent\run\` + id
	if err := os.WriteFile(inner, []byte(command+"\r\n"), 0644); err != nil {
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
	return string(outB), ec
}
