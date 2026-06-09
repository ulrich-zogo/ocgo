package codex

var (
	writeStateFunc = WriteDesktopState
)

func SetWriteStateFuncForTest(fn func(path string, state DesktopState) error) (restore func()) {
	prev := writeStateFunc
	if fn != nil {
		writeStateFunc = fn
	}
	return func() { writeStateFunc = prev }
}

// SetExecForTest replaces the package-level exec hooks used
// by checkCodexVersion. The restore closure puts back the
// real implementations. It is intended for tests that need
// to drive the codex version check without a real codex
// binary on PATH.
//
// The signatures mirror the unexported package vars so
// callers do not need to import os/exec directly.
func SetExecForTest(lookPath func(string) (string, error), commandOutput func(string, ...string) ([]byte, error)) (restore func()) {
	prevLook := execLookPath
	prevOut := execCommandOutput
	if lookPath != nil {
		execLookPath = lookPath
	}
	if commandOutput != nil {
		execCommandOutput = commandOutput
	}
	return func() {
		execLookPath = prevLook
		execCommandOutput = prevOut
	}
}
