package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"ocgo/internal/doctor"
)

// ErrDoctorFailed is returned by runDoctor when the doctor
// report has one or more error checks. The cobra command's
// RunE returns it, causing the CLI to exit with a non-zero
// code (1) after writing the report. The report is written
// before the error is returned so the user sees the full
// output.
var ErrDoctorFailed = errors.New("doctor found errors in the configuration")

// DoctorCmd returns the cobra command for `ocgo doctor`.
//
// The doctor is read-only: it never modifies configuration,
// never starts the daemon, never writes backups, never
// switches the Codex Desktop provider. Its sole purpose is
// to give the user a stable view of why their setup works
// or does not work.
func DoctorCmd() *cobra.Command {
	var (
		mode string
		json bool
	)

	cmd := &cobra.Command{
		Use:           "doctor [codex]",
		Short:         "Diagnose OCGO, Codex CLI, and Codex Desktop setup",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: strings.TrimSpace(`
ocgo doctor runs a series of read-only checks against the
local OCGO installation, the Codex CLI, and Codex Desktop.

Usage:
  ocgo doctor
  ocgo doctor codex

Flags:
  --mode cli|desktop|all Select the scope of the run.
  --json                 Emit a machine-readable JSON report.

The doctor never modifies your configuration. If it reports
an error, follow the remediation hints to fix the issue.
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			if len(args) == 1 && args[0] == "codex" {
				return nil
			}
			if len(args) > 0 {
				return fmt.Errorf("unknown doctor scope %q (supported: codex)", strings.Join(args, " "))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd, args, mode, json)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "all", "Doctor mode: all, cli, or desktop")
	cmd.Flags().BoolVar(&json, "json", false, "Emit a JSON report")
	return cmd
}

// runDoctor executes the doctor with the given args. It is
// extracted from the cobra handler so tests can call it
// directly. The args[0] can be "codex" to scope the run to
// Codex (the only scope supported today).
func runDoctor(cmd *cobra.Command, args []string, modeFlag string, wantJSON bool) error {
	mode := doctor.Mode(modeFlag)
	if !mode.IsValid() {
		return fmt.Errorf("invalid --mode %q (want all, cli, or desktop)", modeFlag)
	}
	// Currently "ocgo doctor" and "ocgo doctor codex" are
	// equivalent; the codex subcommand is the only scope.
	_ = args
	rep := doctor.NewRunner().RunCodex(context.Background(), mode)

	var writeErr error
	if wantJSON {
		writeErr = writeDoctorJSON(cmd.OutOrStdout(), rep)
	} else {
		writeErr = writeDoctorText(cmd.OutOrStdout(), rep)
	}
	if writeErr != nil {
		return writeErr
	}

	if rep.Status == doctor.StatusError {
		return ErrDoctorFailed
	}
	return nil
}

// writeDoctorJSON serializes the report as compact JSON.
func writeDoctorJSON(w io.Writer, rep doctor.Report) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// writeDoctorText writes a human-readable, no-color report.
// The output is deterministic so it can be diffed in tests.
func writeDoctorText(w io.Writer, rep doctor.Report) error {
	if _, err := fmt.Fprintln(w, "OCGO Doctor"); err != nil {
		return err
	}
	groups := groupChecks(rep.Checks)
	// Sort group names for stable output.
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(w, "\n%s\n", name); err != nil {
			return err
		}
		for _, c := range groups[name] {
			if err := writeCheckLine(w, c); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "\nOverall: %s\n", rep.Status); err != nil {
		return err
	}
	return nil
}

func writeCheckLine(w io.Writer, c doctor.Check) error {
	marker, _ := statusMarker(c.Status)
	if _, err := fmt.Fprintf(w, "  %-7s  %-26s  %s\n", marker, c.Label, c.Message); err != nil {
		return err
	}
	if c.Remediation != "" {
		if _, err := fmt.Fprintf(w, "           %-26s  → %s\n", "", c.Remediation); err != nil {
			return err
		}
	}
	return nil
}

func statusMarker(s doctor.Status) (string, bool) {
	switch s {
	case doctor.StatusOK:
		return "OK", true
	case doctor.StatusWarning:
		return "WARNING", true
	case doctor.StatusError:
		return "ERROR", true
	case doctor.StatusSkipped:
		return "SKIP", true
	}
	return string(s), false
}

// groupChecks groups checks by Group, with checks that have
// no Group landing in "Checks". Stable order is preserved
// within each group.
func groupChecks(checks []doctor.Check) map[string][]doctor.Check {
	out := map[string][]doctor.Check{}
	for _, c := range checks {
		name := c.Group
		if name == "" {
			name = "Checks"
		}
		out[name] = append(out[name], c)
	}
	return out
}

// ExitCodeFor maps a doctor report status to the process
// exit code. StatusOK and StatusWarning both return 0; only
// StatusError returns 1. The CLI entry point uses this to
// set the cobra command's exit code.
func ExitCodeFor(rep doctor.Report) int {
	switch rep.Status {
	case doctor.StatusError:
		return 1
	case doctor.StatusWarning, doctor.StatusOK, doctor.StatusSkipped:
		return 0
	}
	return 0
}

