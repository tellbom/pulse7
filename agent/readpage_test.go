package main

import (
	"fmt"
	"strings"
	"testing"
)

func mkLines(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "line-%04d\n", i)
	}
	return sb.String()
}

func TestReadPageDefaultMatchesHistoricWindow(t *testing.T) {
	// 100 lines x 10 bytes: auto window must stop inside the 4096-byte cap
	out := readPage(mkLines(100), 0, 0)
	if !strings.HasPrefix(out, "[第 1-") || !strings.Contains(out, "共 100 行]") {
		t.Fatalf("missing range annotation: %q", out[:60])
	}
	if !strings.Contains(out, "offset=4") == false && !strings.Contains(out, "还有") {
		t.Fatalf("missing continuation hint for a truncated file:\n%s", out[:200])
	}
	if !strings.Contains(out, "line-0001") {
		t.Fatal("content missing")
	}
}

func TestReadPageExplicitWindow(t *testing.T) {
	out := readPage(mkLines(200), 100, 50)
	if !strings.HasPrefix(out, "[第 100-149 行，共 200 行]\n") {
		t.Fatalf("bad range header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if !strings.Contains(out, "line-0100") || !strings.Contains(out, "line-0149") {
		t.Fatal("window content wrong")
	}
	if strings.Contains(out, "line-0099") || strings.Contains(out, "line-0150") {
		t.Fatal("window off by one")
	}
	if !strings.Contains(out, "还有 51 行未读；继续读取请传 offset=150") {
		t.Fatalf("bad continuation hint: %v", out[len(out)-80:])
	}
}

func TestReadPageExactTail(t *testing.T) {
	out := readPage(mkLines(200), 151, 50)
	if !strings.HasPrefix(out, "[第 151-200 行，共 200 行]\n") {
		t.Fatalf("bad tail header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if strings.Contains(out, "未读") {
		t.Fatal("tail must not claim unread lines")
	}
}

func TestReadPageOffsetBeyondEnd(t *testing.T) {
	out := readPage(mkLines(10), 999, 5)
	if !strings.Contains(out, "beyond the end") || strings.Contains(out, "line-") {
		t.Fatalf("out-of-range must be a plain note: %q", out)
	}
}

func TestReadPageLimitClamp(t *testing.T) {
	out := readPage(mkLines(5000), 1, 99999)
	if !strings.HasPrefix(out, "[第 1-2000 行，共 5000 行]\n") {
		t.Fatalf("limit must clamp to maxReadLines: %q", strings.SplitN(out, "\n", 2)[0])
	}
}

func TestReadPageEmptyFile(t *testing.T) {
	if out := readPage("", 1, 10); out != "[empty file]\n" {
		t.Fatalf("empty file: %q", out)
	}
}

func TestReadPageSmallFileWhole(t *testing.T) {
	out := readPage("a\nb\nc", 0, 0)
	if !strings.HasPrefix(out, "[第 1-3 行，共 3 行]\n") || strings.Contains(out, "未读") {
		t.Fatalf("small file should read whole: %q", out)
	}
}
