//go:build windows

package process

import "syscall"

func DetachedAttrs() *syscall.SysProcAttr {
	return nil
}

func signalZeroAvailable() bool {
	return false
}
