//go:build !windows

package client

import "os/exec"

func hideProbeWindow(_ *exec.Cmd) {}
