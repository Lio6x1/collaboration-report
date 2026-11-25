//go:build linux

package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Linux发行版把应用放在同一个PID命名空间。另一个容器里看不到PID不能作为退出证据。
func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error) {
	if err := ctx.Err(); err != nil {
		return processFacts{}, err
	}
	// 先证明procfs可读；未挂载或无权限均是未知，绝不返回ABSENT。
	if _, err := os.ReadFile("/proc/self/cmdline"); err != nil {
		return processFacts{}, err
	}
	dir := filepath.Join("/proc", strconv.FormatUint(pid, 10))
	raw, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(dir); errors.Is(statErr, os.ErrNotExist) {
				return processFacts{State: "ABSENT"}, nil
			}
		}
		return processFacts{}, err
	}
	if len(raw) == 0 || raw[len(raw)-1] != 0 {
		// 僵尸/退出中的空命令行无法证明身份，等待真正回收后再重试。
		return processFacts{}, errors.New("PROCESS_ARGUMENTS_UNAVAILABLE")
	}
	return processFacts{State: "PRESENT", Args: strings.Split(string(raw[:len(raw)-1]), "\x00")}, nil
}
