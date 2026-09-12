package main

import (
	"strings"
	"testing"
	"time"
)

func TestH3ForegroundRegistersNativeIdentity(t *testing.T) {
	protocol, _ := captureEvents(t)
	j, _ := testJobRunner(t, time.Second)
	if _, code, err := j.Run("echo native-identity"); err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	m := newBackgroundTaskManager(j, t.TempDir(), t.TempDir())
	list := m.list()
	for _, field := range []string{"creation_time=", "image_path=", "source=foreground", "native-identity"} {
		if !strings.Contains(list, field) {
			t.Fatalf("missing %s: %s", field, list)
		}
	}
	if !strings.Contains(protocol.String(), `"type":"process"`) {
		t.Fatalf("missing process events: %s", protocol.String())
	}
}
