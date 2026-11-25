//go:build windows

package oss

import "os"

// Windows 文件权限由 ACL 表达，不能把 os.FileMode 的 0666 当成实际公开访问。
func privateMode(os.FileMode) bool { return true }
