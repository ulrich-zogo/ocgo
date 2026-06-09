package codex

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const minCodexVersion = "0.81.0"

var (
	execLookPath      = exec.LookPath
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

func (m Manager) CheckVersion() error {
	return checkCodexVersion()
}

func checkCodexVersion() error {
	if _, err := execLookPath("codex"); err != nil {
		return fmt.Errorf("codex is not installed, install with: npm install -g @openai/codex")
	}
	out, err := execCommandOutput("codex", "--version")
	if err != nil {
		return fmt.Errorf("failed to get codex version: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return fmt.Errorf("unexpected codex version output: %s", string(out))
	}
	version := fields[len(fields)-1]
	if CompareVersions(version, minCodexVersion) < 0 {
		return fmt.Errorf("codex version %s is too old, minimum required is %s; update with: npm update -g @openai/codex", version, minCodexVersion)
	}
	return nil
}

func CompareVersions(a, b string) int {
	ap, bp := VersionParts(a), VersionParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func VersionParts(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	fields := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < len(fields) && i < 3; i++ {
		part := fields[i]
		for j, r := range part {
			if r < '0' || r > '9' {
				part = part[:j]
				break
			}
		}
		out[i], _ = strconv.Atoi(part)
	}
	return out
}
