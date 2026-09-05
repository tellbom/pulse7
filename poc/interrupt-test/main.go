// dev-only interrupt tester: starts the agent in its own process group and
// delivers real console ctrl events (CTRL_BREAK maps to os.Interrupt in Go).
// usage: interrupt-test <one|two> <exe> [args...]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

var (
	k32                     = syscall.NewLazyDLL("kernel32.dll")
	procGenerateCtrlEvent   = k32.NewProc("GenerateConsoleCtrlEvent")
)

func sendBreak(pgid uint32) bool {
	r1, _, _ := procGenerateCtrlEvent.Call(1, uintptr(pgid)) // CTRL_BREAK_EVENT
	return r1 != 0
}

func main() {
	mode := os.Args[1]
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		fmt.Println("START-ERR:", err)
		os.Exit(2)
	}
	fmt.Println("[tester] child pid:", cmd.Process.Pid)
	time.Sleep(6 * time.Second)
	fmt.Println("[tester] sending CTRL_BREAK #1")
	sendBreak(uint32(cmd.Process.Pid))
	if mode == "two" {
		time.Sleep(400 * time.Millisecond)
		fmt.Println("[tester] sending CTRL_BREAK #2")
		sendBreak(uint32(cmd.Process.Pid))
	}
	err := cmd.Wait()
	fmt.Println("[tester] child exit err:", err)
	_ = unsafe.Sizeof(0)
}
