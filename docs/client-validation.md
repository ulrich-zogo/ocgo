# Client validation

OCGO supports three client paths:

1. Claude Code CLI
2. Codex CLI
3. Codex Desktop

## Codex CLI

Run:

```bash
ocgo launch codex --config
ocgo doctor codex --mode cli
```

Expected:

- profile `ocgo-launch` exists;
- provider points to local OCGO proxy;
- `wire_api = "responses"`;
- selected model is valid.

## Claude Code

Run:

```bash
ocgo launch claude --model minimax-m3 -- --help
```

Expected:

- Claude receives the local Anthropic-compatible endpoint;
- model is valid;
- additional args are preserved.

## Codex Desktop

Run:

```bash
ocgo daemon start
ocgo codex desktop enable opencode --model minimax-m3
ocgo doctor codex --mode desktop
ocgo codex desktop enable chatgpt
```

Expected:

- OpenCode mode is enabled;
- backup exists;
- restore to ChatGPT/OpenAI is possible;
- daemon health is valid.

### Restore validation

After enabling OpenCode mode, always validate restore:

```bash
ocgo codex desktop enable chatgpt
ocgo codex desktop status
```

Expected:

- mode is `chatgpt` or equivalent;
- the previous Desktop config is restored;
- the backup remains available or is referenced in OCGO state;
- `ocgo doctor codex --mode desktop` does not report a corrupted managed state.

Do not edit the Codex Desktop config manually while OCGO is managing it unless you first restore ChatGPT mode or take a backup.

## Troubleshooting

Use:

```bash
ocgo doctor
ocgo doctor codex
ocgo doctor codex --mode cli
ocgo doctor codex --mode desktop
ocgo doctor codex --json
```
