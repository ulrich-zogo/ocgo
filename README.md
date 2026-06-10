<h1 align="center">ocgo</h1>
<div align="center">
  <a href="https://github.com/ulrich-zogo/ocgo/releases">
    <img alt="GitHub release" src="https://img.shields.io/github/v/release/emanuelcasco/ocgo?color=blue">
  </a>
  <a href="https://github.com/ulrich-zogo/ocgo/releases">
    <img alt="GitHub downloads" src="https://img.shields.io/github/downloads/emanuelcasco/ocgo/total">
  </a>
  <a href="https://github.com/ulrich-zogo/ocgo/blob/main/LICENSE">
    <img alt="GitHub license" src="https://img.shields.io/github/license/emanuelcasco/ocgo">
  </a>
  <a href="https://go.dev/doc/go1.22">
    <img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/emanuelcasco/ocgo">
  </a>
</div>

<br/>

<div align="center">
 <img alt="Logo" src="assets/ocgo-logo.svg">
</div>

<br/>

<div align="center">
  <a href="https://github.com/ulrich-zogo/ocgo">ocgo</a> starts a local Anthropic/OpenAI-compatible proxy that lets Claude Code, Codex CLI, and Codex Desktop use an OpenCode Go subscription.
</div>

## What ocgo does

ocgo starts a local proxy that translates requests from your AI coding tools into OpenCode Go API calls.

- **Claude Code** uses the Anthropic Messages API proxy (`/v1/messages`).
- **Codex CLI** uses the OpenAI Chat Completions and Responses API proxy (`/v1/chat/completions`, `/v1/responses`).
- **Codex Desktop** uses the same proxy through a configurable provider.
- Your real OpenCode Go API key stays in the local config. Clients receive a fake key like `ocgo` or `unused`.

## Supported clients

- Claude Code (Anthropic Messages API)
- Codex CLI (OpenAI Chat Completions / Responses API)
- Codex Desktop (OpenAI Responses API via `ocgo-desktop` provider)

## Requirements

- Go 1.22 or newer (to build from source)
- A valid OpenCode Go API key
- Claude Code, Codex CLI, or Codex Desktop installed (depending on your workflow)

## Quick start

```bash
# 1. Save your API key
ocgo setup

# 2. Pick a default OpenCode Go model
ocgo list
ocgo opencode model set-default minimax-m3

# 3a. Launch Claude Code
ocgo launch claude --model minimax-m3

# 3b. Or launch Codex CLI
ocgo launch codex --model minimax-m3

# 3c. Or set up Codex Desktop (needs the daemon)
ocgo daemon start
ocgo codex desktop enable opencode --model minimax-m3

# 4. Diagnose any issues
ocgo doctor codex
```

## Installation

### Homebrew

```bash
brew install emanuelcasco/tap/ocgo
```

Or tap first:

```bash
brew tap emanuelcasco/tap
brew install ocgo
```

### Build from source

```bash
git clone https://github.com/ulrich-zogo/ocgo.git
cd ocgo
make build       # builds bin/ocgo
make install     # installs to ~/go/bin
```

## Setup

```bash
ocgo setup
```

Paste your OpenCode Go API key when prompted, or pass it directly:

```bash
ocgo setup --api-key sk-opencode-your-key
```

The API key can also be provided at runtime via the `OCGO_API_KEY` environment variable.

Configuration is saved to `~/.config/ocgo/config.json`. See [docs/configuration.md](docs/configuration.md) for details.

## Models

### List available models

```bash
ocgo list
ocgo ls
ocgo models
```

All three commands are equivalent. The output shows the 18+ models available through OpenCode Go.

### Set a default model

```bash
ocgo opencode model set-default minimax-m3
ocgo opencode model current
```

The default model is shared across `launch claude`, `launch codex`, and `codex desktop enable opencode`. When no default is configured, the first model in the catalog is used as a fallback.

A `--model` flag on any launch command overrides the default for that session.

### Model catalog

ocgo fetches the official OpenCode Go model list on first use and caches it locally in `~/.config/ocgo/model-catalog-cache.json`. If the official source is unreachable, ocgo uses the cache, then falls back to a built-in list of known models.

### Model mapping

Optional model name routing lets you map a tool-specific name to a different OpenCode Go model:

```bash
ocgo mapping claude set claude-sonnet kimi-k2.6
ocgo mapping codex set gpt-5.5 deepseek-v4-pro
ocgo mapping claude show
ocgo mapping codex show
```

Mappings are stored in `~/.config/ocgo/model-mapping.json`. See the [README from earlier versions](#) for full mapping command details.

## Claude Code

### Launch

```bash
ocgo launch claude
ocgo launch claude --model minimax-m3
ocgo launch claude --yes
ocgo launch claude --model minimax-m3 -- -p "Explain this repository"
```

### Environment variables

When starting Claude Code, ocgo sets:

```bash
ANTHROPIC_BASE_URL=http://127.0.0.1:3456
ANTHROPIC_AUTH_TOKEN=unused
```

With `--model`:

```bash
ANTHROPIC_MODEL=minimax-m3
ANTHROPIC_SMALL_FAST_MODEL=minimax-m3
```

Requests for Claude/Anthropic model names are routed through any configured mapping. Unmapped names pass through unchanged.

## Codex CLI

### Launch

```bash
ocgo launch codex
ocgo launch codex --model minimax-m3
ocgo launch codex -- --sandbox workspace-write
```

### Configure without launching

```bash
ocgo launch codex --config
```

This writes the `ocgo-launch` profile and model catalog without starting Codex.

### What gets written

`~/.codex/ocgo-launch.config.toml`:

```toml
openai_base_url = "http://127.0.0.1:3456/v1/"
forced_login_method = "api"
model_provider = "ocgo-launch"
model_catalog_json = "/Users/you/.codex/ocgo-models.json"

[model_providers.ocgo-launch]
name = "OpenCode Go"
base_url = "http://127.0.0.1:3456/v1/"
wire_api = "responses"
```

`~/.codex/ocgo-models.json` contains metadata for every known OpenCode Go model. Codex receives `OPENAI_API_KEY=ocgo` so all requests route through the local proxy.

### Run from terminal

After configuration:

```bash
codex --profile ocgo-launch -m minimax-m3
```

## Codex Desktop

Desktop mode lets Codex Desktop (the macOS/Windows GUI app) use OpenCode Go through the local proxy. Unlike Codex CLI, Desktop does not launch ocgo itself — the daemon must be running.

### Prerequisites

```bash
ocgo setup
ocgo opencode model set-default minimax-m3
ocgo daemon start
```

### Enable OpenCode mode

```bash
ocgo codex desktop enable opencode
ocgo codex desktop enable opencode --model minimax-m3
```

This:

1. Backs up the existing `~/.codex/config.toml` to `~/.config/ocgo/codex-backups/`.
2. Writes an OCGO Desktop provider with `model_provider = "ocgo-desktop"` and `wire_api = "responses"`.
3. Records the OpenCode state in `~/.config/ocgo/codex-desktop-state.json`.

### Check status

```bash
ocgo codex desktop status
ocgo doctor codex --mode desktop
```

### Return to ChatGPT / OpenAI

```bash
ocgo codex desktop enable chatgpt
```

This restores the previous `~/.codex/config.toml` from the backup created by `enable opencode`. The backup is preserved so you can switch back and forth.

### Full Desktop workflow

```bash
# Initial setup
ocgo setup
ocgo opencode model set-default minimax-m3
ocgo daemon start
ocgo codex desktop enable opencode
ocgo doctor codex --mode desktop

# Later, to revert
ocgo codex desktop enable chatgpt
```

## Daemon

ocgo provides two proxy-running mechanisms:

| Command | Use case |
|---|---|
| `ocgo serve` | Foreground proxy (previous behavior) |
| `ocgo serve --background` | Background proxy (legacy) |
| `ocgo daemon start` | Background daemon (recommended for Desktop) |
| `ocgo daemon status` | Daemon health check |
| `ocgo daemon stop` | Stop the daemon |
| `ocgo daemon restart` | Restart the daemon |

The daemon is the recommended mode for Codex Desktop because Desktop needs the proxy to stay up across sessions. `ocgo daemon status` checks both the `/health` endpoint and the daemon state file.

### Daemon files

```
~/.config/ocgo/daemon-state.json
~/.config/ocgo/ocgo.pid
~/.config/ocgo/ocgo.log
```

## Doctor

The doctor is a read-only diagnostic tool:

```bash
ocgo doctor
ocgo doctor codex
ocgo doctor codex --mode cli
ocgo doctor codex --mode desktop
ocgo doctor codex --json
```

It checks:

- OCGO configuration and API key
- Model selection and catalog
- Daemon state and health endpoint
- Local proxy endpoints (`/v1/models`, `/v1/messages/count_tokens`, `/v1/responses`)
- Codex CLI binary, profile, and model catalog
- Codex Desktop config, state, and backup

The doctor never modifies files, never starts the daemon, and never switches the Desktop provider.

Exit codes: `0` = ok or warning; `1` = at least one error.

## Remote Codex

ocgo's proxy listens on `127.0.0.1:3456` by default. When Codex CLI runs on the same machine, this works as-is.

If Codex CLI runs in a container, VM, or cloud workspace, `127.0.0.1` refers to the remote environment, not your local machine. Options:

- Run ocgo inside the same remote environment.
- Change the proxy to listen on `0.0.0.0` and connect via LAN IP.
- Use an SSH tunnel or VPN to forward the port.

> **Warning:** Do not expose the proxy to the public internet. The proxy uses your OpenCode Go API key and has no authentication of its own.

For LAN access, set `~/.config/ocgo/config.json`:

```json
{
  "api_key": "sk-opencode-...",
  "host": "0.0.0.0",
  "port": 3456
}
```

Then in the remote Codex profile, set `openai_base_url = "http://<HOST_IP>:3456/v1/"`.

## Configuration files

See [docs/configuration.md](docs/configuration.md) for a complete reference of configuration files, their locations, and formats.

## Proxy API

The local proxy exposes these endpoints:

| Endpoint | Method | Used by | Notes |
|---|---|---|---|
| `/health` | GET | Monitoring, doctor | Returns `ok` |
| `/v1/messages` | POST | Claude Code | Anthropic Messages → OpenCode Go → Anthropic-shaped response |
| `/v1/messages/count_tokens` | POST | All clients | Local approximate token count, no upstream call |
| `/v1/chat/completions` | POST | Codex CLI | OpenAI-compatible passthrough with API key injection |
| `/v1/responses` | POST | Codex CLI, Desktop | OpenAI Responses API adapter (chat completion bridge) |
| `/v1/models` | GET | All clients | Local model list from catalog/cache/fallback |

- `/v1/messages/count_tokens` is computed locally using character-based heuristics. It is deterministic and non-zero for any meaningful input, but it is not byte-compatible with any proprietary tokenizer.
- `/v1/responses` validates requests locally (empty bodies return 4xx) before forwarding valid requests upstream.
- `/v1/models` uses the official OpenCode Go catalog when available, falls back to the cached version, then falls back to a built-in list.

## Troubleshooting

See [docs/troubleshooting.md](docs/troubleshooting.md) for detailed troubleshooting of:

- Codex Desktop not responding
- Returning to ChatGPT / OpenAI
- Codex CLI not seeing models
- Invalid model errors
- Proxy unreachable
- Remote Codex (container / VM / cloud workspace)
- Token counting inconsistencies
- Doctor diagnostics

## Development

### Build

```bash
make build       # bin/ocgo
make install     # ~/go/bin/ocgo
make test
make run
make clean
```

### Prerequisites

- Go 1.22+
- A valid OpenCode Go API key for testing

```bash
git clone https://github.com/ulrich-zogo/ocgo.git
cd ocgo
go mod download
make build
bin/ocgo setup
```

### Release

```bash
make release TAG=v0.2.0
```

Requires `gh` (GitHub CLI) for creating releases and updating the Homebrew formula.

## Upgrading from earlier versions

```bash
# Refresh the model catalog
ocgo list

# Choose a default shared model
ocgo opencode model set-default minimax-m3

# Regenerate Codex CLI profile
ocgo launch codex --config

# Restart the daemon
ocgo daemon restart

# Re-enable Desktop OpenCode mode
ocgo codex desktop enable opencode

# Verify everything
ocgo doctor codex
```

Changes to note:

- If you previously used `serve --background`, you can keep it for CLI workflows, but use `daemon start` for Codex Desktop.
- If you hardcoded `kimi-k2.6` as a model, switch to the shared default model selection.
- Model mappings remain optional.

## Limitations

- Token counting is local and approximate. It does not attempt to reproduce any proprietary tokenizer.
- The `/v1/responses` adapter targets the text and tool-call workflows used by Codex. It is not a full OpenAI Responses API implementation.
- Codex Desktop requires the local daemon to be actively running.
- Tool calls and streaming are supported in the workflows covered by Claude Code and Codex CLI, but not every edge case of every upstream API is tested.

## License

MIT. See [LICENSE](LICENSE).
