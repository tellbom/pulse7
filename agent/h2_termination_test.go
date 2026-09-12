package main

import (
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func hProcessTable(t *testing.T) map[uint32]syscall.ProcessEntry32 {
	t.Helper()
	h, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	entry := syscall.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	rows := map[uint32]syscall.ProcessEntry32{}
	for err = syscall.Process32First(h, &entry); err == nil; err = syscall.Process32Next(h, &entry) {
		rows[entry.ProcessID] = entry
	}
	if err != syscall.ERROR_NO_MORE_FILES {
		t.Fatal(err)
	}
	return rows
}

func TestH2TimeoutConfirmsProcessTable(t *testing.T) {
	runner, _ := testJobRunner(t, 1500*time.Millisecond)
	done := make(chan string, 1)
	go func() {
		s, _, err := runner.Run(`start "" /b ping -n 30 127.0.0.1 >nul & ping -n 30 127.0.0.1 >nul`)
		if err != nil {
			s += err.Error()
		}
		done <- s
	}()
	time.Sleep(400 * time.Millisecond)
	root := uint32(atomic.LoadUintptr(&runner.curPID))
	before := hProcessTable(t)
	targets := map[uint32]bool{root: true}
	for changed := true; changed; {
		changed = false
		for pid, p := range before {
			if targets[p.ParentProcessID] && !targets[pid] {
				targets[pid] = true
				changed = true
			}
		}
	}
	if root == 0 || len(targets) < 3 {
		t.Fatalf("root=%d targets=%v", root, targets)
	}
	for pid := range targets {
		p := before[pid]
		t.Logf("before pid=%d ppid=%d name=%s", pid, p.ParentProcessID, syscall.UTF16ToString(p.ExeFile[:]))
	}
	if s := <-done; !strings.Contains(s, "TIMEOUT: job tree terminated") {
		t.Fatal(s)
	}
	after := hProcessTable(t)
	for pid := range targets {
		_, exists := after[pid]
		t.Logf("after pid=%d present=%v", pid, exists)
		if exists {
			t.Errorf("residual PID %d", pid)
		}
	}
}

func TestH2NativeKillReportsMissingProcess(t *testing.T) {
	// This PID cannot name a Windows process; no unrelated process is targeted.
	p := &nativeManagedProcess{pid: 0xffffffff}
	if err := p.KillTree(); err == nil {
		t.Fatal("native KillTree silently accepted failed taskkill")
	}
}
