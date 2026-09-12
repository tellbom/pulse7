package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type processIdentityArgs struct {
	PID          uint32 `json:"pid"`
	CreationTime string `json:"creation_time"`
	ImagePath    string `json:"image_path"`
}

func sameProcessIdentity(p processRecord, a processIdentityArgs) bool {
	return p.PID == a.PID && p.CreationTime == a.CreationTime && strings.EqualFold(filepath.Clean(p.ImagePath), filepath.Clean(a.ImagePath))
}

func (r *processRecords) loadDetached(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dir = dir
	b, err := os.ReadFile(filepath.Join(dir, "detached.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &r.previous); err != nil {
		return fmt.Errorf("read detached history: %w", err)
	}
	for _, p := range r.previous {
		if p.PID == 0 || p.CreationTime == "" || p.ImagePath == "" {
			return errors.New("detached history contains incomplete process identity")
		}
	}
	// H4: deliberately do not open, probe or terminate historical PIDs here.
	return nil
}

func (r *processRecords) previousNotice() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.previous) == 0 {
		return ""
	}
	b, _ := json.Marshal(r.previous)
	return "上一会话留下的 detached 记录，未经核实；启动时不会自动终止。终止须通过 task_kill 的 process 参数核对 PID、creation_time、image_path 三项。\n" + string(b)
}

// Called with mu held. Store old unverified records and current live detached
// identities; old records are removed only by a successful explicit kill.
func (r *processRecords) saveDetachedLocked() error {
	if r.dir == "" {
		return nil
	}
	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return err
	}
	records := append([]processRecord{}, r.previous...)
	for _, p := range r.records {
		if p.Detached && p.handle != 0 && p.Status != "terminated" {
			records = append(records, *p)
		}
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.dir, "detached.json"), append(b, '\n'), 0644)
}

func (r *processRecords) killIdentity(a processIdentityArgs) (string, error) {
	failure := func(reason string) (string, error) {
		return "", fmt.Errorf("process_identity_mismatch: 无法确认该 PID 仍是原进程：%s", reason)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var recorded *processRecord
	for _, p := range r.records {
		if sameProcessIdentity(*p, a) {
			recorded = p
			break
		}
	}
	for i := range r.previous {
		if sameProcessIdentity(r.previous[i], a) {
			recorded = &r.previous[i]
			break
		}
	}
	if recorded == nil {
		return failure("请求三元组与已登记记录不匹配")
	}
	// Verify and terminate through this same handle; never reopen by PID after
	// verification and never delegate historical termination to taskkill.
	h, err := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE|syscall.PROCESS_TERMINATE, false, a.PID)
	if err != nil {
		return failure(err.Error())
	}
	defer syscall.CloseHandle(h)
	actual, err := processIdentity(h, a.PID)
	if err != nil {
		return failure(err.Error())
	}
	if !sameProcessIdentity(actual, a) {
		return failure("当前进程创建时间或映像路径不匹配")
	}
	state, err := syscall.WaitForSingleObject(h, 0)
	if err != nil || state != syscall.WAIT_TIMEOUT {
		return failure("进程已经结束或当前状态无法读取")
	}
	if err := syscall.TerminateProcess(h, 1); err != nil {
		return "", fmt.Errorf("terminate verified process: %w", err)
	}
	if _, err := syscall.WaitForSingleObject(h, infiniteWait); err != nil {
		return "", err
	}
	recorded.Status = "terminated"
	emitRuntimeEvent("process", processEvent{Action: "terminated", Process: *recorded})
	for i := len(r.previous) - 1; i >= 0; i-- {
		if sameProcessIdentity(r.previous[i], a) {
			r.previous = append(r.previous[:i], r.previous[i+1:]...)
		}
	}
	if err := r.saveDetachedLocked(); err != nil {
		return "", fmt.Errorf("process terminated; persist detached history failed: %w", err)
	}
	return fmt.Sprintf("process terminated: pid=%d creation_time=%s image_path=%q", a.PID, a.CreationTime, a.ImagePath), nil
}

func (r *processRecords) exitSummary() {
	records, err := r.snapshot()
	if err != nil {
		out("[退出进程清单采集失败] %v\n", err)
	}
	for _, p := range records {
		if p.handle != 0 {
			label := "退出待收割进程"
			if p.Detached {
				label = "退出保留 detached"
			} else if !p.SessionManaged {
				label = "会话 Job 外进程：不保证退出清理，非异常"
			}
			out("[%s] pid=%d creation_time=%s image_path=%q command=%s\n", label, p.PID, p.CreationTime, p.ImagePath, p.Command)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.saveDetachedLocked(); err != nil {
		out("[退出清单落盘失败] %v\n", err)
	}
}

func (r *processRecords) harvest(job uintptr) {
	// Stop observations before collecting and marking the actual Job harvest.
	close(r.stop)
	<-r.done
	if err := r.refresh(); err != nil {
		out("[退出进程清单采集失败] %v\n", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ok, _, err := procTerminateJobObject.Call(job, 1)
	if ok == 0 {
		out("[退出收割失败] %v\n", win32CallError("TerminateJobObject", err))
	} else {
		for _, p := range r.records {
			if !p.Detached && p.handle != 0 && p.SessionManaged {
				if _, err := syscall.WaitForSingleObject(p.handle, infiniteWait); err != nil {
					out("[退出收割核验失败] pid=%d %v\n", p.PID, err)
					continue
				}
				p.Status = "terminated"
				emitRuntimeEvent("process", processEvent{Action: "terminated", Process: *p})
				out("[退出收割进程] pid=%d creation_time=%s image_path=%q command=%s\n", p.PID, p.CreationTime, p.ImagePath, p.Command)
			}
		}
	}
	if err := r.saveDetachedLocked(); err != nil {
		out("[退出清单落盘失败] %v\n", err)
	}
	if r.dir != "" {
		records := make([]processRecord, 0, len(r.records))
		for _, p := range r.records {
			records = append(records, *p)
		}
		b, err := json.MarshalIndent(records, "", "  ")
		if err == nil {
			err = os.WriteFile(filepath.Join(r.dir, "process-exit.json"), append(b, '\n'), 0644)
		}
		if err != nil {
			out("[退出清单落盘失败] %v\n", err)
		}
	}
	for _, p := range r.records {
		if p.handle != 0 {
			syscall.CloseHandle(p.handle)
			p.handle = 0
		}
	}
}
