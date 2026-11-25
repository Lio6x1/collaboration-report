package client

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ProcessOwner struct{ root string }

type processFacts struct {
	State   string `json:"state"`
	Command string `json:"commandLine"`
	Args    []string
}

func NewProcessOwner(root string) *ProcessOwner { return &ProcessOwner{root: root} }

// Verify只接受指定PID、绝对入口和run-owner的实际证据；查询超时不等于旧进程已退出。
func (c *ProcessOwner) Verify(ctx context.Context, pid uint64, role, owner string) (string, error) {
	if pid == 0 || pid > 2147483647 || owner == "" || (role != "document" && role != "edge") {
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	facts, err := c.probeProcess(probeCtx, pid)
	// 先读取再取消，避免主动 cancel 把真实的探测超时原因覆盖掉；失败仍不能当退出证明。
	probeError := probeCtx.Err()
	cancel()
	if err != nil || probeError != nil {
		if probeError == context.DeadlineExceeded {
			return "", errors.New("OUTPUT_EXIT_PROOF_DEADLINE")
		}
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}
	entry := filepath.Join(c.root, ".runtime/bin/document.exe")
	if runtime.GOOS == "linux" {
		entry = filepath.Join(c.root, ".runtime/bin/document")
	}
	if role == "edge" {
		entry = filepath.Join(c.root, "scripts/edge.mjs")
	}
	matched := strings.Contains(strings.ToLower(facts.Command), strings.ToLower(entry)) && strings.Contains(facts.Command, "--run-owner "+owner)
	if runtime.GOOS == "linux" {
		// procfs给出独立argv，必须精确匹配入口与参数值，owner前缀不能代表原实例。
		entryFound, ownerFound := false, false
		for i, arg := range facts.Args {
			entryFound = entryFound || arg == entry
			if arg == "--run-owner" && i+1 < len(facts.Args) && facts.Args[i+1] == owner {
				ownerFound = true
			}
		}
		matched = entryFound && ownerFound
	}
	if facts.State == "PRESENT" && matched {
		return "", errors.New("OLD_OUTPUT_PROCESS_STILL_RUNNING")
	}
	if facts.State != "PRESENT" && facts.State != "ABSENT" {
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}

	return facts.State, nil
}
