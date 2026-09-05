// Minimal reproduction of the agent's exact sbxRunner.Run() path.
// Run from /it task on Win7 to match the agent's execution context.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func main() {
	home := os.Getenv("USERPROFILE")
	ws := `C:\Users\user\real-e2e\S1`
	startExe := `C:\Program Files\Sandboxie\Start.exe`
	box := "Win7Agent"

	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	dir := filepath.Join(home, ".pulse7", "run", id)
	os.MkdirAll(dir, 0755)

	inner := filepath.Join(dir, "inner.cmd")
	bat := filepath.Join(dir, "run.bat")
	rel := `.pulse7\run\` + id

	os.WriteFile(inner, []byte("python calc.py\r\n"), 0644)
	content := "@echo off\r\n" +
		`cd /d "` + ws + `"` + "\r\n" +
		`call "` + inner + `" > "%USERPROFILE%\` + rel + `\out.txt" 2>&1` + "\r\n" +
		`echo %errorlevel% > "%USERPROFILE%\` + rel + `\ec.txt"` + "\r\n"
	os.WriteFile(bat, []byte(content), 0644)

	fmt.Println("[go-test] bat:", bat)
	fmt.Println("[go-test] running Start.exe...")

	cmd := exec.Command(startExe, "/silent", "/box:"+box, "/wait", "cmd.exe", "/c", bat)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	fmt.Printf("[go-test] CombinedOutput err=%v out=%q\n", err, string(out))

	ctnDir := filepath.Join(`C:\Sandbox`, "user", box, "user", "current", ".pulse7", "run", id)
	outB, oerr := os.ReadFile(filepath.Join(ctnDir, "out.txt"))
	ecB, eerr := os.ReadFile(filepath.Join(ctnDir, "ec.txt"))
	fmt.Printf("[go-test] container out.txt: err=%v content=%q\n", oerr, string(outB))
	fmt.Printf("[go-test] container ec.txt:  err=%v content=%q\n", eerr, string(ecB))
	fmt.Println("[go-test] DONE")
}
