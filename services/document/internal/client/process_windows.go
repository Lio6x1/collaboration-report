//go:build windows

package client

import (
	"os/exec"
	"syscall"
)

// PID归属检查是后台恢复的一部分，不应在Windows桌面反复弹出PowerShell窗口。
func hideProbeWindow(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
