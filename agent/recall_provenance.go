package main

import (
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Reuse the original snapshot for successful read pages, never an attachment
// containing a page header and another reference. Errors retain their own output.
func compactResultReference(record sessionMessageRecord, call openai.ToolCall) (string, map[string]int64, error) {
	var args struct {
		Ref    string `json:"content_ref"`
		Offset int64  `json:"byte_offset"`
	}
	if call.Function.Name == "read" && json.Unmarshal([]byte(call.Function.Arguments), &args) == nil && args.Ref != "" && (record.ToolOutcome == nil || record.ToolOutcome.OK) {
		var ref string
		var start, next, total int64
		var complete bool
		header := strings.SplitN(record.Content, "\n", 2)[0]
		n, err := fmt.Sscanf(header, "[content_ref=%s byte_offset=%d next_byte_offset=%d total_bytes=%d complete=%t; UTF-8 bytes]", &ref, &start, &next, &total, &complete)
		if err == nil && n == 5 && ref == args.Ref && start == args.Offset && start >= 0 && next >= start && next <= total && complete == (next == total) {
			_, info, err := verifyLargeContent(sess.path, ref)
			if err != nil {
				return "", nil, err
			}
			if info != total {
				return "", nil, fmt.Errorf("recall snapshot size mismatch")
			}
			return ref, map[string]int64{"byte_offset": start, "byte_limit": next - start, "next_byte_offset": next}, nil
		}
	}
	ref := record.LargeContent["content"]
	if ref == "" {
		ref, err := compactAttachment(record.Content)
		return ref, nil, err
	}
	_, _, err := verifyLargeContent(sess.path, ref)
	return ref, nil, err
}
