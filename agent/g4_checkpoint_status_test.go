package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckpointCountsOrdinaryIndexAndPreservesBytes(t *testing.T) {
	_, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"a.txt": "base\n", "b.txt": "base\n"})
	clone := filepath.Join(t.TempDir(), "clone")
	if b, err := exec.Command(g.gitExe, "clone", "-c", "core.autocrlf=true", g.workspace, clone).CombinedOutput(); err != nil {
		t.Fatalf("clone: %s %v", b, err)
	}
	g.workspace = clone
	ordinary := exec.Command(g.gitExe, "status", "--porcelain=v1")
	ordinary.Dir = g.workspace
	before, statusErr := ordinary.CombinedOutput()
	t.Logf("ordinary status before checkpoint=%q", before)
	if statusErr != nil || len(before) != 0 {
		t.Fatalf("fixture is not a clean clone: %q %v", before, statusErr)
	}
	for _, want := range []int{0, 1, 2} {
		if want > 0 {
			name := []string{"a.txt", "b.txt"}[want-1]
			if err := os.WriteFile(filepath.Join(g.workspace, name), []byte("changed\r\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		result, err := g.CheckpointAs("auto")
		t.Logf("checkpoint: %s error=%v", result, err)
		expected := []string{"工作区相对 HEAD 的变更文件数=0", "工作区相对 HEAD 的变更文件数=1", "工作区相对 HEAD 的变更文件数=2"}[want]
		if err != nil || !strings.Contains(result, expected) {
			t.Fatalf("want %s; got %s err=%v", expected, result, err)
		}
		cmd := g.gitCommand("", "show", g.ref(g.seq)+":a.txt")
		blob, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		disk, err := os.ReadFile(filepath.Join(g.workspace, "a.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(blob) != string(disk) {
			t.Fatalf("checkpoint lost bytes: blob=%q disk=%q", blob, disk)
		}
		t.Logf("stored bytes=%q", blob)
	}
}

func TestCheckpointReportsOrdinaryStatusFailure(t *testing.T) {
	_, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"a.txt": "base\n"})
	if err := os.WriteFile(filepath.Join(g.workspace, ".git", "index"), []byte("invalid index"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := g.CheckpointAs("auto")
	t.Logf("result=%q error=%v", result, err)
	if err == nil || !strings.Contains(err.Error(), "status") || result != "" {
		t.Fatalf("status failure was hidden: %q %v", result, err)
	}
}

func TestStatusRecordsCountPathsNotLines(t *testing.T) {
	for _, tc := range []struct {
		data string
		want int
		bad  bool
	}{
		{"", 0, false},
		{" M line\nbreak.txt\x00?? folder/a.txt\x00?? folder/b.txt\x00", 3, false},
		{"R  new name.txt\x00old name.txt\x00 M another.txt\x00", 2, false},
		{"C  copied.txt\x00source.txt\x00", 1, false},
		{" M unterminated", 0, true},
		{"R  target\x00", 0, true},
	} {
		n, err := countStatusRecords([]byte(tc.data))
		if (err != nil) != tc.bad || (!tc.bad && n != tc.want) {
			t.Fatalf("data=%q count=%d err=%v", tc.data, n, err)
		}
	}
}

func TestFirstCheckpointIncludesPreexistingChanges(t *testing.T) {
	_, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"a.txt": "base\n", "b.txt": "base\n"})
	if err := os.WriteFile(filepath.Join(g.workspace, "a.txt"), []byte("user edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := g.gitRun("", "mv", "b.txt", "renamed b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.workspace, "untracked.txt"), []byte("user file\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := g.CheckpointAs("auto")
	t.Logf("preexisting modified+rename+untracked: %s err=%v", result, err)
	if err != nil || !strings.Contains(result, "工作区相对 HEAD 的变更文件数=3") {
		t.Fatalf("preexisting changes count: %s %v", result, err)
	}
}
