package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"
)

const defaultReadBytes = 32 << 10
const maxReadBytes = 256 << 10

func textEncodingName(enc fileEnc) string {
	if enc == encGBK {
		return "GBK"
	}
	return "UTF-8"
}

type textFileInspection struct {
	info     os.FileInfo
	lines    int
	encoding fileEnc
	reason   string
}

var textInspections = struct {
	sync.Mutex
	entries map[string]textFileInspection
}{entries: make(map[string]textFileInspection)}

func cachedTextInspection(f *os.File, info os.FileInfo) (int, fileEnc, string, error) {
	textInspections.Lock()
	prior, ok := textInspections.entries[f.Name()]
	textInspections.Unlock()
	if ok && unchangedFile(prior.info, info) {
		return prior.lines, prior.encoding, prior.reason, nil
	}
	lines, enc, reason, err := inspectTextFile(f)
	if err != nil {
		return 0, enc, reason, err
	}
	after, err := f.Stat()
	if err != nil || !unchangedFile(info, after) {
		return 0, enc, reason, fmt.Errorf("file changed during inspection")
	}
	textInspections.Lock()
	if len(textInspections.entries) >= 128 {
		textInspections.entries = make(map[string]textFileInspection)
	}
	textInspections.entries[f.Name()] = textFileInspection{after, lines, enc, reason}
	textInspections.Unlock()
	return lines, enc, reason, nil
}

func boundedReadSize(n int) int {
	if n <= 0 {
		return defaultReadBytes
	}
	if n > maxReadBytes {
		return maxReadBytes
	}
	return n
}

// inspectTextFile streams the file once. It never retains a line or file in
// memory, including a file whose first line is several megabytes long.
func inspectTextFile(f *os.File) (totalLines int, encoding fileEnc, reason string, err error) {
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return
	}
	buf := make([]byte, 64<<10)
	first := true
	validUTF8 := true
	carry := []byte{}
	var size int64
	var last byte
	for {
		n, e := f.Read(buf)
		if n > 0 {
			part := buf[:n]
			if first {
				first = false
				if bytes.HasPrefix(part, []byte{0xff, 0xfe}) || bytes.HasPrefix(part, []byte{0xfe, 0xff}) {
					reason = "UTF-16 encoded file: edit/read as text is not supported, refusing to guess (bytes untouched)"
					return
				}
			}
			if bytes.IndexByte(part, 0) >= 0 {
				reason = "binary file (NUL byte detected): not read as text"
				return
			}
			totalLines += bytes.Count(part, []byte{10})
			size += int64(n)
			last = part[n-1]
			if validUTF8 {
				joined := append(carry, part...)
				carry = nil
				for len(joined) > 0 {
					if !utf8.FullRune(joined) {
						carry = append(carry, joined...)
						break
					}
					_, width := utf8.DecodeRune(joined)
					if width == 1 && joined[0] >= 0x80 {
						validUTF8 = false
						break
					}
					joined = joined[width:]
				}
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			err = e
			return
		}
	}
	if size > 0 && last != 10 {
		totalLines++
	}
	if !validUTF8 || len(carry) > 0 {
		encoding = encGBK
	} else {
		encoding = encUTF8
	}
	return
}

func rawCharacterEnd(b []byte, encoding fileEnc) int {
	if encoding == encUTF8 {
		end := len(b)
		for trimmed := 0; trimmed < 4 && end > 0; trimmed++ {
			if utf8.Valid(b[:end]) {
				return end
			}
			end--
		}
		return 0
	}
	i := 0
	for i < len(b) {
		if b[i] < 0x80 {
			i++
			continue
		}
		if i+1 >= len(b) {
			break
		}
		i += 2
	}
	return i
}

func (r *Registry) readFilePage(path string, lineOffset, lineLimit int, byteOffset *int64, byteLimit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err = r.policy.ValidateOpenRead(f, path); err != nil {
		return "", err
	}
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	total, encoding, reason, err := cachedTextInspection(f, info)
	if err != nil {
		return "", err
	}
	r.rememberFileRead(path, info)
	if reason != "" {
		return fmt.Sprintf("[无法按文本读取 %s：%s]\n", path, reason), nil
	}
	if info.Size() == 0 {
		return "[empty file]\n", nil
	}
	start := int64(0)
	currentLine := 1
	if byteOffset != nil {
		start = *byteOffset
		if start < 0 || start > info.Size() {
			return "", fmt.Errorf("byte_offset %d outside file size %d", start, info.Size())
		}
	} else if lineOffset > 1 {
		if lineOffset > total {
			return fmt.Sprintf("[file has %d line(s); offset %d is beyond the end]\n", total, lineOffset), nil
		}
	}
	// Locate a line or report the line containing an explicit byte cursor.
	if lineOffset > 1 && byteOffset == nil {
		buf := make([]byte, 64<<10)
		var pos int64
		target := lineOffset
		if byteOffset != nil {
			target = 0
		}
		for pos < start || (target > 0 && currentLine < target) {
			n, e := f.ReadAt(buf, pos)
			if e != nil && e != io.EOF {
				return "", e
			}
			if n == 0 {
				break
			}
			for i := 0; i < n; i++ {
				if byteOffset != nil && pos+int64(i) >= start {
					break
				}
				if buf[i] == 10 {
					currentLine++
					if target > 0 && currentLine == target {
						start = pos + int64(i) + 1
						break
					}
				}
			}
			if target > 0 && currentLine == target {
				break
			}
			pos += int64(n)
		}
	}
	if start == info.Size() {
		return fmt.Sprintf("[file has %d line(s); byte_offset %d is at end]\n", total, start), nil
	}
	if byteOffset != nil && encoding == encUTF8 && start > 0 {
		var first [1]byte
		if _, err := f.ReadAt(first[:], start); err != nil {
			return "", err
		}
		if first[0]&0xc0 == 0x80 {
			return "", fmt.Errorf("byte_offset must be a character boundary; use next_byte_offset")
		}
	}
	size := boundedReadSize(byteLimit)
	if int64(size) > info.Size()-start {
		size = int(info.Size() - start)
	}
	b := make([]byte, size)
	n, err := f.ReadAt(b, start)
	if err != nil && err != io.EOF {
		return "", err
	}
	b = b[:n]
	end := rawCharacterEnd(b, encoding)
	if end == 0 && len(b) > 0 {
		return "", fmt.Errorf("byte_offset %d is not a %s character boundary", start, textEncodingName(encoding))
	}
	b = b[:end]
	if lineLimit > 0 {
		if lineLimit > maxReadLines {
			lineLimit = maxReadLines
		}
		count := 0
		for i, c := range b {
			if c == 10 {
				count++
				if count >= lineLimit {
					b = b[:i+1]
					break
				}
			}
		}
	}
	next := start + int64(len(b))
	shownLines := bytes.Count(b, []byte{10})
	lastLine := currentLine + shownLines
	if len(b) > 0 && b[len(b)-1] == 10 {
		lastLine--
	}
	if lastLine < currentLine {
		lastLine = currentLine
	}
	content := string(b)
	if encoding == encGBK {
		content = bytesToUTF8(b, cpGBK)
	}
	if start == 0 {
		content = string(bytes.TrimPrefix([]byte(content), utf8BOM))
	}
	after, err := f.Stat()
	if err != nil || !unchangedFile(info, after) {
		return "", fmt.Errorf("file changed during page read; read again")
	}
	version := fmt.Sprintf("size=%d;mtime_ns=%d", info.Size(), info.ModTime().UnixNano())
	if err := r.observeRead("file", path, version, start, next, content); err != nil {
		return "", fmt.Errorf("read observation persistence failed: %w", err)
	}
	if byteOffset != nil {
		return fmt.Sprintf("[byte_offset=%d next_byte_offset=%d total_bytes=%d complete=%v encoding=%s；源文件字节偏移，继续读取请传 byte_offset=%d]\n%s", start, next, info.Size(), next == info.Size(), textEncodingName(encoding), next, content), nil
	}
	return fmt.Sprintf("[第 %d-%d 行，共 %d 行；byte_offset=%d next_byte_offset=%d total_bytes=%d complete=%v；字节偏移指源文件]\n%s", currentLine, lastLine, total, start, next, info.Size(), next == info.Size(), content), nil
}
