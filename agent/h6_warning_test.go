package main

import (
	"strings"
	"testing"
	"time"
)

func TestH6ProcessWarningIsVisibleWithoutTermination(t *testing.T) {
	protocol, _ := captureEvents(t)
	j, _ := testJobRunner(t, time.Second)
	hDirectSessionPing(t, j)
	hDirectSessionPing(t, j)
	m := newBackgroundTaskManager(j, t.TempDir(), t.TempDir())
	m.processWarnThreshold = 1
	m.warnProcessCount()
	count, err := j.ProcessCount()
	if err != nil || count < 2 {
		t.Fatalf("warning terminated processes count=%d err=%v", count, err)
	}
	if !strings.Contains(protocol.String(), `"kind":"process_count"`) {
		t.Fatalf("missing warning event: %s", protocol.String())
	}
}
