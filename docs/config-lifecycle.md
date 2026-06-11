# Configuration lifecycle

## Show paths

```bash
ocgo config paths
ocgo config paths --json
```

## Inspect configuration

```bash
ocgo config inspect
ocgo config inspect --json
```

Secrets are redacted.

## Backup

```bash
ocgo config backup
ocgo config backup --output ./ocgo-backup.zip
ocgo config backup --include-codex-config
```

## Restore

```bash
ocgo config restore ./ocgo-backup.zip --dry-run
ocgo config restore ./ocgo-backup.zip --yes
```

## Reset

```bash
ocgo config reset --dry-run
ocgo config reset --scope ocgo --yes
ocgo config reset --scope codex-cli --yes
ocgo config reset --scope codex-desktop --yes
ocgo config reset --scope cache --yes
ocgo config reset --scope all --yes
```

## Safety

* Destructive operations require confirmation or `--yes`.
* Backups are created before reset and restore.
* Secrets are never printed.
* Codex non-OCGO profiles are preserved.
