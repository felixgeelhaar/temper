//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// configureDaemonProcess sets Unix-specific process attributes for daemon detachment
func configureDaemonProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
