# services/document/internal/exporter/process_linux.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/122-process-linux.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：转换固定版本，管理进程和产物；不能重新读取变化中的正文。

主要声明：`runProgram`、`killProgramGroup`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`c9c57ad45d0e7ed9f871ceb65dbc637cfe71bce6f1e11c5628574e5cdc1c508c`。[参考实现](../../../../services/document/internal/exporter/process_linux.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
