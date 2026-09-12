package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testJobRunner(t *testing.T, timeout time.Duration) (*jobObjectRunner, string) {
	t.Helper()
	workspace := t.TempDir()
	runner := &jobObjectRunner{
		Workspace:  workspace,
		Home:       homeDir(),
		Timeout:    timeout,
		MemLimitMB: 256,
	}
	t.Cleanup(runner.Close)
	return runner, workspace
}

func TestJobObjectForegroundChildSurvivesUntilSessionClose(t *testing.T) {
	runner, workspace := testJobRunner(t, 5*time.Second)
	sentinel := filepath.Join(workspace, "session-child-finished.txt")
	command := `start "" /b cmd.exe /d /c "ping -n 2 127.0.0.1 >nul & echo alive>session-child-finished.txt"`
	out, code, err := runner.Run(command)
	if err != nil || code != 0 {
		t.Fatalf("Run output=%q code=%d err=%v", out, code, err)
	}
	time.Sleep(1500 * time.Millisecond)
	if b, err := os.ReadFile(sentinel); err != nil || !strings.Contains(string(b), "alive") {
		t.Fatalf("foreground child did not survive command completion: bytes=%q err=%v", b, err)
	}
}

// Compatibility ruling: counts cover explicit members; escaped descendants are not members.
func TestJobObjectProcessCountIncludesSessionChildren(t *testing.T) {
	runner, _ := testJobRunner(t, 5*time.Second)
	pid := hDirectSessionPing(t, runner)
	count, err := runner.ProcessCount()
	if err != nil || count != 1 {
		t.Fatalf("direct member count=%d err=%v", count, err)
	}
	runner.Close()
	if _, present := hProcessTable(t)[pid]; present {
		t.Fatal("direct member survived Close")
	}
}

func TestJobObjectSessionCloseTerminatesDescendants(t *testing.T) {
	runner, workspace := testJobRunner(t, 5*time.Second)
	sentinel := filepath.Join(workspace, "may-finish-after-close.txt")
	command := `start "" /b cmd.exe /d /c "ping -n 3 127.0.0.1 >nul & echo escaped>may-finish-after-close.txt"`
	if out, code, err := runner.Run(command); err != nil || code != 0 {
		t.Fatalf("Run output=%q code=%d err=%v", out, code, err)
	}
	count, err := runner.ProcessCount()
	if err != nil {
		t.Fatal(err)
	}
	if runner.silentBreakaway && count != 0 {
		t.Fatalf("escaped children counted in Job: %d", count)
	}
	runner.Close()
	time.Sleep(3500 * time.Millisecond)
	_, err = os.Stat(sentinel)
	if runner.silentBreakaway {
		if err != nil {
			t.Fatalf("Win7 escaped child did not complete: %v", err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("non-breakaway descendant survived: %v", err)
	}
}

func TestJobObjectAssignsBeforeResume(t *testing.T) {
	runner, _ := testJobRunner(t, 5*time.Second)
	var assigned atomic.Bool
	var resumedBeforeAssignment atomic.Bool
	runner.onAssigned = func() { assigned.Store(true) }
	runner.beforeResume = func() {
		if !assigned.Load() {
			resumedBeforeAssignment.Store(true)
		}
	}

	out, code, err := runner.Run(`echo ORDER-OK`)
	if err != nil || code != 0 || !strings.Contains(out, "ORDER-OK") {
		t.Fatalf("Run output=%q code=%d err=%v", out, code, err)
	}
	if !assigned.Load() || resumedBeforeAssignment.Load() {
		t.Fatalf("assigned=%v resumedBeforeAssignment=%v", assigned.Load(), resumedBeforeAssignment.Load())
	}
}

func TestJobObjectAssignmentFailureBlocksCommandBeforeItRuns(t *testing.T) {
	runner, workspace := testJobRunner(t, 5*time.Second)
	sentinel := filepath.Join(workspace, "must-not-exist.txt")
	assignCalls := 0
	runner.assignProcess = func(_, _ uintptr) error {
		assignCalls++
		return errors.New("injected nested-job denial")
	}
	resumed := false
	runner.beforeResume = func() { resumed = true }

	out, code, err := runner.Run(`echo escaped>must-not-exist.txt`)
	if err == nil || code != -1 || !strings.Contains(err.Error(), "containment unavailable") {
		t.Fatalf("Run output=%q code=%d err=%v", out, code, err)
	}
	if resumed {
		t.Fatal("command was resumed after Job assignment failed")
	}
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Fatalf("blocked command created %s: %v", sentinel, statErr)
	}
	if _, _, secondErr := runner.Run(`echo escaped-again>must-not-exist.txt`); secondErr == nil {
		t.Fatal("restricted session allowed a later command")
	}
	if assignCalls != 1 {
		t.Fatalf("restricted session retried containment assignment %d times", assignCalls)
	}
}

func TestJobObjectTimeoutTerminatesDescendantTree(t *testing.T) {
	runner, workspace := testJobRunner(t, 300*time.Millisecond)
	sentinel := filepath.Join(workspace, "descendant-survived.txt")
	command := `start "" /b cmd.exe /d /c "ping -n 3 127.0.0.1 >nul & echo escaped>descendant-survived.txt" & ping -n 10 127.0.0.1 >nul`

	out, _, err := runner.Run(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "TIMEOUT: job tree terminated") {
		t.Fatalf("timeout output = %q", out)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Fatalf("descendant survived Job timeout and created %s: %v", sentinel, statErr)
	}
}

func TestJobObjectInterruptTerminatesDescendantTree(t *testing.T) {
	runner, workspace := testJobRunner(t, 20*time.Second)
	sentinel := filepath.Join(workspace, "interrupt-descendant-survived.txt")
	command := `start "" /b cmd.exe /d /c "ping -n 3 127.0.0.1 >nul & echo escaped>interrupt-descendant-survived.txt" & ping -n 10 127.0.0.1 >nul`
	type result struct {
		out  string
		code int
		err  error
	}
	done := make(chan result, 1)
	go func() {
		out, code, err := runner.Run(command)
		done <- result{out: out, code: code, err: err}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadUintptr(&runner.curProcess) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadUintptr(&runner.curProcess) == 0 {
		t.Fatal("runner never published its active process")
	}
	runner.Interrupt()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("interrupted Run output=%q code=%d err=%v", got.out, got.code, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after Interrupt")
	}
	time.Sleep(2500 * time.Millisecond)
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Fatalf("descendant survived Job interrupt and created %s: %v", sentinel, statErr)
	}
}
