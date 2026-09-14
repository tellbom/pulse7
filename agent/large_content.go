package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

const largeContentThreshold = 64 << 10
const largeContentPreviewBytes = 32 << 10
const largeContentMaxReadBytes = 256 << 10

var largeContentRefPattern = regexp.MustCompile(`^lc1\.([a-f0-9]{24})\.([a-f0-9-]{36})\.(content|reasoning|toolcall-[0-9]+)\.([a-f0-9]{64})$`)

var largeVerified = struct {
	sync.Mutex
	entries map[string]os.FileInfo
}{entries: make(map[string]os.FileInfo)}

func largeSafePath(sessionPath, path string) error {
	root, _, _, err := resolveExistingPrefix(filepath.Dir(sessionPath))
	if err != nil {
		return err
	}
	resolved, _, _, err := resolveExistingPrefix(path)
	if err != nil {
		return err
	}
	return requirePathWithin(root, resolved)
}

func unchangedFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && os.SameFile(a, b)
}

func largeSessionKey(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	return hex.EncodeToString(sum[:12]), nil
}

func largeContentPath(sessionPath, uuid, field string) string {
	return filepath.Join(sessionPath+".content", uuid+"-"+field+".txt")
}

func largeContentPreview(s string) string {
	if len(s) <= largeContentPreviewBytes {
		return s
	}
	n := largeContentPreviewBytes
	for n > 0 && !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n]
}

func writeLargeContent(sessionPath, uuid, field, value string) (string, error) {
	key, err := largeSessionKey(sessionPath)
	if err != nil {
		return "", err
	}
	dir := sessionPath + ".content"
	if err := largeSafePath(sessionPath, dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(value))
	ref := fmt.Sprintf("lc1.%s.%s.%s.%x", key, uuid, field, sum)
	path := largeContentPath(sessionPath, uuid, field)
	if err := largeSafePath(sessionPath, path); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".large-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := (&Policy{Workspace: filepath.Dir(sessionPath)}).ValidateOpenFile(tmp, tmpPath); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := io.WriteString(tmp, value); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return ref, nil
}

func externalizeLargeMessage(sessionPath string, record *sessionMessageRecord) error {
	m := record.ChatCompletionMessage
	// ToolCalls is a slice: copy before editing so the live response still executes
	// with its original arguments after this record has been committed.
	m.ToolCalls = append([]openai.ToolCall(nil), m.ToolCalls...)
	fields := map[string]string{}
	add := func(field, value string, set func(string)) error {
		ref, err := writeLargeContent(sessionPath, record.UUID, field, value)
		if err != nil {
			return err
		}
		fields[field] = ref
		switch field {
		case "content":
			set(largeContentPreview(value) + "\n[Full content: " + ref + "; use content_ref read/grep]")
		case "reasoning":
			set(largeContentPreview(value) + "\n[Full reasoning: " + ref + "; use content_ref read/grep]")
		default:
			marker, _ := json.Marshal(map[string]string{"_large_content_ref": ref, "_note": "Original tool arguments are saved in the session; read this ref. This is a history projection, not executable arguments."})
			set(string(marker))
		}
		return nil
	}
	if len(m.Content) >= largeContentThreshold {
		if err := add("content", m.Content, func(v string) { m.Content = v }); err != nil {
			return err
		}
	}
	if len(m.ReasoningContent) >= largeContentThreshold {
		if err := add("reasoning", m.ReasoningContent, func(v string) { m.ReasoningContent = v }); err != nil {
			return err
		}
	}
	for i := range m.ToolCalls {
		if len(m.ToolCalls[i].Function.Arguments) < largeContentThreshold {
			continue
		}
		field := fmt.Sprintf("toolcall-%d", i)
		j := i
		if err := add(field, m.ToolCalls[i].Function.Arguments, func(v string) { m.ToolCalls[j].Function.Arguments = v }); err != nil {
			return err
		}
	}
	record.ChatCompletionMessage = m
	record.LargeContent = fields
	// A very large array of individually small fields can still cross the JSONL
	// boundary. Spill the largest remaining text field until the record fits.
	for {
		if _, err := encodeJSONLine(*record); err == nil {
			return nil
		} else if !strings.Contains(err.Error(), "JSONL record exceeds") {
			return err
		}
		field, value := "", ""
		choose := func(k, v string) {
			if len(v) > len(value) {
				field, value = k, v
			}
		}
		if _, ok := fields["content"]; !ok {
			choose("content", m.Content)
		}
		if _, ok := fields["reasoning"]; !ok {
			choose("reasoning", m.ReasoningContent)
		}
		for i := range m.ToolCalls {
			k := fmt.Sprintf("toolcall-%d", i)
			if _, ok := fields[k]; !ok {
				choose(k, m.ToolCalls[i].Function.Arguments)
			}
		}
		if value == "" {
			return errors.New("session record exceeds JSONL limit after all text fields were externalized")
		}
		switch field {
		case "content":
			if err := add(field, value, func(v string) { m.Content = v }); err != nil {
				return err
			}
		case "reasoning":
			if err := add(field, value, func(v string) { m.ReasoningContent = v }); err != nil {
				return err
			}
		default:
			i, _ := strconv.Atoi(strings.TrimPrefix(field, "toolcall-"))
			if err := add(field, value, func(v string) { m.ToolCalls[i].Function.Arguments = v }); err != nil {
				return err
			}
		}
		record.ChatCompletionMessage = m
		record.LargeContent = fields
	}
}

func parseLargeContentRef(sessionPath, ref string) (string, string, string, error) {
	parts := largeContentRefPattern.FindStringSubmatch(ref)
	if parts == nil {
		return "", "", "", errors.New("invalid large content reference")
	}
	key, err := largeSessionKey(sessionPath)
	if err != nil {
		return "", "", "", err
	}
	if key != parts[1] {
		return "", "", "", errors.New("large content reference belongs to another session")
	}
	return largeContentPath(sessionPath, parts[2], parts[3]), parts[3], parts[4], nil
}

func verifyLargeContent(sessionPath, ref string) (string, int64, error) {
	path, _, want, err := parseLargeContentRef(sessionPath, ref)
	if err != nil {
		return "", 0, err
	}
	if err := largeSafePath(sessionPath, path); err != nil {
		return "", 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("large content reference unavailable: %w", err)
	}
	defer f.Close()
	if err := (&Policy{Workspace: filepath.Dir(sessionPath)}).ValidateOpenRead(f, path); err != nil {
		return "", 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	largeVerified.Lock()
	previous := largeVerified.entries[ref]
	largeVerified.Unlock()
	if unchangedFile(previous, info) {
		return path, info.Size(), nil
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return "", 0, errors.New("large content reference checksum mismatch")
	}
	after, err := f.Stat()
	if err != nil || !unchangedFile(info, after) {
		return "", 0, errors.New("large content changed while verifying")
	}
	largeVerified.Lock()
	if len(largeVerified.entries) >= 128 {
		largeVerified.entries = make(map[string]os.FileInfo)
	}
	largeVerified.entries[ref] = after
	largeVerified.Unlock()
	return path, n, nil
}

// readLargeContent reads raw UTF-8 field bytes. Offsets are byte offsets;
// next is the next safe byte boundary, and a request is capped at 256 KiB.
func readLargeContent(sessionPath, ref string, offset, limit int64) (text string, total, next int64, err error) {
	path, total, err := verifyLargeContent(sessionPath, ref)
	if err != nil {
		return "", 0, 0, err
	}
	if offset < 0 || offset > total {
		return "", total, 0, errors.New("large content offset out of range")
	}
	if limit <= 0 {
		limit = largeContentPreviewBytes
	}
	if limit > largeContentMaxReadBytes {
		limit = largeContentMaxReadBytes
	}
	f, err := os.Open(path)
	if err != nil {
		return "", total, 0, err
	}
	defer f.Close()
	// Reject a mid-rune offset instead of silently repeating bytes.
	if err := (&Policy{Workspace: filepath.Dir(sessionPath)}).ValidateOpenRead(f, path); err != nil {
		return "", 0, 0, err
	}
	start := offset
	for start > 0 && start < total {
		var b [1]byte
		if _, err = f.ReadAt(b[:], start); err != nil {
			return "", total, 0, err
		}
		if b[0]&0xc0 != 0x80 {
			break
		}
		return "", total, 0, errors.New("offset is not a UTF-8 boundary; use next_byte_offset")
	}
	remaining := total - start
	if remaining < limit {
		limit = remaining
	}
	buf := make([]byte, limit)
	if _, err = io.ReadFull(io.NewSectionReader(f, start, limit), buf); err != nil {
		return "", total, 0, err
	}
	end := len(buf)
	if start+int64(end) < total {
		for end > 0 && !utf8.Valid(buf[:end]) {
			end--
		}
	}
	if end == 0 && start < total {
		return "", total, 0, errors.New("read limit splits UTF-8 rune")
	}
	return string(buf[:end]), total, start + int64(end), nil
}

func validateLargeMessageReferences(sessionPath string, record sessionMessageRecord) error {
	for field, ref := range record.LargeContent {
		path, parsed, _, err := parseLargeContentRef(sessionPath, ref)
		if err != nil {
			return err
		}
		if parsed != field || path != largeContentPath(sessionPath, record.UUID, field) {
			return errors.New("large content field mismatch")
		}
		if _, _, err := verifyLargeContent(sessionPath, ref); err != nil {
			return fmt.Errorf("large content reference unavailable: %w", err)
		}
	}
	return nil
}

func (r *Registry) contentRefPath(ref string) (string, error) {
	if sess == nil || !sameResolvedPath(sess.workspace, r.workspace) {
		return "", errors.New("no active session for content reference")
	}
	path, _, err := verifyLargeContent(sess.path, ref)
	if err != nil {
		return "", err
	}
	return r.absPath(path)
}

func (r *Registry) readContentRef(ref string, offset *int64, limit int) (string, error) {
	if _, err := r.contentRefPath(ref); err != nil {
		return "", err
	}
	start := int64(0)
	if offset != nil {
		start = *offset
	}
	text, total, next, err := readLargeContent(sess.path, ref, start, int64(limit))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[content_ref=%s byte_offset=%d next_byte_offset=%d total_bytes=%d complete=%v; UTF-8 bytes]\n%s", ref, start, next, total, next == total, text), nil
}

func hydrateLargeMessage(sessionPath string, record *sessionMessageRecord) error {
	for field, ref := range record.LargeContent {
		path, _, err := verifyLargeContent(sessionPath, ref)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch field {
		case "content":
			record.Content = string(b)
		case "reasoning":
			record.ReasoningContent = string(b)
		default:
			i, err := strconv.Atoi(strings.TrimPrefix(field, "toolcall-"))
			if err != nil || i < 0 || i >= len(record.ToolCalls) {
				return errors.New("invalid toolcall content reference")
			}
			record.ToolCalls[i].Function.Arguments = string(b)
		}
	}
	return nil
}

func loadSessionFullRecords(path string) ([]sessionMessageRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	var msgs []sessionMessageRecord
	for sc.Scan() {
		var envelope struct {
			Role   string `json:"role"`
			System string `json:"system"`
		}
		if err := json.Unmarshal(sc.Bytes(), &envelope); err != nil {
			return nil, err
		}
		if envelope.Role == "_meta" {
			continue
		}
		if envelope.Role == "_clear" {
			msgs = []sessionMessageRecord{{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: envelope.System}}}
			continue
		}
		var record sessionMessageRecord
		if err := json.Unmarshal(sc.Bytes(), &record); err != nil {
			return nil, err
		}
		if err := hydrateLargeMessage(path, &record); err != nil {
			return nil, err
		}
		msgs = append(msgs, record)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return msgs, nil
}
