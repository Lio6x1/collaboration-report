//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package local

import "os"

// 不支持可靠文件锁的平台必须拒绝操作，不能降级为仅进程内互斥。
func lockFile(*os.File, bool) error { return ErrUnavailable }
func unlockFile(*os.File)           {}
