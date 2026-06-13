# Branch protection

## Purpose

`main` must be protected so all changes go through pull requests and checks.

## Recommended settings

Branch pattern:

```text
main
```

Enable:

- Require a pull request before merging
- Require approvals
- Require review from Code Owners
- Require status checks to pass before merging
- Require branches to be up to date before merging, if the workflow stays stable
- Require conversation resolution before merging
- Require linear history
- Do not allow bypassing the above settings
- Block force pushes
- Block deletions

These settings apply to administrators as well (`enforce_admins: true`).

## Required checks

Recommended:

```text
test
windows-install-smoke
```

Do not require:

```text
real-daemon-smoke
release-install-smoke-build
manual release smoke tests
```

because they are opt-in or platform-specific.

## Merge strategy

Recommended repository settings:

- Allow squash merging: enabled
- Allow merge commits: disabled
- Allow rebase merging: disabled
- Automatically delete head branches: enabled

## Apply via script

```bash
scripts/apply-main-branch-protection.sh
```

Options:

- `--repo ulrich-zogo/ocgo` — target repository
- `--dry-run` — print payload without mutation
- `--yes` — skip confirmation prompt
- `--required-check <name>` — add a required check (repeatable, default: `test,windows-install-smoke`)

## Audit

```bash
scripts/audit-repo-governance.sh
```
