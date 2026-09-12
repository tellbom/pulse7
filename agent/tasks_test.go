package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeManagedProcess struct {
	pid    int
	done   chan processWaitResult
	killed bool
	mu     sync.Mutex
}

func (p *fakeManagedProcess) PID() int { return p.pid }
func (p *fakeManagedProcess) Wait() (int, error) {
	result := <-p.done
	return result.exitCode, result.err
}
func (p *fakeManagedProcess) KillTree() error {
	p.mu.Lock()
	p.killed = true
	p.mu.Unlock()
	select {
	case p.done <- processWaitResult{exitCode: 1}:
	default:
	}
	return nil
}

type fakeManagedRunner struct {
	processCount int
	started      []*fakeManagedProcess
	output       string
}

func (r *fakeManagedRunner) Run(string) (string, int, error) { return "", 0, nil }
func (r *fakeManagedRunner) Mode() string                    { return "fake" }
func (r *fakeManagedRunner) Interrupt()                      {}
func (r *fakeManagedRunner) ProcessCount() (int, error)      { return r.processCount, nil }
func (r *fakeManagedRunner) StartManaged(spec processStartSpec, _ bool) (managedProcess, error) {
	if err := os.WriteFile(spec.OutputFile, []byte(r.output), 0o644); err != nil {
		return nil, err
	}
	p := &fakeManagedProcess{pid: 1000 + len(r.started), done: make(chan processWaitResult, 1)}
	if err := os.WriteFile(spec.PIDFile, []byte(fmt.Sprint(p.pid)), 0o644); err != nil {
		return nil, err
	}
	r.started = append(r.started, p)
	return p, nil
}

func TestBackgroundTaskReturnsImmediatelySupportsIncrementalOutputAndKill(t *testing.T) {
	runner := &fakeManagedRunner{output: "one\ntwo\n"}
	m := newBackgroundTaskManager(runner, filepath.Join(t.TempDir(), "tasks"), t.TempDir())
	m.configure(50, 5, 3600, 50, time.Second)
	task, err := m.start("long command", false)
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.output(task.ID, 0, 4)
	if err != nil || !strings.HasSuffix(first, "one\n") || !strings.Contains(first, "next_offset=4") {
		t.Fatalf("first output=%q err=%v", first, err)
	}
	second, err := m.output(task.ID, 4, 4)
	if err != nil || !strings.HasSuffix(second, "two\n") {
		t.Fatalf("second output=%q err=%v", second, err)
	}
	if _, err := m.kill(task.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(m.list(), "status=killed") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !runner.started[0].killed || !strings.Contains(m.list(), "status=killed") {
		t.Fatalf("task was not killed: %s", m.list())
	}
}

func TestCappedTaskWriterMarksAndNeverExceedsHardLimit(t *testing.T) {
	var output bytes.Buffer
	w := &cappedTaskWriter{w: &output, remaining: 100}
	payload := bytes.Repeat([]byte("x"), 500)
	if n, err := w.Write(payload); err != nil || n != len(payload) {
		t.Fatalf("Write n=%d err=%v", n, err)
	}
	if output.Len() > 100 || !strings.Contains(output.String(), "output truncated") {
		t.Fatalf("capped output length=%d body=%q", output.Len(), output.String())
	}
	if n, err := w.Write(payload); err != nil || n != len(payload) || output.Len() > 100 {
		t.Fatalf("discard after cap n=%d err=%v length=%d", n, err, output.Len())
	}
}

func TestTaskWorkerEnforcesOutputFileHardLimit(t *testing.T) {
	dir := t.TempDir()
	commandPath := filepath.Join(dir, "command.cmd")
	outputPath := filepath.Join(dir, "task.output")
	statusPath := filepath.Join(dir, "task.status.json")
	pidPath := filepath.Join(dir, "task.pid")
	if err := os.WriteFile(commandPath, []byte("@for /l %%i in (1,1,500) do @echo output-line-%%i\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := runTaskWorker([]string{
		"--command-file", commandPath, "--output-file", outputPath,
		"--status-file", statusPath, "--pid-file", pidPath,
		"--workspace", dir, "--max-bytes", "512",
	})
	if code != 0 {
		t.Fatalf("task worker exit code = %d", code)
	}
	b, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 512 || !strings.Contains(string(b), "output truncated") {
		t.Fatalf("worker output length=%d body=%q", len(b), b)
	}
	status, err := os.ReadFile(statusPath)
	if err != nil || !strings.Contains(string(status), `"truncated":true`) {
		t.Fatalf("worker status=%q err=%v", status, err)
	}
}

func TestTaskWorkerFailsWhenPIDCannotBePersisted(t *testing.T) {
	dir := t.TempDir()
	commandPath := filepath.Join(dir, "command.cmd")
	if err := os.WriteFile(commandPath, []byte("@echo must-not-run\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := runTaskWorker([]string{
		"--command-file", commandPath, "--output-file", filepath.Join(dir, "task.output"),
		"--status-file", filepath.Join(dir, "task.status.json"), "--pid-file", dir,
		"--workspace", dir, "--max-bytes", "512",
	})
	if code == 0 {
		t.Fatal("task worker reported success after PID persistence failed")
	}
}

func TestWaitTaskPIDRejectsMalformedHandshake(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.pid")
	if err := os.WriteFile(path, []byte("not-a-pid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := waitTaskPID(path, 20*time.Millisecond); err == nil || !strings.Contains(err.Error(), "invalid worker PID") {
		t.Fatalf("waitTaskPID error=%v", err)
	}
}

func TestTaskWarningsDoNotBlockStarts(t *testing.T) {
	runner := &fakeManagedRunner{processCount: 2}
	m := newBackgroundTaskManager(runner, filepath.Join(t.TempDir(), "tasks"), t.TempDir())
	m.configure(50, 0, 0, 1, time.Second)
	var visible bytes.Buffer
	oldOut := teeOut
	teeOut = &visible
	t.Cleanup(func() { teeOut = oldOut })
	if _, err := m.start("still starts", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(visible.String(), "任务仍已启动") || !strings.Contains(visible.String(), "新进程未被阻止") {
		t.Fatalf("warnings missing: %q", visible.String())
	}
}

func TestBackgroundTaskConfigRejectsInvalidHardLimit(t *testing.T) {
	if err := validateBackgroundTaskConfig(0, 5, 3600, 50); err == nil || !strings.Contains(err.Error(), "background-task-max-output-mb") {
		t.Fatalf("validation error=%v", err)
	}
	if err := validateBackgroundTaskConfig(50, -1, 3600, 50); err == nil || !strings.Contains(err.Error(), "background-task-warn-count") {
		t.Fatalf("validation error=%v", err)
	}
	if err := validateBackgroundTaskConfig(50, 5, -1, 50); err == nil || !strings.Contains(err.Error(), "background-task-warn-sec") {
		t.Fatalf("validation error=%v", err)
	}
	if err := validateBackgroundTaskConfig(50, 5, 3600, -1); err == nil || !strings.Contains(err.Error(), "process-warn-threshold") {
		t.Fatalf("validation error=%v", err)
	}
}

func TestExitSummaryListsHarvestedAndDetachedTasks(t *testing.T) {
	runner := &fakeManagedRunner{}
	m := newBackgroundTaskManager(runner, filepath.Join(t.TempDir(), "tasks"), t.TempDir())
	m.configure(50, 5, 3600, 50, time.Second)
	if _, err := m.start("harvest me", false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.start("leave me", true); err != nil {
		t.Fatal(err)
	}
	var visible bytes.Buffer
	oldOut := teeOut
	teeOut = &visible
	t.Cleanup(func() { teeOut = oldOut })
	m.exitSummary()
	got := visible.String()
	if !strings.Contains(got, "退出收割后台任务") || !strings.Contains(got, "harvest me") {
		t.Fatalf("harvested task missing from exit summary: %q", got)
	}
	if !strings.Contains(got, "退出保留 detached") || !strings.Contains(got, "leave me") {
		t.Fatalf("detached task missing from exit summary: %q", got)
	}
}

func TestTaskListIncludesSessionProcessOverview(t *testing.T) {
	runner := &fakeManagedRunner{processCount: 2}
	m := newBackgroundTaskManager(runner, filepath.Join(t.TempDir(), "tasks"), t.TempDir())
	m.configure(50, 5, 3600, 1, time.Second)
	got := m.list()
	if !strings.Contains(got, "session_processes=2") || !strings.Contains(got, "threshold=1") || !strings.Contains(got, "exceeded=true") {
		t.Fatalf("process overview missing from /tasks output: %q", got)
	}
}
