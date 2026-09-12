package main

import (
	"encoding/binary"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	procGetExitCodeProcess        = k32.NewProc("GetExitCodeProcess")
	procQueryInformationJobObject = k32.NewProc("QueryInformationJobObject")
)

const (
	jobObjectBasicAccountingInfoClass = 1
	createNewProcessGroup             = 0x00000200
	detachedProcess                   = 0x00000008
)

func workerArgs(spec processStartSpec) []string {
	return []string{
		"task-worker",
		"--command-file", spec.CommandFile,
		"--output-file", spec.OutputFile,
		"--status-file", spec.StatusFile,
		"--pid-file", spec.PIDFile,
		"--workspace", spec.Workspace,
		"--max-bytes", strconv.FormatInt(spec.MaxBytes, 10),
	}
}

type processWaitResult struct {
	exitCode int
	err      error
}

type nativeManagedProcess struct {
	records *processRecords
	pid     uint32
	done    chan processWaitResult
}

func newNativeManagedProcess(process uintptr, pid uint32) *nativeManagedProcess {
	p := &nativeManagedProcess{pid: pid, done: make(chan processWaitResult, 1)}
	go func() {
		_, waitErr := syscall.WaitForSingleObject(syscall.Handle(process), infiniteWait)
		var code uint32
		if r1, _, callErr := procGetExitCodeProcess.Call(process, uintptr(unsafe.Pointer(&code))); r1 == 0 && waitErr == nil {
			waitErr = win32CallError("GetExitCodeProcess", callErr)
		}
		procCloseHandle.Call(process)
		p.done <- processWaitResult{exitCode: int(code), err: waitErr}
	}()
	return p
}

func (p *nativeManagedProcess) PID() int { return int(p.pid) }
func (p *nativeManagedProcess) Wait() (int, error) {
	result := <-p.done
	return result.exitCode, result.err
}
func (p *nativeManagedProcess) KillTree() error {
	if p.records != nil {
		return p.records.terminateTree(p.pid)
	}
	return terminateProcessTree(p.pid)
}

type execManagedProcess struct {
	cmd  *exec.Cmd
	done chan processWaitResult
}

func startExecManaged(cmd *exec.Cmd) (*execManagedProcess, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &execManagedProcess{cmd: cmd, done: make(chan processWaitResult, 1)}
	go func() {
		err := cmd.Wait()
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		p.done <- processWaitResult{exitCode: code, err: err}
	}()
	return p, nil
}

func (p *execManagedProcess) PID() int { return p.cmd.Process.Pid }
func (p *execManagedProcess) Wait() (int, error) {
	result := <-p.done
	return result.exitCode, result.err
}
func (p *execManagedProcess) KillTree() error {
	terminateProcessTree(uint32(p.PID()))
	return nil
}

func startDetachedManaged(spec processStartSpec) (managedProcess, error) {
	cmd := exec.Command(spec.Exe, workerArgs(spec)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true, CreationFlags: createBreakawayFromJob | createNewProcessGroup | detachedProcess,
	}
	return startExecManaged(cmd)
}

func (j *jobObjectRunner) StartManaged(spec processStartSpec, detached bool) (managedProcess, error) {
	if err := j.StartSession(); err != nil {
		return nil, err
	}
	j.jobMu.Lock()
	job, restricted := j.sessionJob, j.restricted
	j.jobMu.Unlock()
	if restricted != nil && !detached {
		j.emitMode("state")
		return nil, restricted
	}
	process, thread, pid, err := createSuspendedApplication(spec.Exe, workerArgs(spec))
	if err != nil {
		return nil, j.markRestricted(fmt.Errorf("containment unavailable: suspended task creation failed: %w", err))
	}
	if !detached {
		if err := assignNativeProcessToJob(job, process); err != nil {
			terminateSuspendedProcess(process)
			procCloseHandle.Call(process)
			procCloseHandle.Call(thread)
			restricted = j.markRestricted(fmt.Errorf("containment unavailable: Job assignment failed before command start: %w", err))
			return nil, restricted
		}
	}
	source := "background"
	if detached {
		source = "detached"
	}
	if err := j.processes.add(pid, spec.Command, source, spec.TaskID, detached); err != nil {
		terminateSuspendedProcess(process)
		procCloseHandle.Call(thread)
		procCloseHandle.Call(process)
		return nil, err
	}
	if err := resumeNativeThread(thread); err != nil {
		terminateSuspendedProcess(process)
		procCloseHandle.Call(process)
		procCloseHandle.Call(thread)
		return nil, err
	}
	procCloseHandle.Call(thread)
	managed := newNativeManagedProcess(process, pid)
	managed.records = j.processes
	return managed, nil
}

func (j *jobObjectRunner) ProcessCount() (int, error) {
	j.jobMu.Lock()
	job := j.sessionJob
	j.jobMu.Unlock()
	if job == 0 {
		return 0, nil
	}
	var data [48]byte
	if r1, _, callErr := procQueryInformationJobObject.Call(job, jobObjectBasicAccountingInfoClass,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0); r1 == 0 {
		return 0, win32CallError("QueryInformationJobObject", callErr)
	}
	return int(binary.LittleEndian.Uint32(data[40:44])), nil
}

func (s *sbxRunner) StartManaged(spec processStartSpec, detached bool) (managedProcess, error) {
	if detached {
		return startDetachedManaged(spec)
	}
	args := []string{"/silent", "/box:" + s.Box, "/wait", spec.Exe}
	args = append(args, workerArgs(spec)...)
	cmd := exec.Command(s.StartExe, args...)
	cmd.SysProcAttr = sysProcHidden()
	return startExecManaged(cmd)
}

func (s *sbxRunner) ProcessCount() (int, error) {
	out, err := exec.Command(s.StartExe, "/box:"+s.Box, "/listpids").CombinedOutput()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, field := range strings.Fields(string(out)) {
		if _, err := strconv.Atoi(field); err == nil {
			count++
		}
	}
	return count, nil
}

func createSuspendedApplication(application string, args []string) (uintptr, uintptr, uint32, error) {
	appName, err := syscall.UTF16PtrFromString(application)
	if err != nil {
		return 0, 0, 0, err
	}
	parts := []string{syscall.EscapeArg(application)}
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	commandLinePtr, err := syscall.UTF16PtrFromString(strings.Join(parts, " "))
	if err != nil {
		return 0, 0, 0, err
	}
	var startup syscall.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var processInfo syscall.ProcessInformation
	err = syscall.CreateProcess(appName, commandLinePtr, nil, nil, false,
		createSuspended|createBreakawayFromJob|createNoWindow, nil, nil, &startup, &processInfo)
	if err != nil {
		return 0, 0, 0, err
	}
	return uintptr(processInfo.Process), uintptr(processInfo.Thread), processInfo.ProcessId, nil
}
