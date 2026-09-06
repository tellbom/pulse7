package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// T1 (retrieval): tree tool — one call replaces dozens of `ls`. Gives the
// model a project skeleton so it stops walking directories one by one.
// Directories show their direct child count; common noise dirs are skipped.
type treeEntry struct {
	name     string
	isDir    bool
	children int
}

var treeSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true, "__pycache__": true, ".venv": true,
	"bin": true, "obj": true,
}

func (r *Registry) toolTree(argsJSON string) (string, error) {
	var a struct {
		Path      string `json:"path"`
		MaxDepth  int    `json:"max_depth"`
		MaxEntries int   `json:"max_entries"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if a.MaxDepth <= 0 {
		a.MaxDepth = 3
	}
	if a.MaxEntries <= 0 {
		a.MaxEntries = 300
	}
	root := r.workspace
	if a.Path != "" {
		p, err := r.absPath(a.Path)
		if err != nil {
			return "", err
		}
		root = p
	}
	var lines []string
	omittedDirs, omittedFiles, total := 0, 0, 0
	buildTreeLines(root, root, 0, a.MaxDepth, a.MaxEntries, &lines, &total, &omittedDirs, &omittedFiles, "")
	if len(lines) == 0 {
		return "(empty directory)", nil
	}
	out := strings.Join(lines, "\n")
	if total > a.MaxEntries {
		out += fmt.Sprintf("\n... 另有 %d 个目录 / %d 个文件未显示（共 %d 项超出 %d 上限）。可用更大 max_depth 或指定子目录查看。",
			omittedDirs, omittedFiles, total, a.MaxEntries)
	}
	return out, nil
}

func buildTreeLines(root, dir string, depth, maxDepth, maxEntries int,
	lines *[]string, total, omitDirs, omitFiles *int, prefix string) {
	if depth >= maxDepth || len(*lines) >= maxEntries {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// sort: dirs first, then files, alphabetical
	var dirs, files []treeEntry
	for _, e := range entries {
		if e.IsDir() {
			name := e.Name()
			if treeSkipDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}
			dirs = append(dirs, treeEntry{name: name, isDir: true})
		} else {
			files = append(files, treeEntry{name: e.Name()})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	count := 0
	for _, d := range dirs {
		if len(*lines) >= maxEntries {
			*omitDirs++
			*total++
			continue
		}
		sub := filepath.Join(dir, d.name)
		n := countDirectChildren(sub)
		conn := "├── "
		if count == len(dirs)+len(files)-1 {
			conn = "└── "
		}
		*lines = append(*lines, fmt.Sprintf("%s%s%s/ (%d)", prefix, conn, d.name, n))
		*total++
		count++
		buildTreeLines(root, sub, depth+1, maxDepth, maxEntries, lines, total, omitDirs, omitFiles, prefix+"│   ")
	}
	for _, f := range files {
		if len(*lines) >= maxEntries {
			*omitFiles++
			*total++
			continue
		}
		conn := "├── "
		if count == len(dirs)+len(files)-1 {
			conn = "└── "
		}
		*lines = append(*lines, fmt.Sprintf("%s%s%s", prefix, conn, f.name))
		*total++
		count++
	}
}

func countDirectChildren(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			name := e.Name()
			if treeSkipDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}
		}
		n++
	}
	return n
}
