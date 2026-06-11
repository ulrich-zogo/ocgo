package buildinfo

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestCurrentIncludesDefaults(t *testing.T) {
	info := Current()
	if info.Version != "dev" {
		t.Errorf("Version = %q, want %q", info.Version, "dev")
	}
	if info.Commit != "unknown" {
		t.Errorf("Commit = %q, want %q", info.Commit, "unknown")
	}
	if info.Date != "unknown" {
		t.Errorf("Date = %q, want %q", info.Date, "unknown")
	}
}

func TestCurrentIncludesRuntimeFields(t *testing.T) {
	info := Current()
	if info.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
}

func TestJSONMarshal(t *testing.T) {
	info := Current()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Info
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != info.Version {
		t.Errorf("Version roundtrip: got %q, want %q", decoded.Version, info.Version)
	}
	if decoded.OS == "" {
		t.Error("OS is empty in JSON")
	}
	if decoded.Arch == "" {
		t.Error("Arch is empty in JSON")
	}
}

func TestJSONHasStableFields(t *testing.T) {
	info := Current()
	data, _ := json.Marshal(info)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	expected := []string{"version", "commit", "date", "go_version", "os", "arch"}
	for _, key := range expected {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON missing field: %s", key)
		}
	}
}
