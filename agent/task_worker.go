package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

const taskOutputTruncatedMarker = "\n[pulse7: background task output truncated at configured hard limit]\n"

type cappedTaskWriter struct {
	mu        sync.Mutex
	w         io.Writer
	remaining int64
	marked    bool
}

func (w *cappedTaskWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	original := len(p)
	marker := []byte(taskOutputTruncatedMarker)
	contentLimit := w.remaining
	if !w.marked {
		contentLimit -= int64(len(marker))
	}
	if contentLimit < 0 {
		contentLimit = 0
	}
	if int64(len(p)) <= contentLimit {
		if _, err := w.w.Write(p); err != nil {
			return 0, err
		}
		w.remaining -= int64(len(p))
		return original, nil
	}
	if contentLimit > 0 {
		if _, err := w.w.Write(p[:contentLimit]); err != nil {
			return 0, err
		}
		w.remaining -= contentLimit
	}
	if !w.marked && w.remaining >= int64(len(marker)) {
		if _, err := w.w.Write(marker); err != nil {
			return 0, err
		}
		w.remaining -= int64(len(marker))
		w.marked = true
	}
	return original, nil
}

func runTaskWorker(args []string) int {
	fs := flag.NewFlagSet("task-worker", flag.ContinueOnError)
	commandFile := fs.String("command-file", "", "")
	outputFile := fs.String("output-file", "", "")
	statusFile := fs.String("status-file", "", "")
	pidFile := fs.String("pid-file", "", "")
	workspace := fs.String("workspace", "", "")
	maxBytes := fs.Int64("max-bytes", 0, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *commandFile == "" || *outputFile == "" || *statusFile == "" || *pidFile == "" || *workspace == "" || *maxBytes <= 0 {
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(*outputFile), 0o755); err != nil {
		return 1
	}
	output, err := os.OpenFile(*outputFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 1
	}
	writer := &cappedTaskWriter{w: output, remaining: *maxBytes}
	if err := os.WriteFile(*pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		fmt.Fprintf(writer, "[pulse7 task worker error] persist pid: %v\n", err)
		output.Close()
		return 1
	}
	commandProcessor := os.Getenv("COMSPEC")
	if commandProcessor == "" {
		commandProcessor = "cmd.exe"
	}
	cmd := exec.Command(commandProcessor, "/d", "/s", "/c", *commandFile)
	cmd.Dir = *workspace
	cmd.Stdout = writer
	cmd.Stderr = writer
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	storageError := ""
	if syncErr := output.Sync(); syncErr != nil {
		storageError = "sync output: " + syncErr.Error()
		exitCode = 1
	}
	if closeErr := output.Close(); closeErr != nil {
		if storageError != "" {
			storageError += "; "
		}
		storageError += "close output: " + closeErr.Error()
		exitCode = 1
	}
	status, _ := json.Marshal(map[string]interface{}{
		"exit_code": exitCode, "truncated": writer.marked, "storage_error": storageError,
	})
	if writeErr := os.WriteFile(*statusFile, append(status, '\n'), 0o644); writeErr != nil {
		fmt.Fprintln(os.Stderr, writeErr)
		return 1
	}
	return exitCode
}
