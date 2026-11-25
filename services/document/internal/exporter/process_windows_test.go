package exporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestMain(m *testing.M) {
	switch os.Getenv("EXPORT_CONSOLE_TEST_HELPER") {
	case "launcher":
		// 模拟本机后台Doc：父进程本身没有控制台，默认启动控制台程序会新建窗口。
		if codePage, _ := windows.GetConsoleCP(); codePage != 0 {
			fmt.Fprintln(os.Stderr, "测试父进程必须脱离控制台")
			os.Exit(1)
		}
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "EXPORT_CONSOLE_TEST_HELPER=probe")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := runProgram(cmd); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				os.Exit(exit.ExitCode())
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "probe":
		// 查实际Windows控制台句柄，而非只检查启动参数是否被赋值。
		kernel := windows.NewLazySystemDLL("kernel32.dll")
		window, _, _ := kernel.NewProc("GetConsoleWindow").Call()
		fmt.Fprintf(os.Stdout, "window=%d\n", window)
		fmt.Fprintln(os.Stderr, "converter diagnostic")
		os.Exit(7)
	case "tree-parent":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "EXPORT_CONSOLE_TEST_HELPER=tree-child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
		if err := child.Run(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	case "tree-child":
		// 孙进程保留父进程的输出句柄；只杀父进程无法让Go的复制管道读到EOF。
		fmt.Fprintln(os.Stdout, "descendant ready")
		pids := fmt.Sprintf("%d %d", os.Getppid(), os.Getpid())
		if err := os.WriteFile(os.Getenv("EXPORT_TREE_PID_FILE"), []byte(pids), 0600); err != nil {
			os.Exit(2)
		}
		// 清理文件独立于被测Job：就绪前发生测试失败也能可靠收回后代；
		// 测试被外部中断时还有30秒自身上限，不遗留长驻helper。
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(os.Getenv("EXPORT_TREE_PID_FILE") + ".stop"); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// 验收真实取消与管道EOF，不只检查Cancel是否赋值；失败路径也须收回两代helper。
func TestRunProgramCancelsDescendantsHoldingOutput(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "tree.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), "EXPORT_CONSOLE_TEST_HELPER=tree-parent", "EXPORT_TREE_PID_FILE="+pidFile)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	done := make(chan error, 1)
	go func() { done <- runProgram(cmd) }()
	var handles []windows.Handle
	finished := false
	t.Cleanup(func() {
		cancel()
		if err := os.WriteFile(pidFile+".stop", nil, 0600); err != nil {
			t.Error(err)
		}
		for _, handle := range handles {
			_ = windows.TerminateProcess(handle, 1)
			_, _ = windows.WaitForSingleObject(handle, 2000)
			_ = windows.CloseHandle(handle)
		}
		if !finished {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("清理helper后转换等待仍未结束")
			}
		}
	})
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(strings.Fields(string(data))) == 2 {
			for _, raw := range strings.Fields(string(data)) {
				pid, err := strconv.ParseUint(raw, 10, 32)
				if err != nil {
					t.Fatal(err)
				}
				handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
				if err != nil {
					t.Fatal(err)
				}
				handles = append(handles, handle)
			}
			break
		}
		select {
		case err := <-done:
			finished = true
			t.Fatalf("helper尚未生成后代：%v", err)
		case <-ctx.Done():
			t.Fatal("未观察到helper进程树就绪")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		finished = true
		if err == nil {
			t.Fatal("取消转换不能报告成功")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("取消只杀了父进程，后代占住输出管道使转换无法返回")
	}
	for _, handle := range handles {
		if state, err := windows.WaitForSingleObject(handle, 1000); err != nil || state != windows.WAIT_OBJECT_0 {
			t.Fatalf("转换返回后进程仍未退出：state=%d err=%v", state, err)
		}
	}
	if output.String() != "descendant ready\n" {
		t.Fatalf("应保留取消前的输出：%q", output.String())
	}
}

func TestRunProgramDoesNotCreateConsole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), "EXPORT_CONSOLE_TEST_HELPER=launcher")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.DETACHED_PROCESS}
	var output, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &diagnostic
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("后台转换仍须保留真实失败退出码：%v，%s", err, diagnostic.String())
	}
	if output.String() != "window=0\n" {
		t.Fatalf("后台转换不应创建控制台窗口：%s", output.String())
	}
	if diagnostic.String() != "converter diagnostic\n" {
		t.Fatalf("隐藏窗口不能丢失转换诊断：%s", diagnostic.String())
	}
}

// 取消可能早于Start，或在Start返回/加入Job之间发生；不能让Cancel等待装配而卡住Wait。
func TestRunProgramCancellationAroundStart(t *testing.T) {
	for _, delay := range []time.Duration{0, 100 * time.Microsecond} {
		for range 8 {
			ctx, cancel := context.WithCancel(context.Background())
			if delay == 0 {
				cancel()
			}
			cmd := exec.CommandContext(ctx, os.Args[0])
			cmd.Env = append(os.Environ(), "EXPORT_CONSOLE_TEST_HELPER=probe")
			var diagnostic bytes.Buffer
			cmd.Stdout, cmd.Stderr = &diagnostic, &diagnostic
			done := make(chan error, 1)
			go func() { done <- runProgram(cmd) }()
			timer := time.AfterFunc(delay, cancel)
			select {
			case err := <-done:
				if err == nil {
					t.Error("启动期间取消不能报告成功")
				}
			case <-time.After(2 * time.Second):
				t.Error("启动期间取消阻塞了进程收回")
			}
			timer.Stop()
			cancel()
		}
	}
}
