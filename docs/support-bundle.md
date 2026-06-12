# Support bundle

## Purpose

The `ocgo support bundle` command generates a redacted, portable, ZIP archive
of diagnostic information about your OCGO installation. It is designed to be
safe to attach to bug reports.

## What is included

The bundle contains:

- **manifest.json** — metadata about the bundle itself
- **version.json** — OCGO version, build commit, Go version, OS/arch
- **doctor.json** — output of `ocgo doctor --json`
- **daemon-status.json** — current daemon status (state, pid, health)
- **config-paths.json** — all config file paths
- **config-inspect.json** — config inspection from `ocgo config inspect`
- **environment.json** — limited OS/Go environment metadata
- **logs/ocgo.log** — daemon log file (redacted, truncated at 1 MB)
- **state/** — existing OCGO state files (daemon state, model selection,
  model mapping, codex desktop state, codex profile, codex models)

Missing files are noted in the manifest as `"status": "missing"` and do not
block the bundle generation.

## What is never included

- Full environment variables
- Shell history
- Arbitrary user files
- Unredacted API keys or tokens
- User conversation content
- Prompts or model responses

## Redaction

The bundle is always redacted by default. The following patterns are removed
from all included files:

- `api_key`, `apikey`, `access_token`, `refresh_token`, `token`, `secret`,
  `password` values in JSON and text
- `Authorization` header values
- `x-api-key` header values
- `Bearer <token>` patterns
- OpenAI-style keys (`sk-...`)
- Environment variable values for `ANTHROPIC_AUTH_TOKEN`,
  `OPENCODE_GO_API_KEY`, `OPENAI_API_KEY`

Redacted values are replaced with `[REDACTED]`.

## Logs

Log files are included by default. To exclude them:

```bash
ocgo support bundle --no-logs
```

Log files larger than 1 MB are truncated to the last 1 MB. The manifest
notes truncation.

## Output

By default, the bundle is written to
`~/.config/ocgo/support-bundles/ocgo-support-bundle-<timestamp>.zip`.

Custom output path:

```bash
ocgo support bundle --output ./my-bundle.zip
```

The command refuses to overwrite an existing file unless `--force` is given:

```bash
ocgo support bundle --output ./my-bundle.zip --force
```

## JSON output

```bash
ocgo support bundle --json
```

Returns a single JSON object with no surrounding text.

## Troubleshooting

If the bundle fails to create, check that your config directory exists and
is readable. The command does not need the daemon to be running.

## Sharing safely

The bundle is designed to be safe to share. However, always review the
contents before attaching to a public issue:

```bash
unzip -l ocgo-support-bundle-*.zip
unzip -p ocgo-support-bundle-*.zip manifest.json
```

On Windows:

```powershell
Expand-Archive ocgo-support-bundle-*.zip -DestinationPath .\support-review
Get-Content .\support-review\manifest.json
```
