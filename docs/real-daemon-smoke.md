# Real daemon smoke test

## Purpose

This test launches a real OCGO daemon process and verifies local HTTP endpoints.

## Why it is opt-in

The test starts a background process and opens a local TCP port. It is
intentionally not part of the default `go test ./...` run.

## Run on Linux/macOS

```bash
OCGO_E2E_REAL_DAEMON=1 go test ./internal/e2e -run RealDaemon -v
```

## Run on Windows PowerShell

```powershell
$env:OCGO_E2E_REAL_DAEMON = "1"
go test ./internal/e2e -run RealDaemon -v
Remove-Item Env:\OCGO_E2E_REAL_DAEMON
```

## What it verifies

- binary builds from source
- daemon starts and creates PID/state files
- `/health` responds 200
- `/v1/models` returns 18 official models
- `/v1/messages/count_tokens` returns a local token estimate
- double start does not spawn a second daemon
- restart works and health recovers
- stop works and health goes down
- double stop does not panic

## Safety

The test runs with a temporary `HOME`/`USERPROFILE` and a built binary in
a temp directory. It never touches the real `~/.config/ocgo` or `~/.codex`.
