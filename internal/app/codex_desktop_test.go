package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocgo/internal/codex"
)

func setupCodexDesktopTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	cfgPath := filepath.Join(dir, ".config", "ocgo", "config.json")
	if err := writeFile(cfgPath, `{"api_key":"test-key","host":"127.0.0.1","port":3456}`); err != nil {
		t.Fatal(err)
	}
	return dir
}

type codexDesktopStub = daemonStub

func TestCodexDesktopStatusNotManaged(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Codex Desktop is not managed by OCGO") {
		t.Fatalf("output missing 'not managed' message:\n%s", got)
	}
}

func TestCodexDesktopStatusOpenCodeWithDaemon(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	if err := codex.WriteDesktopState(stateFile, codex.DesktopState{
		Version:   codex.DesktopStateVersion,
		Mode:      codex.DesktopModeOpenCode,
		UpdatedAt: time.Now().UTC(),
		BaseURL:   "http://127.0.0.1:3456/v1/",
		Model:     "minimax-m3",
	}); err != nil {
		t.Fatal(err)
	}
	stub := &daemonStub{}
	stub.healthy.Store(true)
	stub.install(t)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Codex Desktop mode: opencode",
		"Base URL: http://127.0.0.1:3456/v1/",
		"Model: minimax-m3",
		"Daemon: running",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestCodexDesktopStatusOpenCodeWithDaemonDown(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	if err := codex.WriteDesktopState(stateFile, codex.DesktopState{
		Version:   codex.DesktopStateVersion,
		Mode:      codex.DesktopModeOpenCode,
		UpdatedAt: time.Now().UTC(),
		BaseURL:   "http://127.0.0.1:3456/v1/",
		Model:     "minimax-m3",
	}); err != nil {
		t.Fatal(err)
	}
	stub := &daemonStub{}
	stub.healthy.Store(false)
	stub.install(t)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Daemon: not running") {
		t.Fatalf("output missing 'Daemon: not running':\n%s", got)
	}
}

func TestCodexDesktopStatusChatGPT(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	if err := codex.WriteDesktopState(stateFile, codex.DesktopState{
		Version:   codex.DesktopStateVersion,
		Mode:      codex.DesktopModeChatGPT,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Codex Desktop mode: chatgpt") {
		t.Fatalf("output missing chatgpt mode:\n%s", got)
	}
}

func TestCodexDesktopEnableOpenCode(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	stub := &daemonStub{}
	stub.install(t)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "enable", "opencode"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Codex Desktop enabled with OCGO/OpenCode Go",
		"Base URL: http://127.0.0.1:3456/v1/",
		"Model: minimax-m3",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	st, err := codex.ReadDesktopState(stateFile)
	if err != nil {
		t.Fatalf("state should be readable: %v", err)
	}
	if st.Mode != codex.DesktopModeOpenCode {
		t.Errorf("state.Mode = %q, want opencode", st.Mode)
	}
	desktopConfig := filepath.Join(home, ".codex", "config.toml")
	if _, err := os.Stat(desktopConfig); err != nil {
		t.Fatalf("desktop config should exist: %v", err)
	}
}

func TestCodexDesktopEnableOpenCodeWithModel(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	stub := &daemonStub{}
	stub.install(t)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "enable", "opencode", "--model", "minimax-m2.7"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Model: minimax-m2.7") {
		t.Fatalf("output missing explicit model:\n%s", got)
	}
	st, err := codex.ReadDesktopState(stateFile)
	if err != nil {
		t.Fatalf("state should be readable: %v", err)
	}
	if st.Model != "minimax-m2.7" {
		t.Errorf("state.Model = %q, want minimax-m2.7", st.Model)
	}
}

func TestCodexDesktopEnableChatGPT(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	original := "model = \"gpt-5\"\nmodel_provider = \"openai\"\n"
	desktopConfig := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(desktopConfig), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktopConfig, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	stub := &daemonStub{}
	stub.install(t)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "enable", "opencode"})
	var out1 bytes.Buffer
	root.SetOut(&out1)
	root.SetErr(&out1)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	root2 := NewRootCommand("test")
	root2.SetArgs([]string{"codex", "desktop", "enable", "chatgpt"})
	var out2 bytes.Buffer
	root2.SetOut(&out2)
	root2.SetErr(&out2)
	if err := root2.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out2.String()
	if !strings.Contains(got, "Codex Desktop restored to ChatGPT/OpenAI configuration") {
		t.Fatalf("output missing chatgpt restore message:\n%s", got)
	}
	st, err := codex.ReadDesktopState(stateFile)
	if err != nil {
		t.Fatalf("state should be readable: %v", err)
	}
	if st.Mode != codex.DesktopModeChatGPT {
		t.Errorf("state.Mode = %q, want chatgpt", st.Mode)
	}
	got2, err := os.ReadFile(desktopConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(got2) != original {
		t.Errorf("restored content = %q, want %q", got2, original)
	}
}

func TestCodexDesktopEnableChatGPTWithoutBackupFails(t *testing.T) {
	home := setupCodexDesktopTestHome(t)
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	stub := &daemonStub{}
	stub.install(t)
	_ = home

	root := NewRootCommand("test")
	root.SetArgs([]string{"codex", "desktop", "enable", "opencode"})
	var out1 bytes.Buffer
	root.SetOut(&out1)
	root.SetErr(&out1)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if err := codex.WriteDesktopState(stateFile, codex.DesktopState{
		Version:   codex.DesktopStateVersion,
		Mode:      codex.DesktopModeOpenCode,
		UpdatedAt: time.Now().UTC(),
		BaseURL:   "http://127.0.0.1:3456/v1/",
		Model:     "minimax-m3",
	}); err != nil {
		t.Fatal(err)
	}

	root2 := NewRootCommand("test")
	root2.SetArgs([]string{"codex", "desktop", "enable", "chatgpt"})
	var out2 bytes.Buffer
	root2.SetOut(&out2)
	root2.SetErr(&out2)
	err := root2.Execute()
	if err == nil {
		t.Fatal("expected error for missing backup")
	}
	if !strings.Contains(err.Error(), "no backup") {
		t.Fatalf("error should mention 'no backup': %v", err)
	}
}

func TestRootRegistersCodexDesktop(t *testing.T) {
	root := NewRootCommand("test")
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "codex" {
			found = true
			for _, sub := range c.Commands() {
				if sub.Name() == "desktop" {
					return
				}
			}
			t.Fatalf("codex command found but missing 'desktop' subcommand")
		}
	}
	if !found {
		t.Fatal("root command should register 'codex'")
	}
}
