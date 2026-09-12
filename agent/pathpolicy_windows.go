package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// Policy resolves reparse points and protects Git metadata. Multi-link files
// remain readable; mutation handle validation rejects their unknown impact.
type Policy struct {
	Workspace string
}

var getFinalPathNameByHandleW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

func (p *Policy) Check(path string) error {
	_, err := p.Resolve(path)
	return err
}

func (p *Policy) Resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := rejectProtectedPath(p.Workspace, abs); err != nil {
		return "", err
	}
	resolved, _, _, err := resolveExistingPrefix(abs)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	if err := rejectProtectedPath(p.Workspace, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func (p *Policy) ResolveRead(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, _, _, err := resolveExistingPrefix(abs)
	if err != nil {
		return "", fmt.Errorf("resolve read path %s: %w", path, err)
	}
	return resolved, nil
}

// ValidateOpenFile binds the boundary decision to the exact handle used for
// I/O. This closes the check/open race for reads and for writes performed on
// an already opened, non-truncated handle.
func (p *Policy) ValidateOpenFile(f *os.File, expected string) error {
	final, info, err := finalPathFromHandle(syscall.Handle(f.Fd()))
	if err != nil {
		return err
	}
	if err := rejectProtectedPath(p.Workspace, final); err != nil {
		return err
	}
	if info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 && info.NumberOfLinks > 1 {
		return fmt.Errorf("hard_link_impact_unknown: 文件 %s 存在 %d 个硬链接名称，pulse7 无法确定本次写入实际影响哪些路径，因此拒绝", expected, info.NumberOfLinks)
	}
	if !strings.EqualFold(filepath.Clean(expected), filepath.Clean(final)) {
		return fmt.Errorf("path identity changed during operation: %s opened as %s", expected, final)
	}
	return nil
}

func (p *Policy) ValidateOpenRead(f *os.File, expected string) error {
	final, _, err := finalPathFromHandle(syscall.Handle(f.Fd()))
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(expected), filepath.Clean(final)) {
		return fmt.Errorf("path identity changed during read: %s opened as %s", expected, final)
	}
	return nil
}

func requirePathWithin(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("outside workspace")
	}
	return nil
}

func rejectProtectedPath(root, path string) error {
	rel := strings.TrimPrefix(filepath.Clean(path), filepath.VolumeName(path))
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		if strings.EqualFold(part, ".git") {
			return fmt.Errorf("path %s is protected Git metadata", path)
		}
		if strings.Contains(part, ":") {
			return fmt.Errorf("path %s uses a protected alternate data stream", path)
		}
	}
	return nil
}

// resolveExistingPrefix opens the closest existing path component and asks the
// kernel for its final name. Missing suffixes are then appended to that final
// parent, so a write through a junction cannot escape before the file exists.
func resolveExistingPrefix(path string) (resolved string, exists bool, links uint32, err error) {
	current := filepath.Clean(path)
	var suffix []string
	for {
		_, statErr := os.Lstat(current)
		if statErr == nil {
			break
		}
		if !os.IsNotExist(statErr) {
			return "", false, 0, statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false, 0, statErr
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		current = parent
	}
	final, info, err := finalPathInformation(current)
	if err != nil {
		return "", false, 0, err
	}
	resolved = final
	for _, part := range suffix {
		resolved = filepath.Join(resolved, part)
	}
	exists = len(suffix) == 0
	if exists && info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
		links = info.NumberOfLinks
	}
	return resolved, exists, links, nil
}

func finalPathInformation(path string) (string, syscall.ByHandleFileInformation, error) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", syscall.ByHandleFileInformation{}, err
	}
	h, err := syscall.CreateFile(ptr, 0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", syscall.ByHandleFileInformation{}, err
	}
	defer syscall.CloseHandle(h)
	return finalPathFromHandle(h)
}

func finalPathFromHandle(h syscall.Handle) (string, syscall.ByHandleFileInformation, error) {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return "", info, err
	}
	buf := make([]uint16, syscall.MAX_LONG_PATH)
	n, _, callErr := getFinalPathNameByHandleW.Call(
		uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if n == 0 || n >= uintptr(len(buf)) {
		return "", info, fmt.Errorf("GetFinalPathNameByHandleW: %v", callErr)
	}
	return normalizeFinalPath(syscall.UTF16ToString(buf[:n])), info, nil
}

func normalizeFinalPath(path string) string {
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	}
	return strings.TrimPrefix(path, `\\?\`)
}
