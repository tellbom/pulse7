package main

// ansi.go: codepage transcoding for shell I/O and workspace files
// (file-encoding T1, amendments A1/A2/A3).
//
// Two DIFFERENT codepage sources, on purpose — do not "unify" them:
//
//   shell OUTPUT decoding uses GetConsoleOutputCP() (A1): it describes what
//   cmd.exe children on THIS machine actually wrote. Chinese Win7: 936;
//   English Win7: 437 (where CP_ACP=1252 would be wrong). Falls back to
//   CP_ACP when there is no console (session 0 / service context).
//
//   FILE content decoding hardcodes 936 (A2): a file's encoding depends on
//   WHO WROTE IT, not on who reads it. In this environment non-UTF-8 files
//   are GBK (Chinese-locale colleagues' source code), so an English Win7
//   must still decode them as 936 — CP_ACP there is 1252 and would yield
//   plausible-looking mojibake with no U+FFFD to flag it.
//
// All conversions go through Win32 (same lazy-proc style as jobobject.go),
// zero third-party deps.

import (
	"bytes"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var (
	procMultiByteToWideChar = k32.NewProc("MultiByteToWideChar")
	procWideCharToMultiByte = k32.NewProc("WideCharToMultiByte")
	procGetConsoleOutputCP  = k32.NewProc("GetConsoleOutputCP")
)

const (
	cpAcp = 0 // CP_ACP: system default Windows ANSI codepage
	cpGBK = 936
)

// consoleCodepage (A1): the codepage this machine's cmd.exe children used
// for their output — this is what out.txt bytes are encoded in. Returns
// CP_ACP when no console is attached (GetConsoleOutputCP yields 0).
func consoleCodepage() uint {
	cp, _, _ := procGetConsoleOutputCP.Call()
	if cp == 0 {
		return cpAcp
	}
	return uint(cp)
}

// bytesToUTF8 converts codepage-encoded bytes to a UTF-8 string.
// Undecodable sequences surface as U+FFFD (flagged, never silently dropped).
func bytesToUTF8(b []byte, cp uint) string {
	if len(b) == 0 {
		return ""
	}
	n, _, _ := procMultiByteToWideChar.Call(
		uintptr(cp), 0,
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
		0, 0)
	if n <= 0 {
		return string(b) // conversion impossible; keep raw bytes
	}
	buf := make([]uint16, n)
	n, _, _ = procMultiByteToWideChar.Call(
		uintptr(cp), 0,
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(n))
	if n <= 0 {
		return string(b)
	}
	return string(utf16.Decode(buf[:n]))
}

// utf8ToCodepage converts a UTF-8 string to codepage bytes. Characters the
// codepage cannot represent degrade to the system default char ('?').
func utf8ToCodepage(s string, cp uint) []byte {
	if s == "" {
		return nil
	}
	if utf8.ValidString(s) {
		// fast path: pure ASCII round-trips identically in every codepage
		if isASCII(s) {
			return []byte(s)
		}
	}
	u16 := utf16.Encode([]rune(s))
	n, _, _ := procWideCharToMultiByte.Call(
		uintptr(cp), 0,
		uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)),
		0, 0, 0, 0)
	if n <= 0 {
		return []byte(s)
	}
	buf := make([]byte, n)
	n, _, _ = procWideCharToMultiByte.Call(
		uintptr(cp), 0,
		uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(n),
		0, 0)
	if n <= 0 {
		return []byte(s)
	}
	return buf[:n]
}

// codepageRoundTrips reports whether s survives a UTF-8 -> cp -> UTF-8 round
// trip unchanged, i.e. every character is representable in cp. Lossy strings
// must NOT be used for byte matching (a '?' could false-match) or written
// back (silent corruption of the model's intent).
func codepageRoundTrips(s string, cp uint) bool {
	return bytesToUTF8(utf8ToCodepage(s, cp), cp) == s
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// decodeShellOutput (A1): out.txt bytes -> UTF-8 tool-result string.
// Output that is already valid UTF-8 (a tool printing UTF-8, or a console
// switched to 65001) passes through untouched.
func decodeShellOutput(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return bytesToUTF8(b, consoleCodepage())
}

// fileEnc describes how a workspace file is encoded.
type fileEnc int

const (
	encUTF8 fileEnc = iota
	encGBK
	encUTF16LE
	encUTF16BE
	encBinary
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// detectFileEncoding sniffs a file's encoding from its raw bytes:
// BOMs first, then UTF-8 validity, then a NUL-binary heuristic, else GBK
// (A2: hardcoded — see the file header for why this must not be CP_ACP).
func detectFileEncoding(b []byte) (enc fileEnc, hasBOM bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE, 0x00, 0x00}):
		return encBinary, false // UTF-32: refuse rather than mangle
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return encUTF16LE, true
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return encUTF16BE, true
	case bytes.HasPrefix(b, utf8BOM):
		return encUTF8, true
	}
	if utf8.Valid(b) {
		return encUTF8, false
	}
	if bytes.IndexByte(b, 0) >= 0 {
		return encBinary, false
	}
	return encGBK, false
}

// decodeFileBytes (A2): raw file bytes -> UTF-8 view for the model.
// Returns "" and ok=false when the file should not be fed as text
// (binary / UTF-16), with reason filled in.
func decodeFileBytes(b []byte) (text string, ok bool, reason string) {
	enc, hasBOM := detectFileEncoding(b)
	switch enc {
	case encUTF8:
		return string(bytes.TrimPrefix(b, utf8BOM)), true, ""
	case encGBK:
		return bytesToUTF8(b, cpGBK), true, ""
	case encUTF16LE, encUTF16BE:
		return "", false, "UTF-16 encoded file: edit/read as text is not supported, refusing to guess (bytes untouched)"
	case encBinary:
		return "", false, "binary file (NUL byte or UTF-32 detected): not read as text"
	}
	_ = hasBOM
	return string(b), true, ""
}

// encodeFileBytes (A3/T1.2): UTF-8 content -> raw bytes in the file's
// original encoding, preserving its BOM. Used by write (overwrite) and edit
// write-back so unrelated bytes and the file's identity stay intact.
func encodeFileBytes(content string, enc fileEnc, hasBOM bool) []byte {
	var out []byte
	switch enc {
	case encGBK:
		out = utf8ToCodepage(content, cpGBK)
	case encUTF16LE:
		u := utf16.Encode([]rune(content))
		out = make([]byte, 0, len(u)*2)
		for _, c := range u {
			out = append(out, byte(c), byte(c>>8))
		}
		if hasBOM {
			out = append([]byte{0xFF, 0xFE}, out...)
		}
		return out
	case encUTF16BE:
		u := utf16.Encode([]rune(content))
		out = make([]byte, 0, len(u)*2)
		for _, c := range u {
			out = append(out, byte(c>>8), byte(c))
		}
		if hasBOM {
			out = append([]byte{0xFE, 0xFF}, out...)
		}
		return out
	default: // encUTF8
		out = []byte(content)
	}
	if hasBOM {
		out = append(append([]byte{}, utf8BOM...), out...)
	}
	return out
}
