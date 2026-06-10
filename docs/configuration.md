# Configuration

ocgo stores its configuration in `~/.config/ocgo/`. Codex-related files are stored in `~/.codex/`.

## File reference

| File | Purpose |
|---|---|
| `~/.config/ocgo/config.json` | API key, host, and port for the local proxy |
| `~/.config/ocgo/model-mapping.json` | Optional Claude/Codex model name routing |
| `~/.config/ocgo/model-selection.json` | Shared default OpenCode Go model |
| `~/.config/ocgo/model-catalog-cache.json` | Cached official OpenCode Go model list |
| `~/.config/ocgo/daemon-state.json` | Daemon process state |
| `~/.config/ocgo/codex-desktop-state.json` | Desktop provider switching state (opencode vs chatgpt) |
| `~/.config/ocgo/codex-backups/` | Backups of Codex Desktop config before switching to OpenCode |
| `~/.codex/ocgo-launch.config.toml` | Codex CLI profile written by `ocgo launch codex` |
| `~/.codex/ocgo-models.json` | Codex model catalog written by `ocgo launch codex` |
| `~/.codex/config.toml` | Codex Desktop active config (modified by `enable opencode` / `enable chatgpt`) |

Do not commit these files to version control.

## Config file format

`~/.config/ocgo/config.json`:

```json
{
  "api_key": "sk-opencode-your-key",
  "host": "127.0.0.1",
  "port": 3456
}
```

- `api_key` — your OpenCode Go API key
- `host` — proxy bind address (default `127.0.0.1`)
- `port` — proxy port (default `3456`)

## Repository ownership

This fork is documented and released under:

```
ulrich-zogo/ocgo
```

Release artifacts, Homebrew formulas, and all documentation should reference this fork. The original upstream is not referenced in active docs.

## Environment variable

The API key can also be set at runtime without a config file:

```bash
export OCGO_API_KEY=sk-opencode-your-key
```

When both the file and the env var are set, the env var takes precedence.
