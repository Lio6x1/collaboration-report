//go:build linux

package client

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// 用真实Linux子进程核验原PID/入口/启动标记，不能用假ABSENT替代OS退出事实。
func TestProcessOwnerLinuxKeepsLiveOwnerAndAcceptsActualExit(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, ".runtime", "bin", "document")
	if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(executable, entry); err != nil {
		t.Fatal(err)
	}
	owner := "linux-owner-proof"
	child := exec.Command(entry, "-test.run=^TestProcessOwnerLinuxHelper$", "--", "--run-owner", owner)
	child.Env = append(os.Environ(), "COLLAB_PROCESS_OWNER_HELPER=1")
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		input.Close()
		if !waited {
			child.Process.Kill()
			child.Wait()
		}
	})
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == "PROCESS_OWNER_READY" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("actual child did not become ready")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("actual child readiness timeout")
	}
	probe := NewProcessOwner(root)
	pid := uint64(child.Process.Pid)
	if state, err := probe.Verify(context.Background(), pid, "document", "linux-owner"); err != nil || state != "PRESENT" {
		t.Fatalf("owner prefix is not the original instance: state=%q err=%v", state, err)
	}
	if _, err = probe.Verify(context.Background(), pid, "document", owner); err == nil || err.Error() != "OLD_OUTPUT_PROCESS_STILL_RUNNING" {
		t.Fatalf("live original output must remain blocked: %v", err)
	}
	// 当前PID属于另一启动标记时只能确认原实例已不在，不能误把同前缀当原owner。
	if state, err := probe.Verify(context.Background(), pid, "document", owner+"-another"); err != nil || state != "PRESENT" {
		t.Fatalf("different original owner: state=%q err=%v", state, err)
	}
	input.Close()
	if err = child.Wait(); err != nil {
		t.Fatal(err)
	}
	waited = true
	if state, err := probe.Verify(context.Background(), pid, "document", owner); err != nil || state != "ABSENT" {
		t.Fatalf("actual reaped child: state=%q err=%v", state, err)
	}
}

func TestProcessOwnerLinuxHelper(t *testing.T) {
	if os.Getenv("COLLAB_PROCESS_OWNER_HELPER") != "1" {
		return
	}
	fmt.Println("PROCESS_OWNER_READY")
	// 等父进程关闭stdin后正常退出，由父进程Wait获得真正退出的内核事实。
	os.Stdin.Read(make([]byte, 1))
}
