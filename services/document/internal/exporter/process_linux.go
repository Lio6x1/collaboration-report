package exporter

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// 转换器与浏览器必须在自己的进程组中，context取消时清理整组，不能
// 只停止Pandoc/Node父进程而让孙进程继续写原RUNNING作业的制品。
func runProgram(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
	// Pdeathsig只覆盖直接子进程。Doc强制退出后的完整后代清理依赖正式
	// 部署的独立容器PID空间，不能把此选项当作宿主机全进程树保证。
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	if cmd.Cancel != nil {
		cmd.Cancel = func() error { return killProgramGroup(cmd) }
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer killProgramGroup(cmd)
	return cmd.Wait()
}

func killProgramGroup(cmd *exec.Cmd) error {
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
