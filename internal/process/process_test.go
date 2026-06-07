package process

import (
	"strings"
	"testing"
)

func TestParseWindowsNetstatPID(t *testing.T) {
	output := strings.Join([]string{
		"Proto  Local Address          Foreign Address        State           PID",
		"TCP    127.0.0.1:3456       0.0.0.0:0              LISTENING       4321",
		"TCP    [::1]:9999           [::]:0                 LISTENING       8765",
		"TCP    127.0.0.1:34560      0.0.0.0:0              LISTENING       1111",
	}, "\n")
	pid, err := ParseWindowsNetstatPID(output, 3456)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 4321 {
		t.Fatalf("pid = %d, want 4321", pid)
	}
}

func TestParseWindowsNetstatPIDMatchesIPv6(t *testing.T) {
	output := "TCP    [::]:3456             [::]:0                 LISTENING       2468\n"
	pid, err := ParseWindowsNetstatPID(output, 3456)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 2468 {
		t.Fatalf("pid = %d, want 2468", pid)
	}
}
