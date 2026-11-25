//go:build !windows && !linux

package client

import (
	"context"
	"errors"
)

// 未提供内核事实适配的平台不能猜测旧实例已经退出。
func (c *ProcessOwner) probeProcess(context.Context, uint64) (processFacts, error) {
	return processFacts{}, errors.New("PROCESS_PLATFORM_UNSUPPORTED")
}
