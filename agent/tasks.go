package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	defaultBackgroundTaskMaxOutputMB = 50
	defaultBackgroundTaskWarnCount   = 5
	defaultBackgroundTaskWarnSec     = 3600
	defaultProcessWarnThreshold      = 50
)

func validateBackgroundTaskConfig(maxOutputMB, warnCount, warnSec, processWarn int) error {
	if maxOutputMB <= 0 {
		return errors.New("background-task-max-output-mb must be greater than zero")
	}
	if warnCount < 0 {
		return errors.New("background-task-warn-count must not be negative")
	}
	if warnSec < 0 {
		return errors.New("background-task-warn-sec must not be negative")
	}
	if processWarn < 0 {
		return errors.New("process-warn-threshold must not be negative")
	}
	return nil
}

type processStartSpec struct {
	Command     string
	TaskID      string
	Exe         string
	Workspace   string
	CommandFile string
	OutputFile  string
	StatusFile  string
	PIDFile     string
	MaxBytes    int64
}

type managedProcess interface {
	PID() int
	Wait() (int, error)
	KillTree() error
}

type managedProcessRunner interface {
	StartManaged(processStartSpec, bool) (managedProcess, error)
	ProcessCount() (int, error)
}

type backgroundTask struct {
	done       chan struct{}
	ID         string
	Command    string
	OutputPath string
	StatusPath string
	PIDPath    string
	StartedAt  time.Time
	Status     string
	ExitCode   int
	PID        int
	Detached   bool
	proc       managedProcess
}

type backgroundTaskManager struct {
	mu                   sync.Mutex
	runner               managedProcessRunner
	dir                  string
	workspace            string
	maxOutputBytes       int64
	warnCount            int
	warnDuration         time.Duration
	foregroundTimeout    time.Duration
	processWarnThreshold int
	tasks                map[string]*backgroundTask
}

func newBackgroundTaskManager(runner sandboxRunner, dir, workspace string) *backgroundTaskManager {
	managed, _ := runner.(managedProcessRunner)
	return &backgroundTaskManager{
		runner: managed, dir: dir, workspace: workspace,
		maxOutputBytes: int64(defaultBackgroundTaskMaxOutputMB) * 1024 * 1024,
		warnCount:      defaultBackgroundTaskWarnCount, warnDuration: defaultBackgroundTaskWarnSec * time.Second,
		processWarnThreshold: defaultProcessWarnThreshold, tasks: map[string]*backgroundTask{},
	}
}

func (m *backgroundTaskManager) configure(maxOutputMB, warnCount, warnSec, processWarn int, foregroundTimeout time.Duration) {
	m.maxOutputBytes = int64(maxOutputMB) * 1024 * 1024
	m.warnCount = warnCount
	m.warnDuration = time.Duration(warnSec) * time.Second
	m.processWarnThreshold = processWarn
	m.foregroundTimeout = foregroundTimeout
}

func (m *backgroundTaskManager) launch(command string, detached bool) (*backgroundTask, error) {
	if m.runner == nil {
		return nil, errors.New("selected sandbox runner does not support managed tasks")
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, err
	}
	// Windows reserves the identity before files or registration exist. LUIDs
	// remain unique until reboot, including across manager reconstruction.
	dll, err := syscall.LoadDLL("advapi32.dll")
	if err != nil {
		return nil, fmt.Errorf("allocate task ID: %w", err)
	}
	defer dll.Release()
	allocate, err := dll.FindProc("AllocateLocallyUniqueId")
	if err != nil {
		return nil, fmt.Errorf("allocate task ID: %w", err)
	}
	var luid uint64
	if ok, _, callErr := allocate.Call(uintptr(unsafe.Pointer(&luid))); ok == 0 {
		return nil, fmt.Errorf("AllocateLocallyUniqueId: %w", callErr)
	}
	id := "task-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(luid, 36)
	commandPath := filepath.Join(m.dir, id+".cmd")
	commandBytes := append([]byte("@echo off\r\n"), utf8ToCodepage(command, consoleCodepage())...)
	commandBytes = append(commandBytes, '\r', '\n')
	if err := os.WriteFile(commandPath, commandBytes, 0o644); err != nil {
		return nil, err
	}
	spec := processStartSpec{
		Command: command, TaskID: id,
		Exe: currentExecutable(), Workspace: m.workspace, CommandFile: commandPath,
		OutputFile: filepath.Join(m.dir, id+".output"), StatusFile: filepath.Join(m.dir, id+".status.json"),
		PIDFile: filepath.Join(m.dir, id+".pid"), MaxBytes: m.maxOutputBytes,
	}
	proc, err := m.runner.StartManaged(spec, detached)
	if err != nil {
		return nil, err
	}
	workerPID, err := waitTaskPID(spec.PIDFile, 10*time.Second)
	if err != nil {
		_ = proc.KillTree()
		return nil, fmt.Errorf("background task startup handshake failed: %w", err)
	}
	task := &backgroundTask{
		done: make(chan struct{}),
		ID:   id, Command: command, OutputPath: spec.OutputFile, StatusPath: spec.StatusFile,
		PIDPath: spec.PIDFile, StartedAt: time.Now(), Status: "running", ExitCode: -1,
		PID: workerPID, Detached: detached, proc: proc,
	}
	m.mu.Lock()
	m.tasks[id] = task
	running := m.runningCountLocked()
	m.mu.Unlock()
	emitRuntimeEvent("background_task", backgroundTaskEvent{
		Action: "start", TaskID: id, PID: intPointer(task.PID), Detached: boolPointer(detached), Command: command,
	})
	if detached {
		out("[detached] 该进程将在 pulse7 退出后继续运行：pid=%d command=%s\n", task.PID, command)
	}
	if running > m.warnCount {
		if _, native := m.runner.(*jobObjectRunner); native {
			emitRuntimeEvent("process_warning", processWarningEvent{Kind: "background_count", Current: running, Threshold: m.warnCount})
		}
		out("[警告] 后台任务并发数=%d，已超过告警阈值=%d；任务仍已启动。\n", running, m.warnCount)
	}
	m.warnProcessCount()
	return task, nil
}

func (m *backgroundTaskManager) start(command string, detached bool) (*backgroundTask, error) {
	task, err := m.launch(command, detached)
	if err != nil {
		return nil, err
	}
	go m.observe(task)
	if m.warnDuration > 0 {
		go m.warnIfLongRunning(task.ID, m.warnDuration)
	}
	return task, nil
}

var (
	activeManagedMu sync.Mutex
	activeManaged   managedProcess
)

func interruptActiveManagedProcess() {
	activeManagedMu.Lock()
	process := activeManaged
	activeManagedMu.Unlock()
	if process != nil {
		_ = process.KillTree()
	}
}

func (m *backgroundTaskManager) runForeground(command string, detached bool) (string, int, error) {
	task, err := m.launch(command, detached)
	if err != nil {
		return "", -1, err
	}
	activeManagedMu.Lock()
	activeManaged = task.proc
	activeManagedMu.Unlock()
	defer func() {
		activeManagedMu.Lock()
		activeManaged = nil
		activeManagedMu.Unlock()
	}()
	done := make(chan processWaitResult, 1)
	go func() {
		code, waitErr := task.proc.Wait()
		done <- processWaitResult{exitCode: code, err: waitErr}
	}()
	var result processWaitResult
	if m.foregroundTimeout > 0 {
		select {
		case result = <-done:
		case <-time.After(m.foregroundTimeout):
			_ = task.proc.KillTree()
			result = <-done
			m.finish(task, result)
			b, _ := os.ReadFile(task.OutputPath)
			return decodeShellOutput(b) + "\n[TIMEOUT: detached command tree terminated]", result.exitCode, nil
		}
	} else {
		result = <-done
	}
	m.finish(task, result)
	b, readErr := os.ReadFile(task.OutputPath)
	if readErr != nil {
		return "", result.exitCode, readErr
	}
	return decodeShellOutput(b), result.exitCode, result.err
}

func currentExecutable() string {
	exe, _ := os.Executable()
	return exe
}

func (m *backgroundTaskManager) observe(task *backgroundTask) {
	if task.done != nil {
		defer close(task.done)
	}
	if _, native := m.runner.(*jobObjectRunner); native {
		m.observeNative(task)
		return
	}
	exitCode, waitErr := task.proc.Wait()
	m.finish(task, processWaitResult{exitCode: exitCode, err: waitErr})
}

func (m *backgroundTaskManager) finish(task *backgroundTask, result processWaitResult) {
	m.mu.Lock()
	wasKilled := task.Status == "killed"
	m.mu.Unlock()
	if j, ok := m.runner.(*jobObjectRunner); ok && j.processes != nil && j.processes.taskTerminated(task.ID) {
		wasKilled = true
	}
	status := "exited"
	if wasKilled {
		status = "killed"
	} else if outputWasTruncated(task.OutputPath) {
		status = "output_truncated"
	} else if result.err != nil {
		status = "failed"
	}
	m.mu.Lock()
	task.Status = status
	task.ExitCode = result.exitCode
	m.mu.Unlock()
	emitRuntimeEvent("background_task", backgroundTaskEvent{
		Action: "end", TaskID: task.ID, PID: intPointer(task.PID), Status: status, ExitCode: intPointer(result.exitCode),
	})
}

func (m *backgroundTaskManager) waitHarvested() {
	if _, native := m.runner.(*jobObjectRunner); !native {
		return
	}
	m.mu.Lock()
	var done []chan struct{}
	for _, task := range m.tasks {
		if !task.Detached && task.done != nil {
			done = append(done, task.done)
		}
	}
	m.mu.Unlock()
	for _, ch := range done {
		<-ch
	}
}

func (m *backgroundTaskManager) warnIfLongRunning(id string, after time.Duration) {
	time.Sleep(after)
	m.mu.Lock()
	task := m.tasks[id]
	running := task != nil && task.Status == "running"
	m.mu.Unlock()
	if running {
		if _, native := m.runner.(*jobObjectRunner); native {
			emitRuntimeEvent("process_warning", processWarningEvent{Kind: "background_duration", Current: int(after / time.Second), Threshold: int(m.warnDuration / time.Second), TaskID: id})
		}
		out("[警告] 后台任务 %s 已运行 %v，任务继续运行。\n", id, after)
	}
}

func (m *backgroundTaskManager) warnProcessCount() {
	if m.runner == nil || m.processWarnThreshold <= 0 {
		return
	}
	count, err := m.runner.ProcessCount()
	if _, native := m.runner.(*jobObjectRunner); native {
		if err != nil {
			out("[会话进程计数失败] %v\n", err)
			emitRuntimeEvent("process_count_error", map[string]string{"error": err.Error()})
			return
		}
		if count > m.processWarnThreshold {
			emitRuntimeEvent("process_warning", processWarningEvent{Kind: "process_count", Current: count, Threshold: m.processWarnThreshold})
			out("[进程清理建议] 使用 /tasks 查看进程，由用户或模型决定是否终止。\n")
		}
	}
	if err == nil && count > m.processWarnThreshold {
		out("[警告] 会话进程数=%d，已超过告警阈值=%d；新进程未被阻止。\n", count, m.processWarnThreshold)
	}
}

func (m *backgroundTaskManager) runningCountLocked() int {
	n := 0
	for _, task := range m.tasks {
		if task.Status == "running" {
			n++
		}
	}
	return n
}

func (m *backgroundTaskManager) get(id string) (*backgroundTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[id]
	if task == nil {
		return nil, fmt.Errorf("background task %q not found", id)
	}
	return task, nil
}

func (m *backgroundTaskManager) output(id string, offset, limit int) (string, error) {
	task, err := m.get(id)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(task.OutputPath)
	if errors.Is(err, os.ErrNotExist) {
		b = nil
	} else if err != nil {
		return "", err
	}
	if offset < 0 || offset > len(b) {
		return "", fmt.Errorf("offset %d outside output size %d", offset, len(b))
	}
	if limit <= 0 {
		limit = 4096
	}
	if limit > 1<<20 {
		limit = 1 << 20
	}
	end := offset + limit
	if end > len(b) {
		end = len(b)
	}
	m.mu.Lock()
	status, exitCode := task.Status, task.ExitCode
	m.mu.Unlock()
	_, native := m.runner.(*jobObjectRunner)
	if end > offset && !native {
		emitRuntimeEvent("background_task", backgroundTaskEvent{
			Action: "output", TaskID: id, Status: status, ExitCode: intPointer(exitCode),
			Offset: offset, NextOffset: end,
		})
	}
	return fmt.Sprintf("task_id=%s status=%s exitcode=%d offset=%d next_offset=%d total_bytes=%d\n%s",
		id, status, exitCode, offset, end, len(b), decodeShellOutput(b[offset:end])), nil
}

func (m *backgroundTaskManager) kill(id string) (string, error) {
	task, err := m.get(id)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	if task.Status != "running" {
		status := task.Status
		m.mu.Unlock()
		return "", fmt.Errorf("background task %s is not running (status=%s)", id, status)
	}
	task.Status = "killed"
	m.mu.Unlock()
	// The native runner's handle is the worker itself: kill its tree once.
	// Sandboxie's launcher and worker remain on the existing two-target path.
	if _, native := m.runner.(*jobObjectRunner); !native {
		terminateProcessTree(uint32(task.PID))
	}
	if err := task.proc.KillTree(); err != nil {
		return "", err
	}
	return fmt.Sprintf("task %s termination requested", id), nil
}

func waitTaskPID(path string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		b, err := os.ReadFile(path)
		if err == nil {
			n, parseErr := strconv.Atoi(strings.TrimSpace(string(b)))
			if parseErr == nil && n > 0 {
				return n, nil
			}
			lastErr = fmt.Errorf("invalid worker PID in %s", path)
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return 0, lastErr
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func outputWasTruncated(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), taskOutputTruncatedMarker)
}

func (m *backgroundTaskManager) list() string {
	overview := fmt.Sprintf("session_processes=unavailable threshold=%d", m.processWarnThreshold)
	mode := ""
	if j, ok := m.runner.(*jobObjectRunner); ok {
		s := j.modeState("state")
		emitRuntimeEvent("process_mode", s)
		mode = fmt.Sprintf("mode=%s restricted=%v cleanup_guaranteed=%v %s\n", s.Mode, s.Restricted, s.CleanupGuaranteed, s.Notice)
		if j.processes != nil {
			mode += j.processes.list() + j.processes.previousNotice() + "\n"
		}
	}
	if m.runner != nil {
		if count, err := m.runner.ProcessCount(); err == nil {
			exceeded := m.processWarnThreshold > 0 && count > m.processWarnThreshold
			overview = fmt.Sprintf("session_processes=%d threshold=%d exceeded=%v", count, m.processWarnThreshold, exceeded)
		} else {
			overview += " error=" + err.Error()
		}
	}
	m.mu.Lock()
	items := make([]*backgroundTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		copy := *task
		items = append(items, &copy)
	}
	m.mu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.Before(items[j].StartedAt) })
	if len(items) == 0 {
		return mode + overview + "\nno background tasks"
	}
	var b strings.Builder
	b.WriteString(mode)
	b.WriteString(overview)
	b.WriteByte('\n')
	for _, task := range items {
		size := int64(0)
		if info, err := os.Stat(task.OutputPath); err == nil {
			size = info.Size()
		}
		fmt.Fprintf(&b, "%s pid=%d status=%s runtime=%v output=%dB command=%s\n",
			task.ID, task.PID, task.Status, time.Since(task.StartedAt).Round(time.Second), size, task.Command)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *backgroundTaskManager) exitSummary() {
	if j, ok := m.runner.(*jobObjectRunner); ok && j.processes != nil {
		j.processes.exitSummary()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, task := range m.tasks {
		if task.Detached && task.Status == "running" {
			out("[退出保留 detached] pid=%d command=%s\n", task.PID, task.Command)
		} else if !task.Detached && task.Status == "running" {
			out("[退出收割后台任务] id=%s pid=%d command=%s\n", task.ID, task.PID, task.Command)
		}
	}
}

func (r *Registry) toolTaskOutput(argsJSON string) (string, error) {
	var args struct {
		TaskID string `json:"task_id"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", err
	}
	return r.tasks.output(args.TaskID, args.Offset, args.Limit)
}

func (r *Registry) toolTaskKill(argsJSON string) (string, error) {
	if err := r.ensureMutable("task_kill"); err != nil {
		return "", err
	}
	var args struct {
		TaskID  string               `json:"task_id"`
		Process *processIdentityArgs `json:"process"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", err
	}
	if args.Process != nil {
		if args.TaskID != "" {
			return "", errors.New("provide task_id or process, not both")
		}
		j, ok := r.runner.(*jobObjectRunner)
		if !ok || j.processes == nil {
			return "", errors.New("process identity termination requires JobObject mode")
		}
		return j.processes.killIdentity(*args.Process)
	}
	return r.tasks.kill(args.TaskID)
}
