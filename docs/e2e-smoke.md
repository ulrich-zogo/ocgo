# E2E smoke tests

## Purpose

The E2E smoke tests validate real OCGO user workflows at the CLI/app level
using isolated `HOME`/`USERPROFILE` directories and daemon runtime stubs
where needed.

They do not require:

- Real Codex CLI installation
- Real Claude CLI installation
- Real OpenCode Go API keys
- Upstream network access

## What is covered

- Fresh config diagnostics (`version --json`, `config paths --json`,
  `config inspect --json`, `doctor --json`, `support bundle --json`)
- Model selection (`current`, `set-default`, invalid model rejection)
- Daemon lifecycle (`start`, `status --json`, `restart`, `stop`, double stop)
- Stale PID and stale lock cleanup
- Codex CLI config generation (`launch codex --config`)
- Codex Desktop switching (`enable opencode`, `enable chatgpt`, restore)
- Support bundle after full workflow (secret redaction in zip)
- Config lifecycle (`backup`, `reset --dry-run`, `inspect --json`)

## What is not covered

- Real proxy HTTP endpoints (`/v1/chat/completions`, `/v1/messages`)
- Upstream OpenCode Go API calls
- Real Codex CLI or Claude CLI sessions
- Release artifact verification
- Package manager installation (Scoop, WinGet, Homebrew)

## Running locally

```bash
go test ./internal/e2e -run E2E -v
```

On Windows:

```powershell
go test ./internal/e2e -run E2E -v
```

## Skipped tests

Tests that start the daemon (even with stubs) are skipped when `-short` is
used:

```bash
go test ./internal/e2e -run E2E -short -v
```

## Real daemon process smoke test

The app-level E2E tests above use daemon runtime stubs. To test a real
background daemon process with local HTTP endpoints:

```bash
OCGO_E2E_REAL_DAEMON=1 go test ./internal/e2e -run RealDaemon -v
```

This test builds the OCGO binary from source and starts the actual daemon.
It is opt-in to avoid port conflicts and process management in the default
suite. See [real-daemon-smoke.md](real-daemon-smoke.md).

## Safety

All tests run with temporary `HOME`/`USERPROFILE` directories created via
`t.TempDir()`. They never touch the real `~/.config/ocgo` or `~/.codex`.
