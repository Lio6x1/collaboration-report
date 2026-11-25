//go:build windows

package client

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Windows继续通过既有只读PowerShell查询，保持进程启动标记和超时语义。
func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error) {
	probe := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-File", filepath.Join(c.root, "scripts/process-owner.ps1"), "-ProcessId", strconv.FormatUint(pid, 10))
	hideProbeWindow(probe)
	raw, err := probe.Output()
	if err != nil {
		return processFacts{}, err
	}
	var facts processFacts
	if json.Unmarshal(raw, &facts) != nil {
		return processFacts{}, errors.New("PROCESS_FACTS_INVALID")
	}
	return facts, nil
}
