package process

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestStatusForInvalidPID(t *testing.T) {
	if s := StatusForPID(-1); s != StatusStale {
		t.Errorf("StatusForPID(-1) = %q, want %q", s, StatusStale)
	}
}

func TestStatusForSelfPID(t *testing.T) {
	pid := os.Getpid()
	s := StatusForPID(pid)
	if runtime.GOOS == "windows" {
		if s != StatusUnknown {
			t.Errorf("StatusForPID(self) on windows = %q, want %q", s, StatusUnknown)
		}
	} else {
		if s != StatusPresent {
			t.Errorf("StatusForPID(self) = %q, want %q", s, StatusPresent)
		}
	}
}

func TestStatusForTerminatedChildPID(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessExitImmediately")
	cmd.Env = append(os.Environ(), "OCGO_TEST_HELPER_PROCESS_STATUS=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	s := StatusForPID(pid)
	if s != StatusStale && s != StatusUnknown {
		t.Errorf("StatusForPID(terminated) = %q, want stale or unknown", s)
	}
}

func TestStatusForLiveChildHasNoSideEffects(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessSleep")
	cmd.Env = append(os.Environ(), "OCGO_TEST_HELPER_SLEEP=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	s := StatusForPID(pid)
	if runtime.GOOS == "windows" {
		if s != StatusUnknown {
			t.Fatalf("StatusForPID(live) on windows = %q, want %q", s, StatusUnknown)
		}
	} else {
		if s != StatusPresent {
			t.Fatalf("StatusForPID(live) = %q, want %q", s, StatusPresent)
		}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		t.Fatalf("child exited after StatusForPID: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHelperProcessExitImmediately(t *testing.T) {
	if os.Getenv("OCGO_TEST_HELPER_PROCESS_STATUS") != "1" {
		return
	}
	os.Exit(0)
}

func TestHelperProcessSleep(t *testing.T) {
	if os.Getenv("OCGO_TEST_HELPER_SLEEP") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}
