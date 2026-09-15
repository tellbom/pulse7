package main

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// Record facts for offline duplicate accounting; never prohibit another read.
// Content hashes cover only the returned source page, not an entire live file.
func (r *Registry) observeRead(kind, source, version string, start, next int64, content string) error {
	if r.auditPath == "" {
		return nil
	}
	return appendJSONLine(r.auditPath, map[string]interface{}{
		"event": "read_observation", "ts": time.Now().Format(time.RFC3339Nano), "task": r.taskID,
		"kind": kind, "source": source, "version": version,
		"byte_offset": start, "next_byte_offset": next, "source_bytes": next - start,
		"returned_bytes": len(content), "page_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
	})
}
