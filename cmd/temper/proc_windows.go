//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// configureDaemonProcess sets Windows-specific process attributes for daemon detachment
func configureDaemonProcess(cmd *exec.Cmd) {
	// CREATE_NEW_PROCESS_GROUP detaches from parent console on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
