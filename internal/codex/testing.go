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
