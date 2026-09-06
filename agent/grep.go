package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// T2 (retrieval): grep via frozen ripgrep 13.0.0 external exe (runtime\rg\).
// Falls back to the original Go implementation when rg is unavailable.
// Must NOT be linked as a Go library (§0.6) — external call only.

func rgExePath(exeDir string) string {
	return filepath.Join(exeDir, "runtime", "rg", "rg.exe")
}

func (r *Registry) toolGrep(argsJSON string) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	if _, err := regexp.Compile(a.Pattern); err != nil {
		return "", fmt.Errorf("bad regex: %v", err)
	}
	root := r.workspace
	if a.Path != "" {
		p, err := r.absPath(a.Path)
		if err != nil {
			return "", err
		}
		root = p
	}
	// try ripgrep first
	rgPath := rgExePath(r.exeDir)
	if _, err := os.Stat(rgPath); err == nil {
		result, err := r.grepViaRg(rgPath, a.Pattern, root, a.Glob, false)
		if err == nil {
			// T1.1: a non-ASCII pattern that found nothing may mean the files
			// are GBK — the UTF-8 pattern bytes simply cannot match. Retry
			// once with rg's GBK decoder (it transcodes matches to UTF-8).
			if strings.Contains(result, "no matches") && !isASCII(a.Pattern) {
				if retry, err2 := r.grepViaRg(rgPath, a.Pattern, root, a.Glob, true); err2 == nil {
					return retry, nil
				}
			}
			return result, nil
		}
		// rg failed (bad args etc) — fall through to Go implementation
	}
	return r.grepGo(a.Pattern, root, a.Glob)
}

func (r *Registry) grepViaRg(rgPath, pattern, root, glob string, gbk bool) (string, error) {
	args := []string{
		"--no-heading", "--line-number", "--no-messages", "--smart-case",
		"--max-count", "200",
	}
	if gbk {
		args = append(args, "-E", "gbk")
	}
	if glob != "" {
		args = append(args, "-g", glob)
	}
	args = append(args, pattern, root)
	cmd := exec.Command(rgPath, args...)
	cmd.Dir = root
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		// rg exits 1 on no matches — that's not an error
		if strings.Contains(err.Error(), "exit status 1") {
			return "no matches [ripgrep]", nil
		}
		return "", err
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) > 200 {
		lines = lines[:200]
	}
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "no matches [ripgrep]", nil
	}
	tag := " [ripgrep]"
	if gbk {
		tag = " [ripgrep gbk]"
	}
	if len(lines) >= 200 {
		return strings.Join(lines, "\n") + "\n... (ripgrep, capped at 200 lines)" + tag, nil
	}
	return strings.Join(lines, "\n") + tag, nil
}

// grepGo: the original Go implementation as fallback
func (r *Registry) grepGo(pattern, root, glob string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("bad regex: %v", err)
	}
	var lines []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == ".git" || treeSkipDirs[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(lines) >= 200 {
			return nil
		}
		if glob != "" {
			matched, _ := filepath.Match(glob, filepath.Base(p))
			if !matched {
				return nil
			}
		}
		if fi, _ := d.Info(); fi != nil && fi.Size() > 1024*1024 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(r.workspace, p)
		for i, line := range strings.Split(string(b), "\n") {
			if re.MatchString(line) {
				lines = append(lines, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				if len(lines) >= 200 {
					break
				}
			}
		}
		return nil
	})
	if len(lines) == 0 {
		return "no matches [go-fallback]", nil
	}
	if len(lines) >= 200 {
		return strings.Join(lines, "\n") + "\n... (go-fallback, capped at 200 lines)", nil
	}
	return strings.Join(lines, "\n") + " [go-fallback]", nil
}

// (end of grep.go)
