package main

// ansi.go: shell wrapper I/O transcoding (encoding-pagination T2).
//
// Chinese Windows consoles run codepage 936 (GBK): a shell command's OUTPUT
// (dir listings, error messages, compiler diagnostics) is ANSI-encoded bytes,
// while the agent's whole pipeline is UTF-8. Without this shim the model
// context receives mojibake and json.Marshal irreversibly replaces the
// invalid bytes with U+FFFD. The COMMAND direction has the mirror problem:
// inner.cmd is parsed by cmd.exe under the ANSI codepage, so a UTF-8 command
// containing a Chinese path resolves to garbage.
//
// Both conversions go through Win32 (same style as jobobject.go) using
// CP_ACP - the system's current ANSI codepage - so behavior matches whatever
// cmd.exe actually does, instead of hardcoding 936. No third-party deps.

import (
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var (
	procMultiByteToWideChar = k32.NewProc("MultiByteToWideChar")
	procWideCharToMultiByte = k32.NewProc("WideCharToMultiByte")
)

const cpAcp = 0 // CP_ACP: system default Windows ANSI codepage

// ansiToUTF8 converts ANSI-codepage bytes (e.g. GBK console output) to UTF-8.
// Undecodable sequences become U+FFFD via the wide-char API (lossy input is
// flagged, never silently dropped).
func ansiToUTF8(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// size query
	n, _, _ := procMultiByteToWideChar.Call(
		cpAcp, 0,
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
		0, 0)
	if n <= 0 {
		return string(b) // conversion impossible; keep raw bytes
	}
	buf := make([]uint16, n)
	n, _, _ = procMultiByteToWideChar.Call(
		cpAcp, 0,
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(n))
	if n <= 0 {
		return string(b)
	}
	return string(utf16.Decode(buf[:n]))
}

// utf8ToANSI converts a UTF-8 command string to ANSI-codepage bytes for
// inner.cmd, so cmd.exe parses Chinese paths/arguments correctly. Characters
// without an ANSI representation degrade to the system default char ('?').
func utf8ToANSI(s string) []byte {
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
		cpAcp, 0,
		uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)),
		0, 0, 0, 0)
	if n <= 0 {
		return []byte(s)
	}
	buf := make([]byte, n)
	n, _, _ = procWideCharToMultiByte.Call(
		cpAcp, 0,
		uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(n),
		0, 0)
	if n <= 0 {
		return []byte(s)
	}
	return buf[:n]
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// decodeShellOutput: out.txt bytes -> UTF-8 tool-result string.
// Output that is already valid UTF-8 (e.g. a tool that printed UTF-8, or a
// console switched to 65001) passes through untouched; only non-UTF-8 bytes
// are interpreted in the ANSI codepage.
func decodeShellOutput(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return ansiToUTF8(b)
}
