package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// M3-B: Sandboxie configuration automation. Idempotently ensures the agent box
// exists with Enabled=y and OpenFilePath direct access for the workspace,
// then reloads. Edits go through SbieIni.exe (validated, works for non-admin
// users because SbieSvc performs the write); the ini is only READ directly.
func ensureSandboxieConfig(startExe, box, workspace string) error {
	dir := filepath.Dir(startExe)
	sbieIni := filepath.Join(dir, "SbieIni.exe")
	iniPath := `C:\Windows\Sandboxie.ini`

	content, err := os.ReadFile(iniPath)
	if err != nil {
		content = nil // may not exist yet on fresh installs
	}
	lines := strings.Split(string(content), "\n")

	wantedOpen := workspace + `\*`
	inBox := false
	hasBox := false
	hasEnabled := false
	hasOpen := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			inBox = strings.EqualFold(t, "["+box+"]")
			if inBox {
				hasBox = true
			}
			continue
		}
		if !inBox {
			continue
		}
		lower := strings.ToLower(t)
		if strings.HasPrefix(lower, "enabled=") {
			hasEnabled = true
		}
		if strings.HasPrefix(lower, "openfilepath=") && strings.EqualFold(strings.TrimSpace(t[len("OpenFilePath="):]), wantedOpen) {
			hasOpen = true
		}
	}

	// On a fresh box the section itself is created by the first `set`.
	if !hasBox {
		if out, err := runSbieIni(sbieIni, "set", box, "Enabled", "y"); err != nil {
			return sbieErr("create box", out, err)
		}
		hasEnabled = true
	}
	if !hasEnabled {
		if out, err := runSbieIni(sbieIni, "set", box, "Enabled", "y"); err != nil {
			return sbieErr("set Enabled", out, err)
		}
	}
	if !hasOpen {
		if out, err := runSbieIni(sbieIni, "append", box, "OpenFilePath", wantedOpen); err != nil {
			return sbieErr("append OpenFilePath", out, err)
		}
	}

	// Reload so the running service picks up any change we just made.
	if _, err := exec.Command(startExe, "/reload").Output(); err != nil {
		return sbieErr("reload", "", err)
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

func runSbieIni(sbieIni string, args ...string) (string, error) {
	out, err := exec.Command(sbieIni, args...).CombinedOutput()
	return string(out), err
}

func sbieErr(step, out string, err error) error {
	return &sbieConfigError{step: step, out: out, err: err}
}

type sbieConfigError struct {
	step    string
	out     string
	err     error
}

func (e *sbieConfigError) Error() string {
	msg := "sandboxie config step '" + e.step + "' failed"
	if e.out != "" {
		msg += ": " + strings.TrimSpace(e.out)
	}
	if e.err != nil {
		msg += " (" + e.err.Error() + ")"
	}
	return msg
}

// removeSandboxieBox: uninstall-time cleanup — drop the box section entirely.
func removeSandboxieBox(startExe, box string) error {
	dir := filepath.Dir(startExe)
	sbieIni := filepath.Join(dir, "SbieIni.exe")
	_, _ = runSbieIni(sbieIni, "delete", box) // remove whole section
	_, err := exec.Command(startExe, "/reload").Output()
	return err
}
