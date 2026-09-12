package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRapidBackgroundTasksHaveDistinctIdentityAndFiles(t *testing.T) {
	runner := &fakeManagedRunner{}
	m := newBackgroundTaskManager(runner, filepath.Join(t.TempDir(), "tasks"), t.TempDir())
	m.configure(50, 0, 0, 0, 0)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		runner.output = fmt.Sprintf("output-%d", i)
		task, err := m.launch(fmt.Sprintf("command-%d", i), false)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("STARTED id=%s pid=%d", task.ID, task.PID)
		if seen[task.ID] {
			t.Fatalf("duplicate task ID %s at launch %d", task.ID, i)
		}
		seen[task.ID] = true
	}
	if len(m.tasks) != 50 {
		t.Fatalf("registered tasks=%d, want 50", len(m.tasks))
	}
	for _, task := range m.tasks {
		var index int
		fmt.Sscanf(task.Command, "command-%d", &index)
		content, err := os.ReadFile(task.OutputPath)
		if err != nil || string(content) != fmt.Sprintf("output-%d", index) {
			t.Fatalf("task %s output=%q err=%v", task.ID, content, err)
		}
	}
}

func TestTaskIdentitySurvivesManagerRecreation(t *testing.T) {
	dir, workspace := filepath.Join(t.TempDir(), "tasks"), t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		m := newBackgroundTaskManager(&fakeManagedRunner{}, dir, workspace)
		m.configure(50, 50, 0, 0, 0)
		task, err := m.launch("distinct manager", false)
		if err != nil {
			t.Fatal(err)
		}
		if seen[task.ID] {
			t.Fatalf("recreated manager reused ID %s at %d", task.ID, i)
		}
		seen[task.ID] = true
		t.Logf("manager=%d id=%s", i, task.ID)
	}
}
