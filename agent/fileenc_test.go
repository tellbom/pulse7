package main

// fileenc_test.go: byte-level verification for the file-encoding round
// (A2/A3/A4 of the task spec). These tests assert on RAW BYTES — no
// eyeball judgment.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gbkBytes builds a GBK-encoded file body. Windows test machines here run
// codepage 936, so utf8ToCodepage(s, cpGBK) produces real GBK bytes.
func gbkBytes(t *testing.T, s string) []byte {
	t.Helper()
	if !codepageRoundTrips(s, cpGBK) {
		t.Fatalf("test fixture not GBK-representable: %q", s)
	}
	return utf8ToCodepage(s, cpGBK)
}

// newTestRegistry: a Registry whose workspace is a fresh temp dir.
func newTestRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	d := t.TempDir()
	return &Registry{policy: &Policy{Workspace: d}, man: &manifest{path: filepath.Join(d, "man.jsonl")}}, d
}

func put(t *testing.T, dir, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A3+A4: edit a GBK+CRLF file — every line except the edited one must be
// byte-identical afterwards, and the encoding/CRLF style must survive.
func TestEditGBKPreservesUnrelatedBytes(t *testing.T) {
	body := "alpha\r\n" + "中文注释第一行\r\n" + "beta\r\n" + "中文注释第二行\r\n" + "gamma\r\n"
	src := gbkBytes(t, body)
	r, d := newTestRegistry(t)
	p := put(t, d, "g.cs", src)

	if _, err := r.toolEdit(mustJSON(t, map[string]string{
		"path": p, "old_string": "中文注释第一行", "new_string": "中文注释已修改"})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	after, _ := os.ReadFile(p)

	afterText := bytesToUTF8(after, cpGBK)
	for _, want := range []string{"alpha", "beta", "gamma", "中文注释第二行", "中文注释已修改"} {
		if !strings.Contains(afterText, want) {
			t.Fatalf("after-text missing %q:\n%s", want, afterText)
		}
	}
	if strings.Contains(afterText, "中文注释第一行") {
		t.Fatal("old text still present")
	}
	// BYTE-LEVEL: unchanged lines identical, file still GBK+CRLF (A4).
	beforeLines := bytes.Split(src, []byte("\r\n"))
	afterLines := bytes.Split(after, []byte("\r\n"))
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("line count changed: %d -> %d", len(beforeLines), len(afterLines))
	}
	for i := range beforeLines {
		if i == 1 { // the edited line
			continue
		}
		if !bytes.Equal(beforeLines[i], afterLines[i]) {
			t.Fatalf("line %d bytes changed: %X -> %X", i, beforeLines[i], afterLines[i])
		}
	}
	if !bytes.Contains(after, []byte("\r\n")) {
		t.Fatal("CRLF style lost")
	}
	if bytes.Equal(after, []byte(afterText)) {
		t.Fatal("file was rewritten as UTF-8 instead of staying GBK")
	}
}

// A3: edit with a non-existent old_string — file bytes must be UNTOUCHED.
func TestEditNoMatchLeavesBytesUntouched(t *testing.T) {
	src := gbkBytes(t, "第一行\r\n第二行\r\n")
	r, d := newTestRegistry(t)
	p := put(t, d, "g2.cs", src)
	_, err := r.toolEdit(mustJSON(t, map[string]string{
		"path": p, "old_string": "根本不存在的字符串", "new_string": "x"}))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(src, after) {
		t.Fatalf("bytes changed on no-match:\n%X\n%X", src, after)
	}
}

// A3: UTF-16 file must be refused, not mangled.
func TestEditUTF16Refused(t *testing.T) {
	src := []byte{0xFF, 0xFE, 'a', 0, 'b', 0}
	r, d := newTestRegistry(t)
	p := put(t, d, "u16.txt", src)
	_, err := r.toolEdit(mustJSON(t, map[string]string{
		"path": p, "old_string": "a", "new_string": "z"}))
	if err == nil || !strings.Contains(err.Error(), "UTF-16") {
		t.Fatalf("expected UTF-16 refusal, got %v", err)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(src, after) {
		t.Fatal("UTF-16 file was modified despite refusal")
	}
}

// A2: decodeFileBytes on GBK gives clean UTF-8, zero U+FFFD.
func TestDecodeFileBytesGBK(t *testing.T) {
	text, ok, _ := decodeFileBytes(gbkBytes(t, "中文内容 abc"))
	if !ok || text != "中文内容 abc" || strings.ContainsRune(text, 0xFFFD) {
		t.Fatalf("decode failed: ok=%v text=%q", ok, text)
	}
}

// read on a GBK file must surface readable Chinese.
func TestReadToolGBKZeroFFFD(t *testing.T) {
	r, d := newTestRegistry(t)
	p := put(t, d, "g3.cs", gbkBytes(t, "行一\r\n行二\r\n"))
	out, err := r.toolRead(mustJSON(t, map[string]interface{}{"path": p}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out, 0xFFFD) || !strings.Contains(out, "行一") {
		t.Fatalf("read on GBK not clean: %q", out)
	}
}

// read on binary is refused with a note.
func TestReadBinaryRefused(t *testing.T) {
	r, d := newTestRegistry(t)
	p := put(t, d, "bin.dat", []byte{0x00, 0x01, 0x02, 0x00})
	out, err := r.toolRead(mustJSON(t, map[string]interface{}{"path": p}))
	if err != nil || !strings.Contains(out, "binary") {
		t.Fatalf("binary not flagged: %q %v", out, err)
	}
}

// write: new file UTF-8; overwrite GBK stays GBK.
func TestWriteKeepsEncoding(t *testing.T) {
	r, d := newTestRegistry(t)
	p := filepath.Join(d, "w.cs")
	if _, err := r.toolWrite(mustJSON(t, map[string]string{"path": p, "content": "新建内容"})); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, []byte("新建内容")) {
		t.Fatalf("new file not UTF-8: %X", b)
	}
	put(t, d, "w.cs", gbkBytes(t, "旧内容一行\r\n"))
	if _, err := r.toolWrite(mustJSON(t, map[string]string{"path": p, "content": "新内容两行"})); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, gbkBytes(t, "新内容两行")) {
		t.Fatalf("overwrite did not keep GBK: %X", b)
	}
}

// write: UTF-8 BOM file keeps its BOM on overwrite.
func TestWriteKeepsBOM(t *testing.T) {
	r, d := newTestRegistry(t)
	p := put(t, d, "bom.txt", append([]byte{0xEF, 0xBB, 0xBF}, []byte("带BOM")...))
	if _, err := r.toolWrite(mustJSON(t, map[string]string{"path": p, "content": "覆盖后"})); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) || string(b[3:]) != "覆盖后" {
		t.Fatalf("BOM lost or content wrong: %X", b)
	}
}

// edit: UTF-8 BOM file keeps its BOM.
func TestEditKeepsBOM(t *testing.T) {
	r, d := newTestRegistry(t)
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("开头\r\n中间目标\r\n结尾")...)
	p := put(t, d, "bom2.txt", src)
	if _, err := r.toolEdit(mustJSON(t, map[string]string{
		"path": p, "old_string": "中间目标", "new_string": "已替换"})); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) || !bytes.HasSuffix(b, []byte("已替换\r\n结尾")) {
		t.Fatalf("BOM lost or body wrong: %X", b)
	}
}

// A1: consoleCodepage never returns nonsense (0 falls back to CP_ACP).
func TestConsoleCodepageFallback(t *testing.T) {
	if cp := consoleCodepage(); cp > 65535 {
		t.Fatalf("implausible codepage %d", cp)
	}
}
