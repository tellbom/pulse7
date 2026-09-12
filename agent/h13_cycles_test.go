package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestH13TwentyNativeStartStopCycles(t *testing.T) {
	product := os.Getenv("H_PRODUCT")
	if product == "" {
		t.Fatal("H_PRODUCT required")
	}
	j, dir := testJobRunner(t, time.Second)
	for i := 0; i < 20; i++ {
		if out, code, err := j.Run(fmt.Sprintf("echo cycle-%d", i)); err != nil || code != 0 {
			t.Fatalf("cycle %d foreground %s %d %v", i, out, code, err)
		}
		prefix := filepath.Join(dir, fmt.Sprintf("bg-%d", i))
		command := prefix + ".cmd"
		if err := os.WriteFile(command, []byte("@ping -n 30 127.0.0.1 >nul\r\n"), 0600); err != nil {
			t.Fatal(err)
		}
		spec := processStartSpec{Exe: product, Workspace: dir, CommandFile: command, OutputFile: prefix + ".output", StatusFile: prefix + ".status", PIDFile: prefix + ".pid", MaxBytes: 1024, Command: "cycle ping", TaskID: fmt.Sprintf("cycle-%d", i)}
		p, err := j.StartManaged(spec, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := waitTaskPID(spec.PIDFile, 10*time.Second); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
		if err := p.KillTree(); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Wait(); err != nil {
			t.Fatal(err)
		}
		table := hProcessTable(t)
		if _, present := table[uint32(p.PID())]; present {
			t.Fatalf("cycle %d worker remains", i)
		}
		count, err := j.ProcessCount()
		if err != nil || count != 0 {
			t.Fatalf("cycle %d residual count=%d err=%v", i, count, err)
		}
		t.Logf("cycle=%d foreground_exit=0 background_pid=%d removed=true job_processes=%d", i+1, p.PID(), count)
	}
}
