package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/daemon"
	"ocgo/internal/models"
)

// Mode selects the doctor scope. Unknown values are rejected
// at the command layer.
type Mode string

const (
	ModeAll     Mode = "all"
	ModeCLI     Mode = "cli"
	ModeDesktop Mode = "desktop"
)

// IsValid reports whether m is one of the supported modes.
func (m Mode) IsValid() bool {
	switch m {
	case ModeAll, ModeCLI, ModeDesktop:
		return true
	}
	return false
}

// Paths groups the file-system locations the doctor inspects.
// Tests override these via NewRunnerWithPaths to point at temp
// directories; production code uses the default constructor
// which reads the standard ocgo paths.
type Paths struct {
	ConfigDir          string
	ConfigFile         string
	ModelSelectionFile string
	CatalogCacheFile   string
	DaemonStateFile    string
	CodexConfigFile    string
	CodexProfileFile   string
	CodexCatalogFile   string
	DesktopStateFile   string
	BackupDir          string
}

// DefaultPaths returns the production paths derived from the
// current process HOME. It is safe to call from tests as long
// as HOME is set to a temp directory first.
func DefaultPaths() Paths {
	return Paths{
		ConfigDir:          config.ConfigDir(),
		ConfigFile:         config.ConfigFile(),
		ModelSelectionFile: config.ModelSelectionFile(),
		CatalogCacheFile:   config.ModelCatalogCacheFile(),
		DaemonStateFile:    daemon.DaemonStateFile(),
		CodexConfigFile:    config.CodexConfigFile(),
		CodexProfileFile:   config.CodexProfileConfigFile(),
		CodexCatalogFile:   config.CodexModelCatalogFile(),
		DesktopStateFile:   codex.DesktopStateFile(),
		BackupDir:          config.CodexBackupDir(),
	}
}

// Runner executes a doctor pass. The zero value is invalid;
// use NewRunner or NewRunnerWithPaths.
//
// All dependencies (paths, HTTP client, file access) are
// injected so tests can replace them with in-memory variants.
// Production code uses NewRunner which wires the real
// implementations.
type Runner struct {
	Paths Paths

	// HTTPClient is the http.Client used to hit the local
	// proxy endpoints (/health, /v1/models, ...). If nil, a
	// default client with a short timeout is used.
	HTTPClient *http.Client

	// HTTPTimeout is the per-request timeout for the local
	// proxy. Default 2s.
	HTTPTimeout time.Duration

	// CodeVersion is the build version of the running ocgo
	// binary. Included in core checks for clarity.
	CodeVersion string

	// HostPort is the (host, port) used to talk to the local
	// daemon. If unset, the doctor derives it from the OCGO
	// config file (or the package defaults).
	HostPort func() (host string, port int, err error)
}

// NewRunner returns a Runner with production paths and an
// HTTP client pointing at the OCGO config's host/port.
func NewRunner() Runner {
	return NewRunnerWithPaths(DefaultPaths())
}

// NewRunnerWithPaths returns a Runner that uses the given
// paths. The HTTP client has a short timeout suitable for
// the local proxy; tests can replace Runner.HTTPClient.
func NewRunnerWithPaths(p Paths) Runner {
	return Runner{
		Paths:       p,
		HTTPTimeout: 2 * time.Second,
		HostPort:    hostPortFromConfigFile(p.ConfigFile),
	}
}

// hostPortFromConfigFile returns a HostPort closure that
// resolves the OCGO config file at the given path. If the
// file is missing or invalid, it falls back to package
// defaults (127.0.0.1:3456). The closure does not error on
// missing config; it just returns defaults.
func hostPortFromConfigFile(path string) func() (string, int, error) {
	return func() (string, int, error) {
		host := config.DefaultHost
		port := config.DefaultPort
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return host, port, nil
			}
			return host, port, nil
		}
		var c config.Config
		if err := json.Unmarshal(b, &c); err != nil {
			return host, port, nil
		}
		if c.Host != "" {
			host = c.Host
		}
		if c.Port > 0 {
			port = c.Port
		}
		return host, port, nil
	}
}

// client returns the HTTP client to use, lazily wiring the
// default with the runner's timeout.
func (r Runner) client() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	timeout := r.HTTPTimeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// baseURL returns the "http://host:port" base URL for the
// local proxy. If HostPort fails, it falls back to the
// package default.
func (r Runner) baseURL() string {
	host, port, _ := r.resolveHostPort()
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func (r Runner) resolveHostPort() (string, int, error) {
	if r.HostPort == nil {
		return config.DefaultHost, config.DefaultPort, nil
	}
	return r.HostPort()
}

// RunCodex executes the doctor for the requested Mode. The
// returned Report has a stable order: core checks, then mode
// checks. The doctor never writes to disk; if a file does
// not exist the corresponding check is reported as warning
// (or skipped, depending on the spec).
func (r Runner) RunCodex(ctx context.Context, mode Mode) Report {
	if !mode.IsValid() {
		return NewReport([]Check{
			Error("core.mode", "Doctor mode", "invalid mode: "+string(mode), ""),
		})
	}

	core := r.coreChecks(ctx)
	checks := make([]Check, 0, 50)
	checks = append(checks, core.Checks...)

	switch mode {
	case ModeAll:
		checks = append(checks, r.daemonChecks(ctx).Checks...)
		checks = append(checks, r.codexCLIChecks()...)
		checks = append(checks, r.codexDesktopChecks()...)
	case ModeCLI:
		// CLI mode does not need the proxy: it talks to
		// the local OCGO CLI shim through the codex-launch
		// profile, so daemon/proxy checks and desktop checks
		// are skipped entirely.
		checks = append(checks, r.codexCLIChecks()...)
	case ModeDesktop:
		checks = append(checks, r.daemonChecks(ctx).Checks...)
		checks = append(checks, r.codexDesktopChecks()...)
	}
	return NewReport(checks)
}

// coreChecks returns the configuration/model/catalog checks
// that apply regardless of mode.
func (r Runner) coreChecks(ctx context.Context) Report {
	return Report{
		Checks: []Check{
			r.checkOCGOConfig(),
			r.checkEffectiveModel(),
			r.checkModelCatalog(),
		},
	}
}

// daemonChecks returns the daemon/proxy local-endpoint checks.
// The status depends on whether Desktop is currently in
// OpenCode mode (which mandates the proxy is up). This
// function is called only by RunCodex for ModeDesktop and
// ModeAll; ModeCLI callers skip daemon checks entirely.
func (r Runner) daemonChecks(ctx context.Context) Report {
	checks := []Check{
		r.checkDaemonStateFile(),
		r.checkHealthEndpoint(ctx),
	}
	checks = append(checks, r.localEndpointChecks(ctx)...)
	// If Desktop is in OpenCode mode, the proxy MUST be up.
	desktopMode := readDesktopModeBestEffort(r.Paths.DesktopStateFile)
	if desktopMode == codex.DesktopModeOpenCode {
		health := checks[1]
		if health.Status != StatusOK {
			checks = append(checks, Error(
				"daemon.required_for_desktop",
				"Proxy required for Desktop",
				"Codex Desktop is in OpenCode mode but the local proxy is not healthy: "+health.Message,
				"Run: ocgo daemon start",
			))
		}
	}
	return Report{Checks: checks}
}

// checkOCGOConfig inspects ~/.config/ocgo/config.json.
func (r Runner) checkOCGOConfig() Check {
	id := "core.config"
	path := r.Paths.ConfigFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "OCGO config",
				"config not found at "+path,
				"Run: ocgo setup")
		}
		return Error(id, "OCGO config",
			"failed to read "+path+": "+err.Error(),
			"Run: ocgo setup")
	}
	var c config.Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Error(id, "OCGO config",
			"invalid JSON in "+path+": "+err.Error(),
			"Fix the JSON in "+path+" or run: ocgo setup")
	}
	if c.APIKey == "" {
		return Error(id, "OCGO config",
			"api_key is empty in "+path,
			"Set api_key in "+path+" or run: ocgo setup")
	}
	return OK(id, "OCGO config", path+" loaded")
}

// checkEffectiveModel verifies that the model the user has
// configured (or the fallback) is a known model.
//
// Behavior:
//   - If a model-selection file exists and parses, the
//     configured model must be a known catalog entry;
//     otherwise this check is an error.
//   - If no selection file exists, or it is empty, the
//     doctor reports a warning and uses the first known
//     model as the implicit fallback.
func (r Runner) checkEffectiveModel() Check {
	id := "core.model"
	sel, selErr := readModelSelection(r.Paths.ModelSelectionFile)
	if selErr == nil {
		// A selection file is present. The configured
		// model must be valid.
		if sel.DefaultModel == "" {
			return Error(id, "Effective model",
				"model-selection.json has no default_model",
				"Run: ocgo opencode model set-default <model>")
		}
		if !models.IsKnown(sel.DefaultModel) {
			return Error(id, "Effective model",
				"configured model is not in the catalog: "+sel.DefaultModel,
				"Run: ocgo opencode model set-default <model>")
		}
		return OK(id, "Effective model", sel.DefaultModel)
	}
	// No selection file (or unreadable). Fall back to the
	// first known model and report a warning so the user
	// knows nothing was configured.
	known := models.KnownIDs()
	if len(known) == 0 {
		return Error(id, "Effective model",
			"no known models available",
			"Run: ocgo models to refresh the catalog")
	}
	return Warning(id, "Effective model",
		"no model selected explicitly, falling back to "+known[0],
		"Run: ocgo opencode model set-default <model>")
}

// checkModelCatalog inspects the model catalog and cache.
func (r Runner) checkModelCatalog() Check {
	id := "core.catalog"
	ids := models.KnownIDs()
	if len(ids) == 0 {
		return Error(id, "Model catalog",
			"no models available",
			"Run: ocgo models to refresh the catalog")
	}
	// If the cache file is absent but the static fallback has
	// models, that is a warning (not an error) so the doctor
	// remains useful in offline scenarios.
	cachePath := r.Paths.CatalogCacheFile
	if _, err := os.Stat(cachePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Model catalog",
				fmt.Sprintf("cache missing (%s), using fallback: %d models", cachePath, len(ids)),
				"Run: ocgo models to populate the cache")
		}
		return Warning(id, "Model catalog",
			"cache unreadable: "+err.Error(),
			"")
	}
	return OK(id, "Model catalog", fmt.Sprintf("%d models available", len(ids)))
}

// checkDaemonStateFile verifies the daemon state file
// exists, is parseable, and is the expected version.
func (r Runner) checkDaemonStateFile() Check {
	id := "daemon.state"
	path := r.Paths.DaemonStateFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Daemon state",
				"state file not found: "+path,
				"")
		}
		return Warning(id, "Daemon state",
			"failed to read state file: "+err.Error(),
			"")
	}
	var st daemon.State
	if err := json.Unmarshal(b, &st); err != nil {
		return Error(id, "Daemon state",
			"invalid state JSON: "+err.Error(),
			"Run: ocgo daemon restart")
	}
	if st.Version != daemon.StateVersion {
		return Error(id, "Daemon state",
			fmt.Sprintf("unsupported state version %d (expected %d)", st.Version, daemon.StateVersion),
			"Run: ocgo daemon restart")
	}
	return OK(id, "Daemon state", path)
}

// checkHealthEndpoint hits GET /health on the local proxy.
func (r Runner) checkHealthEndpoint(ctx context.Context) Check {
	id := "daemon.health"
	base := r.baseURL()
	url := base + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Error(id, "Health endpoint",
			"build request: "+err.Error(),
			"Run: ocgo daemon start")
	}
	resp, err := r.client().Do(req)
	if err != nil {
		// Distinguish "no server" from "server but bad".
		return Error(id, "Health endpoint",
			"GET "+url+" failed: "+err.Error(),
			"Run: ocgo daemon start")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !bytes.Equal(bytes.TrimSpace(body), []byte("ok")) {
		// State present but server not actually healthy is
		// a warning (state is stale), unless Desktop relies
		// on the proxy (in which case daemonChecks adds an
		// error).
		return Warning(id, "Health endpoint",
			fmt.Sprintf("GET %s returned %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body))),
			"Run: ocgo daemon restart")
	}
	return OK(id, "Health endpoint", url+" responded ok")
}

// localEndpointChecks runs the read-only probe checks that
// verify the proxy is wired up. They never reach the
// upstream OpenCode Go API.
func (r Runner) localEndpointChecks(ctx context.Context) []Check {
	out := []Check{
		r.checkModelsEndpoint(ctx),
		r.checkCountTokensEndpoint(ctx),
		r.checkResponsesValidation(ctx),
	}
	return out
}

// checkModelsEndpoint hits GET /v1/models.
func (r Runner) checkModelsEndpoint(ctx context.Context) Check {
	id := "proxy.models"
	url := r.baseURL() + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Error(id, "Models endpoint",
			"build request: "+err.Error(),
			"Run: ocgo daemon start")
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return Error(id, "Models endpoint",
			"GET "+url+" failed: "+err.Error(),
			"Run: ocgo daemon start")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Error(id, "Models endpoint",
			fmt.Sprintf("GET %s returned %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body))),
			"Run: ocgo daemon restart")
	}
	var parsed struct {
		Object string           `json:"object"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Error(id, "Models endpoint",
			"GET "+url+" returned unparseable JSON: "+err.Error(),
			"")
	}
	if parsed.Object != "list" {
		return Error(id, "Models endpoint",
			fmt.Sprintf("object = %q, want %q", parsed.Object, "list"),
			"")
	}
	if len(parsed.Data) == 0 {
		return Warning(id, "Models endpoint",
			"GET "+url+" returned 0 models",
			"Run: ocgo models to refresh the catalog")
	}
	return OK(id, "Models endpoint",
		fmt.Sprintf("/v1/models returned %d models", len(parsed.Data)))
}

// checkCountTokensEndpoint posts a minimal Anthropic-style
// payload to /v1/messages/count_tokens and verifies the
// returned input_tokens is non-zero. The doctor never makes
// a real upstream call: this endpoint is local.
func (r Runner) checkCountTokensEndpoint(ctx context.Context) Check {
	id := "proxy.count_tokens"
	url := r.baseURL() + "/v1/messages/count_tokens"
	body := []byte(`{
		"model":"minimax-m3",
		"messages":[{"role":"user","content":"ping"}]
	}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Error(id, "Token counting",
			"build request: "+err.Error(),
			"Run: ocgo daemon restart")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client().Do(req)
	if err != nil {
		return Error(id, "Token counting",
			"POST "+url+" failed: "+err.Error(),
			"Run: ocgo daemon start")
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Error(id, "Token counting",
			fmt.Sprintf("POST %s returned %d: %s", url, resp.StatusCode, strings.TrimSpace(string(rb))),
			"Run: ocgo daemon restart")
	}
	var parsed struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return Error(id, "Token counting",
			"unexpected response: "+err.Error(),
			"")
	}
	if parsed.InputTokens <= 0 {
		return Error(id, "Token counting",
			fmt.Sprintf("input_tokens = %d, want > 0", parsed.InputTokens),
			"Run: ocgo daemon restart")
	}
	return OK(id, "Token counting",
		fmt.Sprintf("/v1/messages/count_tokens returned input_tokens=%d", parsed.InputTokens))
}

// checkResponsesValidation posts an obviously-invalid body
// to /v1/responses and verifies the proxy returns 400
// without ever contacting the upstream. This proves the
// route is wired and validation works.
func (r Runner) checkResponsesValidation(ctx context.Context) Check {
	id := "proxy.responses"
	url := r.baseURL() + "/v1/responses"
	body := []byte(`{}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Error(id, "Responses validation",
			"build request: "+err.Error(),
			"Run: ocgo daemon restart")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client().Do(req)
	if err != nil {
		return Error(id, "Responses validation",
			"POST "+url+" failed: "+err.Error(),
			"Run: ocgo daemon start")
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	// We expect a 4xx: the local validation must reject an
	// empty body. Anything other than a 4xx means the route
	// is broken or the request leaked upstream.
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		return Error(id, "Responses validation",
			fmt.Sprintf("POST %s returned %d, want 4xx: %s", url, resp.StatusCode, strings.TrimSpace(string(rb))),
			"Run: ocgo daemon restart")
	}
	return OK(id, "Responses validation",
		fmt.Sprintf("/v1/responses returned %d (local validation active)", resp.StatusCode))
}

// ---------- helpers ----------

// readDesktopModeBestEffort returns the desktop mode stored
// in the state file, or empty string if the file is missing
// or unreadable. Errors are intentionally swallowed: the
// caller only uses the result to decide whether the proxy
// is required.
func readDesktopModeBestEffort(path string) codex.DesktopMode {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s codex.DesktopState
	if err := json.Unmarshal(b, &s); err != nil {
		return ""
	}
	return s.Mode
}
