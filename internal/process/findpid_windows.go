//go:build windows

package process

import (
	"errors"
	"os/exec"
)

func FindListenerPID(port int) (int, error) {
	if port == 0 {
		return 0, errors.New("missing port")
	}
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return 0, err
	}
	return ParseWindowsNetstatPID(string(out), port)
}
