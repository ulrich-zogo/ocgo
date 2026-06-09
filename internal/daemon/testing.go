package daemon

import (
	"time"

	"ocgo/internal/config"
	"ocgo/internal/process"
)

var (
	healthyFn         = process.Healthy
	readPIDFn         = config.ReadPID
	findListenerPIDFn = process.FindListenerPID
	killPIDFn         = process.KillPID
	startBackgroundFn = process.StartBackground
	waitHealthyFn     = process.WaitHealthy
)

type Runtime struct {
	Healthy         func(base string) bool
	ReadPID         func() (int, error)
	FindListenerPID func(port int) (int, error)
	KillPID         func(pid int) error
	StartBackground func() error
	WaitHealthy     func(base string, timeout time.Duration) error
}

func SetRuntimeForTest(r Runtime) (restore func()) {
	prevHealthy := healthyFn
	prevRead := readPIDFn
	prevFind := findListenerPIDFn
	prevKill := killPIDFn
	prevStart := startBackgroundFn
	prevWait := waitHealthyFn
	if r.Healthy != nil {
		healthyFn = r.Healthy
	}
	if r.ReadPID != nil {
		readPIDFn = r.ReadPID
	}
	if r.FindListenerPID != nil {
		findListenerPIDFn = r.FindListenerPID
	}
	if r.KillPID != nil {
		killPIDFn = r.KillPID
	}
	if r.StartBackground != nil {
		startBackgroundFn = r.StartBackground
	}
	if r.WaitHealthy != nil {
		waitHealthyFn = r.WaitHealthy
	}
	return func() {
		healthyFn = prevHealthy
		readPIDFn = prevRead
		findListenerPIDFn = prevFind
		killPIDFn = prevKill
		startBackgroundFn = prevStart
		waitHealthyFn = prevWait
	}
}
