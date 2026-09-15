package main

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"
)

func resolveStartupMode(args []string, cli bool) (string, bool, error) {
	if len(args) > 0 {
		if cli && args[0] != "repl" {
			return "", false, errors.New("--cli cannot be combined with another subcommand")
		}
		return args[0], false, nil
	}
	if cli {
		return "repl", false, nil
	}
	return "serve", true, nil
}

// Keep the console-subsystem executable so exec/redirects/workers retain their
// existing Windows behavior. Only detach the console allocated for this process;
// never hide the caller's shared PowerShell/cmd window.
func detachOwnedConsole() {
	var processes [2]uint32
	k := syscall.NewLazyDLL("kernel32.dll")
	n, _, _ := k.NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&processes[0])), 2)
	if n == 1 {
		k.NewProc("FreeConsole").Call()
	}
}

func openWebBrowser(url string) error {
	// URL is generated from our numeric listener port; it contains no token or user input.
	c := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait()
	return nil
}

func showStartupError(message string) {
	text, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("pulse7")
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
