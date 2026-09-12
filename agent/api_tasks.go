package main

import (
	"net/http"
	"os"
	"time"
)

func (a *apiServer) tasks(w http.ResponseWriter, r *http.Request) {
	list := []interface{}{}
	processes := []processRecord{}
	history := []interface{}{}
	overview := map[string]interface{}{"count": nil, "threshold": a.cfg.processWarnThreshold, "exceeded": nil}
	if a.reg != nil {
		m := a.reg.tasks
		m.mu.Lock()
		for _, t := range m.tasks {
			var size int64
			info, err := os.Stat(t.OutputPath)
			if err == nil {
				size = info.Size()
			}
			var exit interface{}
			if t.Status != "running" {
				exit = t.ExitCode
			}
			list = append(list, map[string]interface{}{"taskId": t.ID, "command": t.Command, "pid": t.PID, "detached": t.Detached, "startedAt": t.StartedAt, "runtimeMs": time.Since(t.StartedAt).Milliseconds(), "outputBytes": size, "status": t.Status, "exitCode": exit, "outputTruncated": outputWasTruncated(t.OutputPath)})
		}
		m.mu.Unlock()
		if count, err := m.runner.ProcessCount(); err == nil {
			overview["count"] = count
			overview["exceeded"] = count > a.cfg.processWarnThreshold
		} else {
			overview["error"] = err.Error()
		}
		if j, ok := m.runner.(*jobObjectRunner); ok {
			overview["mode"] = j.modeState("state")
			if j.processes != nil {
				j.processes.mu.Lock()
				for _, p := range j.processes.records {
					processes = append(processes, *p)
				}
				for _, p := range j.processes.previous {
					history = append(history, map[string]interface{}{"process": p, "verified": false})
				}
				j.processes.mu.Unlock()
			}
		}
	}
	apiJSON(w, 200, map[string]interface{}{"tasks": list, "processes": processes, "detachedHistory": history, "overview": overview})
}
func (a *apiServer) taskOutput(w http.ResponseWriter, r *http.Request, id string) {
	offset, limit, err := apiPagination(r, 4096, 1<<20)
	if err != nil {
		apiBad(w, err)
		return
	}
	if a.reg == nil {
		apiError(w, 404, "not_found", "task not found")
		return
	}
	m := a.reg.tasks
	t, err := m.get(id)
	if err != nil {
		apiError(w, 404, "not_found", "task not found")
		return
	}
	b, err := os.ReadFile(t.OutputPath)
	if err != nil {
		apiError(w, 500, "output_error", "could not read task output")
		return
	}
	if offset > len(b) {
		apiError(w, 400, "invalid_offset", "offset outside output")
		return
	}
	end := offset + limit
	if end > len(b) {
		end = len(b)
	}
	m.mu.Lock()
	status, exit := t.Status, t.ExitCode
	m.mu.Unlock()
	var code interface{}
	if status != "running" {
		code = exit
	}
	apiJSON(w, 200, map[string]interface{}{"taskId": id, "status": status, "exitCode": code, "offset": offset, "nextOffset": end, "totalBytes": len(b), "content": decodeShellOutput(b[offset:end])})
}
func (a *apiServer) checkpoints(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	list := []interface{}{}
	if a.reg != nil {
		g, err := a.reg.ensureGit()
		if err != nil {
			apiError(w, 500, "checkpoint_error", a.safeError(err))
			return
		}
		refs, err := g.taskRefs()
		if err != nil {
			apiError(w, 500, "checkpoint_error", a.safeError(err))
			return
		}
		for _, ref := range refs {
			meta, err := g.metadataForRef(ref)
			if err != nil {
				apiError(w, 500, "checkpoint_error", a.safeError(err))
				return
			}
			list = append(list, map[string]interface{}{"taskId": meta.TaskID, "seq": meta.Seq, "createdAt": meta.CreatedAt, "commit": meta.Commit, "tree": meta.Tree, "ref": meta.Ref, "kind": meta.Kind, "dirtyFiles": meta.DirtyFiles, "dirtyFilesBasis": meta.DirtyFilesBasis})
		}
	}
	apiJSON(w, 200, map[string]interface{}{"checkpoints": list})
}
