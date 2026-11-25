//go:build !windows && !linux

package exporter

import "os/exec"

// 未验收的平台只保留直接进程取消；正式部署支持Windows Job Object与Linux容器。
func runProgram(cmd *exec.Cmd) error { return cmd.Run() }
