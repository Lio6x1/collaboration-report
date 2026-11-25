package exporter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// 部署验收把本程序作为容器PID1：真实退出它，检查转换后代所在的
	// PID空间一起终止。标记文件位于本次独立证据目录，不触碰正式资源。
	if directory := os.Getenv("EXPORT_NAMESPACE_EVIDENCE"); directory != "" {
		cmd := exec.Command("/bin/sh", "-c", `sleep 600 & echo $! > "$1/child.pid"; wait`, "converter", directory)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := runProgram(cmd); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunProgramCancelsConverterGroup(t *testing.T) {
	directory := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `sleep 600 & echo $! > "$1/child.pid"; wait`, "converter", directory)
	done := make(chan error, 1)
	go func() { done <- runProgram(cmd) }()
	pid, err := waitChildPID(ctx, filepath.Join(directory, "child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("取消转换不能报告成功")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("转换器未在取消后退出")
	}
	// 杀死的后代可能暂时等待PID1回收成为僵尸；Z不再能写文件。
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if errors.Is(readErr, os.ErrNotExist) {
			return
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		fields := strings.Fields(string(data)[strings.LastIndexByte(string(data), ')')+1:])
		if len(fields) > 0 && fields[0] == "Z" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("取消后转换孙进程仍在运行")
}

func waitChildPID(ctx context.Context, path string) (int, error) {
	for {
		data, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strconv.Atoi(strings.TrimSpace(string(data)))
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
