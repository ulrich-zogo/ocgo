// Package doctor provides the OCGO read-only diagnostic engine.
//
// The doctor runs a series of checks against the local OCGO,
// Codex CLI, and Codex Desktop setup and produces a stable,
// machine-readable Report. The doctor never writes to the
// user's configuration: it does not start the daemon, does
// not modify Codex config files, does not write backups, does
// not switch the Desktop provider.
package doctor

import "strings"

// Status is the per-check severity. The overall report status
// is derived from the highest-severity check.
type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
	StatusSkipped Status = "skipped"
)

// A Check is a single diagnostic step.
type Check struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Group       string `json:"group,omitempty"`
	Status      Status `json:"status"`
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// Report is the top-level doctor output. Checks is always
// non-nil; the order is stable and matches the order in which
// the checks were appended.
type Report struct {
	Status Status  `json:"status"`
	Checks []Check `json:"checks"`
}

// NewReport returns a Report with status set from the worst
// check status. If no checks are present, status is StatusOK.
func NewReport(checks []Check) Report {
	out := Report{
		Status: StatusOK,
		Checks: checks,
	}
	for _, c := range checks {
		if c.Status == StatusError {
			out.Status = StatusError
			return out
		}
		if c.Status == StatusWarning {
			out.Status = StatusWarning
		}
	}
	return out
}

// Append returns a new Report with the given checks appended.
// The status is re-derived. The input Report is not modified.
func (r Report) Append(checks ...Check) Report {
	all := make([]Check, 0, len(r.Checks)+len(checks))
	all = append(all, r.Checks...)
	all = append(all, checks...)
	return NewReport(all)
}

// OK returns a Check with status StatusOK. ID and label are
// required. Details and remediation are optional.
func OK(id, label, message string) Check {
	return Check{ID: id, Label: label, Status: StatusOK, Message: message}
}

// Warning returns a Check with status StatusWarning.
func Warning(id, label, message, remediation string) Check {
	return Check{ID: id, Label: label, Status: StatusWarning, Message: message, Remediation: remediation}
}

// Error returns a Check with status StatusError.
func Error(id, label, message, remediation string) Check {
	return Check{ID: id, Label: label, Status: StatusError, Message: message, Remediation: remediation}
}

// Skipped returns a Check with status StatusSkipped.
func Skipped(id, label, message string) Check {
	return Check{ID: id, Label: label, Status: StatusSkipped, Message: message}
}

// WithDetails returns a copy of c with the given details.
func (c Check) WithDetails(details string) Check {
	c.Details = trimForReport(details)
	return c
}

// trimForReport collapses runs of whitespace so multi-line
// details render cleanly in both the text and JSON output.
func trimForReport(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Collapse internal \r\n to \n for cross-platform stability.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return s
}
