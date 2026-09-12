package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var queryFullProcessImageName = k32.NewProc("QueryFullProcessImageNameW")

// CreationTime is an exact decimal FILETIME, not a lossy JavaScript number.
type processRecord struct {
	PID            uint32    `json:"pid"`
	CreationTime   string    `json:"creationTime"`
	ImagePath      string    `json:"imagePath"`
	Command        string    `json:"command"`
	StartedAt      time.Time `json:"startedAt"`
	Status         string    `json:"status"`
	Source         string    `json:"source"`
	TaskID         string    `json:"taskId,omitempty"`
	Detached       bool      `json:"detached"`
	SessionManaged bool      `json:"sessionManaged"`
	ParentPID      uint32    `json:"parentPid,omitempty"`
	handle         syscall.Handle
	created        uint64
	exited         uint64
}

type processEvent struct {
	Action  string        `json:"action"`
	Process processRecord `json:"process"`
}

type processRecords struct {
	job      uintptr
	dir      string
	previous []processRecord
	mu       sync.Mutex
	records  []*processRecord
	stop     chan struct{}
	done     chan struct{}
}

func filetimeValue(t syscall.Filetime) uint64 {
	return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime)
}

func processIdentity(h syscall.Handle, pid uint32) (processRecord, error) {
	var c, e, k, u syscall.Filetime
	if err := syscall.GetProcessTimes(h, &c, &e, &k, &u); err != nil {
		return processRecord{}, fmt.Errorf("GetProcessTimes PID=%d: %w", pid, err)
	}
	name := make([]uint16, 32768)
	size := uint32(len(name))
	if ok, _, err := queryFullProcessImageName.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&size))); ok == 0 {
		return processRecord{}, win32CallError("QueryFullProcessImageNameW", err)
	}
	value := filetimeValue(c)
	return processRecord{PID: pid, CreationTime: strconv.FormatUint(value, 10), ImagePath: syscall.UTF16ToString(name[:size]), created: value}, nil
}

func newProcessRecords() *processRecords {
	return newProcessRecordsForJob(0)
}

func newProcessRecordsForJob(job uintptr) *processRecords {
	r := &processRecords{job: job, stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(r.done)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-r.stop:
				return
			case <-tick.C:
				if err := r.refresh(); err != nil {
					out("[进程登记失败] %v\n", err)
				}
			}
		}
	}()
	return r
}

func (r *processRecords) isSessionMember(h syscall.Handle) (bool, error) {
	if r.job == 0 {
		return false, nil
	}
	var member int32
	if ok, _, err := k32.NewProc("IsProcessInJob").Call(uintptr(h), r.job, uintptr(unsafe.Pointer(&member))); ok == 0 {
		return false, win32CallError("IsProcessInJob", err)
	}
	return member != 0, nil
}

func (r *processRecords) add(pid uint32, command, source, taskID string, detached bool) error {
	h, err := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE, false, pid)
	if err != nil {
		return fmt.Errorf("open process identity PID=%d: %w", pid, err)
	}
	p, err := processIdentity(h, pid)
	if err != nil {
		syscall.CloseHandle(h)
		return err
	}
	p.Command = command
	p.SessionManaged, err = r.isSessionMember(h)
	if err != nil {
		syscall.CloseHandle(h)
		return err
	}
	p.Source = source
	p.TaskID = taskID
	p.Detached = detached
	p.StartedAt = time.Now()
	p.Status = "running"
	p.handle = h
	if detached {
		p.Status = "detached"
	}
	r.mu.Lock()
	r.records = append(r.records, &p)
	if detached {
		if err := r.saveDetachedLocked(); err != nil {
			r.mu.Unlock()
			return fmt.Errorf("persist detached identity: %w", err)
		}
	}
	r.mu.Unlock()
	emitRuntimeEvent("process", processEvent{Action: "created", Process: p})
	return nil
}

func processTable() ([]syscall.ProcessEntry32, error) {
	h, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)
	entry := syscall.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	var entries []syscall.ProcessEntry32
	for err = syscall.Process32First(h, &entry); err == nil; err = syscall.Process32Next(h, &entry) {
		entries = append(entries, entry)
	}
	if err != syscall.ERROR_NO_MORE_FILES {
		return nil, err
	}
	return entries, nil
}

func (r *processRecords) refresh() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Read exit times before resolving ancestry: an old parent's reused PID
	// must not attach an unrelated process to this session.
	for _, p := range r.records {
		if p.handle != 0 {
			event, err := syscall.WaitForSingleObject(p.handle, 0)
			if err != nil {
				return err
			}
			if event == waitObject0 {
				var c, e, k, u syscall.Filetime
				if err := syscall.GetProcessTimes(p.handle, &c, &e, &k, &u); err != nil {
					return err
				}
				p.exited = filetimeValue(e)
				syscall.CloseHandle(p.handle)
				p.handle = 0
				if p.Status != "terminated" {
					p.Status = "exited"
					emitRuntimeEvent("process", processEvent{Action: "ended", Process: *p})
				}
			}
		}
	}
	entries, err := processTable()
	if err != nil {
		return err
	}
	for added := true; added; {
		added = false
		for _, entry := range entries {
			known := false
			for _, p := range r.records {
				if p.PID == entry.ProcessID && p.handle != 0 {
					known = true
					break
				}
			}
			if known {
				continue
			}
			var candidates []*processRecord
			for _, p := range r.records {
				if p.PID == entry.ParentProcessID {
					candidates = append(candidates, p)
				}
			}
			if len(candidates) == 0 {
				continue
			}
			h, openErr := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE, false, entry.ProcessID)
			if openErr != nil {
				return fmt.Errorf("observe descendant PID=%d: %w", entry.ProcessID, openErr)
			}
			child, idErr := processIdentity(h, entry.ProcessID)
			if idErr != nil {
				syscall.CloseHandle(h)
				return idErr
			}
			var parent *processRecord
			for _, p := range candidates {
				if child.created >= p.created && (p.exited == 0 || child.created <= p.exited) {
					parent = p
					break
				}
			}
			if parent == nil {
				syscall.CloseHandle(h)
				continue
			}
			duplicate := false
			for _, p := range r.records {
				if p.PID == child.PID && p.CreationTime == child.CreationTime {
					duplicate = true
					break
				}
			}
			if duplicate {
				syscall.CloseHandle(h)
				continue
			}
			child.Command = parent.Command
			child.SessionManaged, idErr = r.isSessionMember(h)
			if idErr != nil {
				syscall.CloseHandle(h)
				return idErr
			}
			child.Source = parent.Source
			child.TaskID = parent.TaskID
			child.Detached = parent.Detached
			child.ParentPID = parent.PID
			child.StartedAt = time.Now()
			child.Status = "running"
			child.handle = h
			if child.Detached {
				child.Status = "detached"
			}
			r.records = append(r.records, &child)
			added = true
			emitRuntimeEvent("process", processEvent{Action: "created", Process: child})
		}
	}
	return nil
}

func (r *processRecords) snapshot() ([]processRecord, error) {
	if err := r.refresh(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]processRecord, 0, len(r.records))
	for _, p := range r.records {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartedAt.Before(result[j].StartedAt) })
	return result, nil
}

func (r *processRecords) list() string {
	records, err := r.snapshot()
	if err != nil {
		return "process_registry_error=" + err.Error()
	}
	var b strings.Builder
	for _, p := range records {
		fmt.Fprintf(&b, "process pid=%d creation_time=%s image_path=%q source=%s task_id=%s status=%s session_managed=%v started=%s command=%s\n", p.PID, p.CreationTime, p.ImagePath, p.Source, p.TaskID, p.Status, p.SessionManaged, p.StartedAt.Format(time.RFC3339Nano), p.Command)
	}
	return b.String()
}

func (r *processRecords) close() {
	close(r.stop)
	<-r.done
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.records {
		if p.handle != 0 {
			syscall.CloseHandle(p.handle)
			p.handle = 0
		}
	}
}

func (r *processRecords) terminateTree(pid uint32) error {
	if err := r.refresh(); err != nil {
		return fmt.Errorf("capture command tree before termination: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := terminateProcessTree(pid); err != nil {
		return err
	}
	targets := map[uint32]bool{pid: true}
	for changed := true; changed; {
		changed = false
		for _, p := range r.records {
			if p.handle != 0 && targets[p.ParentPID] && !targets[p.PID] {
				targets[p.PID] = true
				changed = true
			}
		}
	}
	for _, p := range r.records {
		if p.handle != 0 && targets[p.PID] {
			p.Status = "terminated"
			emitRuntimeEvent("process", processEvent{Action: "terminated", Process: *p})
		}
	}
	return nil
}

func (r *processRecords) taskTerminated(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.records {
		if p.TaskID == id && p.Status == "terminated" {
			return true
		}
	}
	return false
}
