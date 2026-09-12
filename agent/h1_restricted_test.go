package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestH1RestrictedStateRemainsVisible(t *testing.T) {
	protocol, human := captureEvents(t)
	runner, _ := testJobRunner(t, time.Second)
	runner.assignProcess = func(uintptr, uintptr) error { return errors.New("nested Job denied") }
	if _, _, err := runner.Run("echo forbidden"); err == nil {
		t.Fatal("assignment failure accepted")
	}
	manager := newBackgroundTaskManager(runner, t.TempDir(), t.TempDir())
	for i := 0; i < 2; i++ {
		if s := manager.list(); !strings.Contains(s, "restricted=true") || !strings.Contains(s, "不保证退出时的确定性清理") {
			t.Fatalf("missing sustained restricted state: %s", s)
		}
	}
	if !strings.Contains(protocol.String(), `"type":"process_mode"`) || !strings.Contains(human.String(), "不保证退出时的确定性清理") {
		t.Fatalf("missing mode event or notice: %s / %s", protocol.String(), human.String())
	}
}
