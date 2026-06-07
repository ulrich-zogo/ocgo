package main

import (
	"strings"
	"testing"

	"ocgo/internal/app"
)

func TestRootCommand(t *testing.T) {
	root := app.NewRootCommand("test")
	if root.Use != "ocgo" {
		t.Fatalf("root use = %q, want ocgo", root.Use)
	}
	subs := root.Commands()
	expected := []string{"setup", "list", "mapping", "launch", "serve", "stop", "status"}
	names := make([]string, len(subs))
	for i, c := range subs {
		names[i] = c.Name()
	}
	for _, want := range expected {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected subcommand %q not found in: %v", want, names)
		}
	}
	help := root.HelpTemplate() + root.UsageString()
	if !strings.Contains(help, "ocgo") {
		t.Fatal("help output should contain app name")
	}
}

func TestVersionFlag(t *testing.T) {
	root := app.NewRootCommand("1.0.0-test")
	if root.Version != "1.0.0-test" {
		t.Fatalf("version = %q, want 1.0.0-test", root.Version)
	}
}
