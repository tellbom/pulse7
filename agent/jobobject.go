package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// jobObjectRunner: automatic fallback when the Sandboxie driver is unavailable
// (e.g. production Win7 without SHA-2 patching — patching is NEVER required).
// Provides process-tree kill, timeout and memory cap; no filesystem isolation,
// so the application-level guardrails (path policy / confirm gate / audit /
// git rollback) remain the safety net.
type jobObjectRunner struct {
	Workspace  string
	Home       string
	Timeout    time.Duration
	MemLimitMB uint64

	curJob uintptr // current job handle while a command runs (0 otherwise)
}

var (
	k32                    = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW   = k32.NewProc("CreateJobObjectW")
	procSetInformationJob  = k32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = k32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject = k32.NewProc("TerminateJobObject")
	procCloseHandle        = k32.NewProc("CloseHandle")
	procOpenProcess        = k32.NewProc("OpenProcess")
)

const (
	jobObjectExtendedLimitInfoClass = 9
	jobObjectLimitKillOnJobClose    = 0x00002000
	jobObjectLimitJobMemory         = 0x00000200
)

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimit struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func (j *jobObjectRunner) Mode() string { return "JobObject" }

// Interrupt kills the whole current job tree (M4-T1 Ctrl-C path).
func (j *jobObjectRunner) Interrupt() {
	if job := atomic.LoadUintptr(&j.curJob); job != 0 {
		procTerminateJobObject.Call(job, 1)
	}
}

func (j *jobObjectRunner) Run(command string) (string, int, error) {
	rf, err := buildRunFiles(j.Home, j.Workspace, command)
	if err != nil {
		return "", -1, err
	}
	defer os.RemoveAll(rf.dir)

	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return "", -1, errors.New("CreateJobObject failed")
	}
	defer procCloseHandle.Call(job)
	atomic.StoreUintptr(&j.curJob, job)
	defer atomic.StoreUintptr(&j.curJob, 0)

	var info jobObjectExtendedLimit
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose | jobObjectLimitJobMemory
	if j.MemLimitMB > 0 {
		info.JobMemoryLimit = uintptr(j.MemLimitMB * 1024 * 1024)
	}
	if r1, _, _ := procSetInformationJob.Call(job,
		jobObjectExtendedLimitInfoClass,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info)); r1 == 0 {
		return "", -1, errors.New("SetInformationJobObject failed")
	}

	cmd := exec.Command("cmd.exe", "/c", rf.batPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return "", -1, err
	}
	hProc, _, _ := procOpenProcess.Call(
		uintptr(0x0100|0x0001), 0, uintptr(cmd.Process.Pid)) // PROCESS_SET_QUOTA | PROCESS_TERMINATE
	if hProc == 0 {
		cmd.Process.Kill()
		return "", -1, errors.New("OpenProcess failed")
	}
	defer procCloseHandle.Call(hProc)
	if r1, _, _ := procAssignProcessToJob.Call(job, hProc); r1 == 0 {
		// Win7 has no nested jobs: if the parent already sits in one (e.g. an
		// sshd session job) assignment is denied. Continue WITHOUT the job —
		// output/exit-code capture still works, timeout still kills the direct
		// child; containment is reduced and reported.
		out, ec := waitForCmd(cmd, rf, j)
		return out + "\n[warn] job assignment denied (nested job); reduced containment", ec, nil
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(j.Timeout):
		procTerminateJobObject.Call(job, 1)
		<-done
		out, ec := readResult(filepath.Join(j.Home, ".pulse7", "run", rf.id))
		return out + "\n[TIMEOUT: job tree terminated]", ec, nil
	}
	out, ec := readResult(filepath.Join(j.Home, ".pulse7", "run", rf.id))
	return out, ec, nil
}

// waitForCmd: no-job fallback path (kills only the direct child on timeout).
func waitForCmd(cmd *exec.Cmd, rf *runFiles, j *jobObjectRunner) (string, int) {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(j.Timeout):
		cmd.Process.Kill()
		<-done
		out, ec := readResult(filepath.Join(j.Home, ".pulse7", "run", rf.id))
		return out + "\n[TIMEOUT: direct child killed]", ec
	}
	out, ec := readResult(filepath.Join(j.Home, ".pulse7", "run", rf.id))
	return out, ec
}
