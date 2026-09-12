package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A direct process explicitly assigned to the session Job, not an inherited child.
func hDirectSessionPing(t *testing.T, j *jobObjectRunner) uint32 {
	t.Helper()
	if err := j.StartSession(); err != nil {
		t.Fatal(err)
	}
	h, thread, pid, err := createSuspendedApplication(filepath.Join(os.Getenv("SystemRoot"), "System32", "ping.exe"), []string{"-n", "30", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	defer syscall.CloseHandle(syscall.Handle(thread))
	if ok, _, err := procAssignProcessToJob.Call(j.sessionJob, h); ok == 0 {
		syscall.TerminateProcess(syscall.Handle(h), 1)
		t.Fatal(err)
	}
	if err := j.processes.add(pid, "direct ping fixture", "foreground", "", false); err != nil {
		t.Fatal(err)
	}
	if n, _, err := procResumeThread.Call(thread); n == 0xffffffff {
		t.Fatal(err)
	}
	return pid
}

// Fixture cleanup only: close a known escaped process through its verified handle.
func hCleanupObserved(t *testing.T, p processRecord) {
	t.Helper()
	h, err := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE|1, false, p.PID)
	if err != nil {
		return
	} // An already exited fixture needs no cleanup.
	defer syscall.CloseHandle(h)
	identity, err := processIdentity(h, p.PID)
	if err != nil {
		t.Error(err)
		return
	}
	if identity.CreationTime != p.CreationTime {
		return
	}
	if state, _ := syscall.WaitForSingleObject(h, 0); state == waitObject0 {
		return
	}
	if err := syscall.TerminateProcess(h, 1); err != nil {
		t.Error(err)
		return
	}
	syscall.WaitForSingleObject(h, 5000)
}
