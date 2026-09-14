package main

import (
	"encoding/json"
	"fmt"
	"io"
)

type cliEventRenderer struct {
	narr linePrefixer
}

func newCLIEventRenderer() *cliEventRenderer {
	return &cliEventRenderer{narr: linePrefixer{prefix: "| "}}
}

func (r *cliEventRenderer) render(event runtimeEvent) {
	switch data := event.Data.(type) {
	case processEvent:
		p := data.Process
		out("[process] %s pid=%d creation_time=%s image_path=%q source=%s task_id=%s status=%s command=%s\n", data.Action, p.PID, p.CreationTime, p.ImagePath, p.Source, p.TaskID, p.Status, p.Command)
	case processModeEvent:
		if data.Restricted {
			out("[%s] %s：%s\n", data.Mode, data.Notice, data.Reason)
		} else if data.Notice != "" {
			out("[%s] %s\n", data.Mode, data.Notice)
		}
	case assistantDeltaEvent:
		r.narr.write(data.Delta)
	case toolCallEvent:
		printToolCall(data.Name, string(data.Args))
	case toolResultEvent:
		printToolResult(data.Name, data.Args, data.Result)
	case permissionRequestEvent:
		out("[permission] ASK %s %s\n[y/N]? ", data.Tool, data.Target)
	case permissionResponseEvent:
		if data.Decision == "allow" {
			if data.Requested {
				out("[permission] ALLOWED-BY-USER %s %s\n", data.Tool, data.Target)
			} else {
				out("[permission] AUTO-ALLOWED %s %s (%s)\n", data.Tool, data.Target, data.Source)
			}
		}
	case compactionEvent:
		if data.Method == compressionTruncate {
			out("[上下文已截断：移除 %d 条较早消息]\n", data.Removed)
		} else if data.Method == "micro" {
			out("[上下文微压缩：%d 条旧工具结果替换为原文引用]\n", data.Removed)
		} else {
			out("[上下文已压缩：%d 条消息 → 摘要]\n", data.Removed)
		}
	case backgroundTaskEvent:
		switch data.Action {
		case "start":
			out("[task] STARTED id=%s pid=%d detached=%v command=%s\n", data.TaskID, *data.PID, *data.Detached, data.Command)
		case "end":
			out("[task] FINISHED id=%s status=%s exitcode=%d\n", data.TaskID, data.Status, *data.ExitCode)
		}
	case outsideWorkspaceWriteEvent:
		out("[工作区外写入] %s -> %s；不在 checkpoint 覆盖范围内，无法通过 rollback 回退。\n", data.RequestedPath, data.ResolvedPath)
	case heartbeatEvent:
		out("[等待模型响应... %ds]\n", data.WaitedSeconds)
	}
}

func (r *cliEventRenderer) flushAssistant() {
	r.narr.flush()
}

func (r *cliEventRenderer) resetAssistant() {
	r.narr.buf.Reset()
}

// CLI setup is the composition point, not part of event production.
func newEventBus(streamJSON bool, jsonOut io.Writer) *eventBus {
	b := &eventBus{}
	if streamJSON {
		b.consumers = append(b.consumers, eventConsumer{emit: func(event runtimeEvent) {
			encoded, err := json.Marshal(event)
			if err != nil {
				panic(fmt.Sprintf("encode %s event: %v", event.Type, err))
			}
			if _, err := fmt.Fprintln(jsonOut, string(encoded)); err != nil {
				panic(fmt.Sprintf("write %s event: %v", event.Type, err))
			}
		}})
	}
	cli := newCLIEventRenderer()
	b.consumers = append(b.consumers, eventConsumer{
		emit: cli.render, flushAssistant: cli.flushAssistant, resetAssistant: cli.resetAssistant,
	})
	return b
}

var runtimeEvents = newEventBus(false, io.Discard)

func configureEventOutput(format string) error {
	switch format {
	case "", outputFormatText:
		runtimeEvents = newEventBus(false, io.Discard)
	case outputFormatStreamJSON:
		runtimeEvents = newEventBus(true, newConsoleWriter())
	default:
		return fmt.Errorf("invalid output format %q (want text or stream-json)", format)
	}
	return nil
}
