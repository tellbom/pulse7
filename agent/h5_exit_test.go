package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestH5ExitHelper(t *testing.T) {
	mode := os.Getenv("H_EXIT_MODE")
	if mode == "" {
		return
	}
	dir := os.Getenv("H_EXIT_DIR")
	j := &jobObjectRunner{Workspace: dir, Home: homeDir(), Timeout: time.Second, MemLimitMB: 256}
	if err := j.StartSession(); err != nil {
		t.Fatal(err)
	}
	curRunner = j
	curCfg = &config{cleanupOnExit: true}
	m := newBackgroundTaskManager(j, filepath.Join(dir, "tasks"), dir)
	curRegistry = &Registry{runner: j, tasks: m}
	if err := j.processes.loadDetached(m.dir); err != nil {
		t.Fatal(err)
	}
	if _, _, err := j.Run(`start "" /b ping -n 60 127.0.0.1 >nul`); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(dir, "background.cmd")
	if err := os.WriteFile(command, []byte("@ping -n 60 127.0.0.1 >nul\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := processStartSpec{Exe: os.Getenv("H_PRODUCT"), Workspace: dir, CommandFile: command, Command: "ping background", TaskID: "exit-background", OutputFile: filepath.Join(dir, "task.output"), PIDFile: filepath.Join(dir, "task.pid"), StatusFile: filepath.Join(dir, "task.status"), MaxBytes: 1024}
	p, err := j.StartManaged(spec, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := waitTaskPID(spec.PIDFile, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	task := &backgroundTask{ID: spec.TaskID, Command: spec.Command, PID: p.PID(), Status: "running", OutputPath: spec.OutputFile, proc: p, StartedAt: time.Now(), done: make(chan struct{})}
	m.tasks[task.ID] = task
	go m.observe(task)
	time.Sleep(150 * time.Millisecond)
	records, err := j.processes.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var live []processRecord
	managed := 0
	for _, p := range records {
		if p.handle != 0 {
			live = append(live, p)
			if p.SessionManaged {
				managed++
			}
		}
	}
	if len(live) < 3 {
		t.Fatalf("insufficient live targets: %v", live)
	}
	if managed < 1 {
		t.Fatal("no live explicit session member to verify")
	}
	data, _ := json.Marshal(live)
	if err := os.WriteFile(filepath.Join(dir, "pids.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "normal":
		exitWith(0, "DONE", "H5 exit fixture")
	case "exec-error":
		exitWith(1, "EXEC-ERROR", "H5 injected execution failure")
	case "panic":
		os.Setenv("PULSE7_PANIC_TEST", "1")
		os.Args = []string{os.Args[0], "exec", "panic-fixture"}
		flag.CommandLine = flag.NewFlagSet("pulse7", flag.ContinueOnError)
		main()
	case "ctrl-c":
		watchInterrupt()
		k32.NewProc("SetConsoleCtrlHandler").Call(0, 0)
		for n := 0; n < 2; n++ {
			if ok, _, err := k32.NewProc("GenerateConsoleCtrlEvent").Call(0, 0); ok == 0 {
				t.Fatal(err)
			}
			time.Sleep(200 * time.Millisecond)
		}
		time.Sleep(time.Second)
		t.Fatal("second Ctrl-C did not exit")
	default:
		t.Fatal("unknown fixture mode")
	}
}

func TestH5FourExitPathsHarvestProcessTable(t *testing.T) {
	product := os.Getenv("H_PRODUCT")
	if product == "" {
		t.Fatal("H_PRODUCT must identify the Win7 product binary for exit integration")
	}
	for _, mode := range []string{"normal", "exec-error", "ctrl-c", "panic"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "exit.log")
			log, err := os.Create(logPath)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestH5ExitHelper$")
			cmd.Env = append(os.Environ(), "H_EXIT_MODE="+mode, "H_EXIT_DIR="+dir, "H_PRODUCT="+product)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createBreakawayFromJob | 0x10}
			cmd.Stdout = log
			cmd.Stderr = log
			err = cmd.Run()
			log.Close()
			code := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					code = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			expected := map[string]int{"normal": 0, "exec-error": 1, "ctrl-c": 130, "panic": 2}[mode]
			data, _ := os.ReadFile(logPath)
			t.Logf("mode=%s exit=%d log=\n%s", mode, code, data)
			if code != expected {
				t.Fatalf("want exit %d", expected)
			}
			if !strings.Contains(string(data), "FINISHED id=exit-background status=killed") {
				t.Fatal("harvested task end event missing or falsely reported exited")
			}
			b, err := os.ReadFile(filepath.Join(dir, "pids.json"))
			if err != nil {
				t.Fatal(err)
			}
			var pids []processRecord
			if err := json.Unmarshal(b, &pids); err != nil {
				t.Fatal(err)
			}
			after := hProcessTable(t)
			for _, p := range pids {
				pid := p.PID
				_, present := after[pid]
				t.Log(fmt.Sprintf("mode=%s after pid=%d session_managed=%v present=%v", mode, pid, p.SessionManaged, present))
				if present && p.SessionManaged {
					t.Errorf("residual session member PID %d", pid)
				}
				if !p.SessionManaged {
					hCleanupObserved(t, p)
				}
			}
		})
	}
}
