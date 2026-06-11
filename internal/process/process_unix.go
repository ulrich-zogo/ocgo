//go:build !windows

package process

import "syscall"

func DetachedAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func signalZeroAvailable() bool {
	return true
}
