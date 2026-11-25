package exporter

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 真实转换进程树绑定本Go进程持有的Job Object；Doc退出时OS关闭句柄，
// Pandoc/Node/浏览器子进程一起终止，原RUNNING不能由遗留转换继续写文件。
func runProgram(cmd *exec.Cmd) error {
	// 后台Doc没有控制台，Windows默认会为Pandoc/Node等控制台程序新建窗口。
	// 不创建控制台窗口并隐藏启动窗口，标准输出/错误仍通过原管道收集；
	// 在既有属性上追加，保留调用方设置和下方Job Object的进程树回收约束。
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	assigned := make(chan struct{})
	inJob := false
	if cmd.Cancel != nil {
		// CommandContext默认只杀直接父进程；后代若继承输出管道，Wait会等EOF，
		// 而defer关闭Job又必须等Wait，形成死等。取消时直接终止整棵Job树。
		cmd.Cancel = func() error {
			// Start成功后watchCtx即可调用Cancel，必须等Assign结果，不能先终止
			// 空Job再把仍存活的转换器加入。close也发布inJob，避免并发读写。
			<-assigned
			if inJob {
				return windows.TerminateJobObject(job, 1)
			}
			return cmd.Process.Kill()
		}
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
	inJob = err == nil
	close(assigned)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	// Wait也等待watchCtx/Cancel结束；之后defer才关闭句柄，取消不能访问已关闭Job。
	return cmd.Wait()
}
