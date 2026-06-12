# Troubleshooting

## Codex Desktop does not respond

Check the daemon and Desktop state:

```bash
ocgo daemon status
ocgo doctor codex --mode desktop
```

Common causes:

- The daemon is not running. Start it with `ocgo daemon start`.
- Desktop is in ChatGPT mode. Switch to OpenCode: `ocgo codex desktop enable opencode`.
- The proxy `base_url` in `~/.codex/config.toml` points to the wrong host or port.
- The local proxy port is already in use.
- No backup was created before switching, so reverting to ChatGPT requires manual reconfiguration.

## Returning to ChatGPT / OpenAI

```bash
ocgo codex desktop enable chatgpt
```

If this fails because no backup is available:

- Do **not** re-run `enable opencode` to create a backup unless Codex Desktop is currently configured for ChatGPT/OpenAI. If Desktop is already in OpenCode mode, `enable opencode` would back up the OpenCode config instead of a real ChatGPT config.
- Restore `~/.codex/config.toml` manually from a known-good ChatGPT/OpenAI configuration, or reinstall Codex Desktop to regenerate the default config.

## Codex CLI does not see OpenCode Go models

```bash
ocgo launch codex --config
ocgo doctor codex --mode cli
```

Check that these files exist and are valid:

- `~/.codex/ocgo-models.json` — the model catalog
- `~/.codex/ocgo-launch.config.toml` — the profile file

If missing, run `ocgo launch codex --config`. If the profile exists but models are not showing up in Codex's model picker, verify that `model_provider = "ocgo-launch"` and `model_catalog_json` point to a valid path.

## Invalid model error

```bash
ocgo list
ocgo opencode model current
ocgo opencode model set-default minimax-m3
```

- `ocgo list` shows all available models.
- `ocgo opencode model current` shows which model will be used.
- If the model was manually changed in `~/.config/ocgo/model-selection.json` to an ID not in the catalog, set it again with `ocgo opencode model set-default <model>`.

## Proxy unreachable

```bash
ocgo daemon status
ocgo daemon restart
curl http://127.0.0.1:3456/health
```

- If `curl` returns anything other than `ok`, the proxy is not running or is misconfigured.
- Check `~/.config/ocgo/config.json` for the correct host and port.
- Check that nothing else is using the port (`netstat -an | findstr :3456` on Windows, `lsof -i :3456` on macOS/Linux).

## Token counting is inconsistent

The `/v1/messages/count_tokens` endpoint is a local, approximate, deterministic estimate. It uses rune-count heuristics and structural overhead constants. It is not byte-compatible with Anthropic's, OpenAI's, or any other proprietary tokenizer.

The estimate is intentionally conservative:

- Text is counted as `max(ceil(rune_count/4), word_count, 1)`
- Each message adds a structural overhead
- Each tool definition adds a structural overhead
- Each image adds an explicit overhead

This estimate is sufficient for clients that use token counts to decide whether to continue a conversation. It is not intended for billing or exact context-window accounting.

## Remote Codex (container / VM / cloud workspace)

`127.0.0.1` always refers to the machine where the command runs. If Codex CLI runs inside a container or remote workspace:

- `127.0.0.1:3456` points to the container/workspace, not your host.
- The local ocgo proxy must be reachable from that environment.
- Options:
  - Run ocgo inside the same container or workspace.
  - Expose the proxy on `0.0.0.0` and connect via the host machine's LAN IP.
  - Use an SSH tunnel or similar.

> **Warning:** Do not expose the proxy to the public internet. The proxy uses your OpenCode Go API key upstream.

For LAN access, edit `~/.config/ocgo/config.json`:

```json
{
  "api_key": "sk-opencode-...",
  "host": "0.0.0.0",
  "port": 3456
}
```

Then in the remote Codex profile, set `openai_base_url = "http://<HOST_IP>:3456/v1/"`.

## Support bundle

If you need to report an issue, generate a redacted support bundle:

```bash
ocgo support bundle
```

The bundle is saved to `~/.config/ocgo/support-bundles/` and can be safely
attached to bug reports. See [docs/support-bundle.md](support-bundle.md) for
details.

## Doctor diagnostics overview

```bash
ocgo doctor
ocgo doctor codex --mode cli
ocgo doctor codex --mode desktop
ocgo doctor codex --json
```

The doctor is read-only. It never starts the daemon, never modifies configuration, and never switches the Desktop provider. It checks:

- Config file and API key
- Model selection and catalog
- Daemon state and health endpoint
- Local proxy endpoints (`/v1/models`, `/v1/messages/count_tokens`, `/v1/responses`)
- Codex CLI binary, profile, and model catalog
- Codex Desktop config, state, and backup

Exit codes:

- `0` — all checks passed or only warnings exist
- `1` — at least one check failed with an error

## Reset OCGO safely

```bash
ocgo config inspect
ocgo config backup
ocgo config reset --scope ocgo --dry-run
ocgo config reset --scope ocgo --yes
```
