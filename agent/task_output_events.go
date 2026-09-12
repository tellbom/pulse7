package main

import (
	"fmt"
	"os"
	"time"
)

// H5/H7: the backend watches bytes; consumers receive notifications without
// calling task_output. They can then fetch exactly the advertised byte range.
func (m *backgroundTaskManager) observeNative(task *backgroundTask) {
	done := make(chan processWaitResult, 1)
	go func() { code, err := task.proc.Wait(); done <- processWaitResult{exitCode: code, err: err} }()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	offset := int64(0)
	truncated := false
	observing := true
	publish := func() {
		if !observing {
			return
		}
		next, marked, err := m.publishOutput(task, offset, truncated)
		if err != nil {
			observing = false
			emitRuntimeEvent("background_task", backgroundTaskEvent{Action: "output_error", TaskID: task.ID, Status: "output_observation_stopped", Error: err.Error()})
			out("[后台输出观察停止] task=%s %v；任务未被终止\n", task.ID, err)
			return
		}
		offset = next
		truncated = marked
	}
	publish()
	for {
		select {
		case result := <-done:
			publish()
			m.finish(task, result)
			return
		case <-ticker.C:
			publish()
		}
	}
}

func (m *backgroundTaskManager) publishOutput(task *backgroundTask, offset int64, marked bool) (int64, bool, error) {
	file, err := os.Open(task.OutputPath)
	if err != nil {
		return offset, marked, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return offset, marked, err
	}
	if info.Size() < offset {
		return offset, marked, fmt.Errorf("output file shrank from %d to %d", offset, info.Size())
	}
	if info.Size() > offset {
		emitRuntimeEvent("background_task", backgroundTaskEvent{Action: "output", TaskID: task.ID, Offset: int(offset), NextOffset: int(info.Size())})
		if !marked && info.Size() >= int64(len(taskOutputTruncatedMarker)) {
			tail := make([]byte, len(taskOutputTruncatedMarker))
			if _, err := file.ReadAt(tail, info.Size()-int64(len(tail))); err != nil {
				return offset, marked, err
			}
			if string(tail) == taskOutputTruncatedMarker {
				marked = true
				emitRuntimeEvent("background_task", backgroundTaskEvent{Action: "output_limit", TaskID: task.ID, Status: "output_truncated", NextOffset: int(info.Size())})
				out("[后台输出截断] task=%s 已达到输出硬上限，任务继续运行\n", task.ID)
			}
		}
	}
	return info.Size(), marked, nil
}
