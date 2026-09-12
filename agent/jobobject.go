package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

	sessionJob      uintptr
	curProcess      uintptr
	curPID          uintptr
	jobMu           sync.Mutex
	closed          bool
	restricted      error
	processes       *processRecords
	silentBreakaway bool

	// Test seams for proving the security-sensitive ordering. Production uses
	// the native implementations below.
	assignProcess func(job, process uintptr) error
	onAssigned    func()
	beforeResume  func()
}

var (
	k32                    = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW   = k32.NewProc("CreateJobObjectW")
	procSetInformationJob  = k32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = k32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject = k32.NewProc("TerminateJobObject")
	procCloseHandle        = k32.NewProc("CloseHandle")
	procResumeThread       = k32.NewProc("ResumeThread")
)

const (
	jobObjectExtendedLimitInfoClass = 9
	jobObjectLimitKillOnJobClose    = 0x00002000
	jobObjectLimitJobMemory         = 0x00000200
	jobObjectLimitSilentBreakawayOK = 0x00001000
	createSuspended                 = 0x00000004
	createBreakawayFromJob          = 0x01000000
	createNoWindow                  = 0x08000000
	waitObject0                     = 0
	infiniteWait                    = 0xffffffff
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
	// MSVC aligns JOBOBJECT_BASIC_LIMIT_INFORMATION to 8 bytes even on
	// 32-bit Windows. Go's 386 ABI makes it 44 bytes, so insert the missing
	// four bytes before IoInfo. Keeping this outside the nested structure
	// avoids Go's trailing-zero-size-field padding on amd64.
	_                     [8 - unsafe.Sizeof(uintptr(0))]byte
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func (j *jobObjectRunner) Mode() string { return "JobObject" }

func (j *jobObjectRunner) markRestricted(err error) error {
	j.jobMu.Lock()
	if j.restricted == nil {
		j.restricted = err
	}
	restricted := j.restricted
	j.jobMu.Unlock()
	j.emitMode("entered")
	return restricted
}

// H1: publish the same persistent state on entry, later commands and /tasks.
func (j *jobObjectRunner) modeState(action string) processModeEvent {
	j.jobMu.Lock()
	defer j.jobMu.Unlock()
	s := processModeEvent{Action: action, Mode: "JobObject", CleanupGuaranteed: j.restricted == nil}
	s.CleanupScope = "session_job_members"
	s.DescendantsMayEscape = j.silentBreakaway
	if j.silentBreakaway {
		s.Notice = "Win7 兼容模式：仅主动加入会话 Job 的直接进程保证退出清理；第三方后代可脱离，不保证清理，也不视为异常。"
	}
	if j.restricted != nil {
		s.Restricted = true
		s.Reason = j.restricted.Error()
		s.Notice = "受限模式：不保证退出时的确定性清理；未纳入会话 Job 的命令不会启动"
	}
	return s
}

func (j *jobObjectRunner) emitMode(action string) {
	emitRuntimeEvent("process_mode", j.modeState(action))
}

func (j *jobObjectRunner) StartSession() error {
	j.jobMu.Lock()
	defer j.jobMu.Unlock()
	if j.closed {
		return errors.New("session Job is closed")
	}
	if j.sessionJob != 0 {
		return nil
	}
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return errors.New("CreateJobObject failed")
	}
	var info jobObjectExtendedLimit
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	version, versionErr := syscall.GetVersion()
	if versionErr != nil {
		procCloseHandle.Call(job)
		return fmt.Errorf("GetVersion: %w", versionErr)
	}
	j.silentBreakaway = version&0xff == 6 && (version>>8)&0xff == 1
	if j.silentBreakaway {
		info.BasicLimitInformation.LimitFlags |= jobObjectLimitSilentBreakawayOK
	}
	if j.MemLimitMB > 0 {
		info.BasicLimitInformation.LimitFlags |= jobObjectLimitJobMemory
		info.JobMemoryLimit = uintptr(j.MemLimitMB * 1024 * 1024)
	}
	if r1, _, callErr := procSetInformationJob.Call(job, jobObjectExtendedLimitInfoClass,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); r1 == 0 {
		procCloseHandle.Call(job)
		return win32CallError("SetInformationJobObject", callErr)
	}
	j.sessionJob = job
	j.processes = newProcessRecordsForJob(job)
	return nil
}

func (j *jobObjectRunner) Close() {
	j.jobMu.Lock()
	defer j.jobMu.Unlock()
	if j.closed {
		return
	}
	j.closed = true
	if j.sessionJob != 0 {
		j.processes.harvest(j.sessionJob)
		procCloseHandle.Call(j.sessionJob)
		j.sessionJob = 0
	}
}

// Interrupt kills only the current foreground command tree. Other children
// from earlier commands stay inside the session Job until session close.
func (j *jobObjectRunner) Interrupt() {
	if pid := atomic.LoadUintptr(&j.curPID); pid != 0 {
		if err := j.processes.terminateTree(uint32(pid)); err != nil {
			out("[进程终止失败] %v\n", err)
		}
	}
}

func (j *jobObjectRunner) publishProcess(process uintptr, pid uint32) {
	atomic.StoreUintptr(&j.curProcess, process)
	atomic.StoreUintptr(&j.curPID, uintptr(pid))
}

func (j *jobObjectRunner) clearProcess() {
	atomic.StoreUintptr(&j.curPID, 0)
	atomic.StoreUintptr(&j.curProcess, 0)
}

func (j *jobObjectRunner) Run(command string) (string, int, error) {
	if err := j.StartSession(); err != nil {
		return "", -1, err
	}
	j.jobMu.Lock()
	restricted := j.restricted
	job := j.sessionJob
	j.jobMu.Unlock()
	if restricted != nil {
		j.emitMode("state")
		return "", -1, restricted
	}
	rf, err := buildRunFiles(j.Home, j.Workspace, command)
	if err != nil {
		return "", -1, err
	}
	defer os.RemoveAll(rf.dir)

	process, thread, pid, err := createSuspendedBatch(rf.batPath)
	if err != nil {
		restricted = j.markRestricted(fmt.Errorf("containment unavailable: suspended process creation failed: %w", err))
		return "", -1, restricted
	}
	defer procCloseHandle.Call(thread)
	defer procCloseHandle.Call(process)

	assign := j.assignProcess
	if assign == nil {
		assign = assignNativeProcessToJob
	}
	if err := assign(job, process); err != nil {
		// The process is still suspended, so terminating it here proves no user
		// command or descendant could run outside our Job. Win7 nested Jobs are
		// unsupported unless the host permits breakaway; that case is blocked.
		terminateSuspendedProcess(process)
		restricted := j.markRestricted(fmt.Errorf("containment unavailable: Job assignment failed before command start: %w", err))
		return "", -1, restricted
	}
	// Publish only after the suspended process belongs to this Job. Interrupt
	// must never observe an empty Job and then miss the command being prepared.
	if err := j.processes.add(pid, command, "foreground", "", false); err != nil {
		terminateSuspendedProcess(process)
		return "", -1, err
	}
	defer func() {
		if err := j.processes.refresh(); err != nil {
			out("[进程登记失败] %v\n", err)
		}
	}()
	j.publishProcess(process, pid)
	defer j.clearProcess()
	if j.onAssigned != nil {
		j.onAssigned()
	}
	if j.beforeResume != nil {
		j.beforeResume()
	}
	if err := resumeNativeThread(thread); err != nil {
		terminateSuspendedProcess(process)
		syscall.WaitForSingleObject(syscall.Handle(process), infiniteWait)
		return "", -1, fmt.Errorf("containment unavailable: command resume failed: %w", err)
	}

	waitMS := durationMilliseconds(j.Timeout)
	event, waitErr := syscall.WaitForSingleObject(syscall.Handle(process), waitMS)
	if waitErr != nil {
		if err := j.processes.terminateTree(pid); err != nil {
			return "", -1, fmt.Errorf("wait failed: %v; terminate command tree: %w", waitErr, err)
		}
		syscall.WaitForSingleObject(syscall.Handle(process), infiniteWait)
		return "", -1, fmt.Errorf("WaitForSingleObject failed: %w", waitErr)
	}
	out, ec := readResult(filepath.Join(j.Home, ".pulse7", "run", rf.id))
	if event == syscall.WAIT_TIMEOUT {
		if err := j.processes.terminateTree(pid); err != nil {
			return out, ec, fmt.Errorf("TIMEOUT: command tree termination failed: %w", err)
		}
		syscall.WaitForSingleObject(syscall.Handle(process), infiniteWait)
		out, ec = readResult(filepath.Join(j.Home, ".pulse7", "run", rf.id))
		return out + "\n[TIMEOUT: job tree terminated]", ec, nil
	}
	if event != waitObject0 {
		if err := j.processes.terminateTree(pid); err != nil {
			return out, ec, fmt.Errorf("unexpected process wait result %d; terminate: %w", event, err)
		}
		return out, ec, fmt.Errorf("unexpected process wait result: %d", event)
	}
	return out, ec, nil
}

func createSuspendedBatch(batchPath string) (uintptr, uintptr, uint32, error) {
	commandProcessor := os.Getenv("COMSPEC")
	if commandProcessor == "" {
		root := os.Getenv("SystemRoot")
		if root != "" {
			commandProcessor = filepath.Join(root, "System32", "cmd.exe")
		} else {
			commandProcessor = "cmd.exe"
		}
	}
	appName, err := syscall.UTF16PtrFromString(commandProcessor)
	if err != nil {
		return 0, 0, 0, err
	}
	commandLine := syscall.EscapeArg(commandProcessor) + " /d /s /c " + syscall.EscapeArg(batchPath)
	commandLinePtr, err := syscall.UTF16PtrFromString(commandLine)
	if err != nil {
		return 0, 0, 0, err
	}
	var startup syscall.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var processInfo syscall.ProcessInformation
	err = syscall.CreateProcess(
		appName, commandLinePtr, nil, nil, false,
		createSuspended|createBreakawayFromJob|createNoWindow,
		nil, nil, &startup, &processInfo,
	)
	if err != nil {
		return 0, 0, 0, err
	}
	return uintptr(processInfo.Process), uintptr(processInfo.Thread), processInfo.ProcessId, nil
}

func terminateProcessTree(pid uint32) error {
	cmd := exec.Command("taskkill.exe", "/PID", fmt.Sprint(pid), "/T", "/F")
	cmd.SysProcAttr = sysProcHidden()
	result, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill PID=%d /T /F: %w: %s", pid, err, strings.TrimSpace(decodeShellOutput(result)))
	}
	return nil
}

func assignNativeProcessToJob(job, process uintptr) error {
	if r1, _, callErr := procAssignProcessToJob.Call(job, process); r1 == 0 {
		return win32CallError("AssignProcessToJobObject", callErr)
	}
	return nil
}

func resumeNativeThread(thread uintptr) error {
	if r1, _, callErr := procResumeThread.Call(thread); uint32(r1) == 0xffffffff {
		return win32CallError("ResumeThread", callErr)
	}
	return nil
}

func terminateSuspendedProcess(process uintptr) {
	syscall.TerminateProcess(syscall.Handle(process), 1)
	syscall.WaitForSingleObject(syscall.Handle(process), infiniteWait)
}

func durationMilliseconds(d time.Duration) uint32 {
	if d <= 0 {
		return infiniteWait
	}
	ms := d / time.Millisecond
	if ms < 1 {
		return 1
	}
	if ms >= time.Duration(infiniteWait) {
		return infiniteWait - 1
	}
	return uint32(ms)
}

func win32CallError(name string, callErr error) error {
	if callErr == nil || errors.Is(callErr, syscall.Errno(0)) || strings.TrimSpace(callErr.Error()) == "The operation completed successfully." {
		return errors.New(name + " failed")
	}
	return fmt.Errorf("%s failed: %w", name, callErr)
}
