//go:build !windows

package oss

import "os"

func privateMode(mode os.FileMode) bool { return mode.Perm()&0077 == 0 }
