package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestH4DetachedPersistenceAndVerifiedKill(t *testing.T) {
	j, _ := testJobRunner(t, time.Second)
	if err := j.StartSession(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := j.processes.loadDetached(dir); err != nil {
		t.Fatal(err)
	}
	rf, err := buildRunFiles(j.Home, j.Workspace, "ping -n 30 127.0.0.1 >nul")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rf.dir)
	process, thread, pid, err := createSuspendedBatch(rf.batPath)
	if err != nil {
		t.Fatal(err)
	}
	defer procCloseHandle.Call(thread)
	defer procCloseHandle.Call(process)
	defer terminateSuspendedProcess(process)
	if err := j.processes.add(pid, "persist detached fixture", "detached", "", true); err != nil {
		t.Fatal(err)
	}
	p, err := processIdentity(syscall.Handle(process), pid)
	if err != nil {
		t.Fatal(err)
	}
	j.processes.exitSummary()
	j.Close()
	if state, _ := syscall.WaitForSingleObject(syscall.Handle(process), 0); state != syscall.WAIT_TIMEOUT {
		t.Fatal("detached process harvested")
	}
	if _, err := os.Stat(filepath.Join(dir, "process-exit.json")); err != nil {
		t.Fatal(err)
	}
	next := newProcessRecords()
	defer next.close()
	if err := next.loadDetached(dir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next.previousNotice(), "未经核实") {
		t.Fatal(next.previousNotice())
	}
	if state, _ := syscall.WaitForSingleObject(syscall.Handle(process), 0); state != syscall.WAIT_TIMEOUT {
		t.Fatal("startup auto-terminated detached process")
	}
	// Construct stale history referencing the live victim, as after PID reuse.
	for _, field := range []string{"creation_time", "image_path", "pid"} {
		old := next.previous[0]
		switch field {
		case "creation_time":
			next.previous[0].CreationTime = "0"
		case "image_path":
			next.previous[0].ImagePath = "C:/wrong.exe"
		case "pid":
			next.previous[0].PID = 0xffffffff
		}
		fake := next.previous[0]
		_, err := next.killIdentity(processIdentityArgs{PID: fake.PID, CreationTime: fake.CreationTime, ImagePath: fake.ImagePath})
		if err == nil || !strings.Contains(err.Error(), "无法确认该 PID 仍是原进程") {
			t.Fatalf("%s: %v", field, err)
		}
		if state, _ := syscall.WaitForSingleObject(syscall.Handle(process), 0); state != syscall.WAIT_TIMEOUT {
			t.Fatalf("%s mismatch killed victim", field)
		}
		t.Logf("%s mismatch refused; victim PID=%d remains alive: %v", field, pid, err)
		next.previous[0] = old
	}
	message, err := next.killIdentity(processIdentityArgs{PID: pid, CreationTime: p.CreationTime, ImagePath: p.ImagePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(message)
	if state, _ := syscall.WaitForSingleObject(syscall.Handle(process), 0); state != waitObject0 {
		t.Fatal("matched identity not terminated")
	}
}

func TestH4ProcessKillRejectsMismatchedIdentity(t *testing.T) {
	j, _ := testJobRunner(t, time.Second)
	if err := j.StartSession(); err != nil {
		t.Fatal(err)
	}
	rf, err := buildRunFiles(j.Home, j.Workspace, "ping -n 30 127.0.0.1 >nul")
	if err != nil {
		t.Fatal(err)
	}
	process, thread, pid, err := createSuspendedBatch(rf.batPath)
	if err != nil {
		t.Fatal(err)
	}
	defer procCloseHandle.Call(thread)
	defer procCloseHandle.Call(process)
	defer terminateSuspendedProcess(process)
	if err := j.processes.add(pid, "identity fixture", "detached", "", true); err != nil {
		t.Fatal(err)
	}
	p, err := processIdentity(syscall.Handle(process), pid)
	if err != nil {
		t.Fatal(err)
	}
	reg := &Registry{runner: j, tasks: newBackgroundTaskManager(j, t.TempDir(), j.Workspace)}
	args, _ := json.Marshal(map[string]interface{}{"process": map[string]interface{}{"pid": pid, "creation_time": "0", "image_path": p.ImagePath}})
	_, err = reg.toolTaskKill(string(args))
	if err == nil || !strings.Contains(err.Error(), "无法确认该 PID 仍是原进程") {
		t.Fatalf("identity mismatch not explicit: %v", err)
	}
	if event, _ := syscall.WaitForSingleObject(syscall.Handle(process), 0); event != syscall.WAIT_TIMEOUT {
		t.Fatal("mismatched process terminated")
	}
}
