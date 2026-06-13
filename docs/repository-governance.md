# Repository governance

## Goals

This repository is maintained as the primary OCGO repository under `ulrich-zogo/ocgo`.

The governance goals are:

- keep `main` protected
- use pull requests for all changes
- prefer squash merges
- require CI before merge
- prevent accidental direct pushes
- prevent force-pushes and branch deletion
- remove historical fork presentation when possible
- keep repository metadata pointing to `ulrich-zogo/ocgo`

## Repository ownership

Canonical repository:

```text
ulrich-zogo/ocgo
```

Canonical Homebrew tap:

```text
ulrich-zogo/homebrew-tap
```

No repository files, scripts, docs, manifests, release tooling, or package metadata should point to the original upstream owner.

## Fork detachment

GitHub does not expose a normal repository-file based way to detach a repository from a fork network.

To remove the "forked from ..." label while preserving the current repository URL, issues, pull requests, releases, stars, and settings, request a fork-network detach from GitHub Support.

See:

```text
docs/github-support-unfork-request.md
```

## Branch protection

`main` must be protected.

Recommended settings:

- Require pull request before merging
- Require status checks before merging
- Require conversation resolution before merging
- Require linear history
- Block force pushes
- Block deletions
- Do not allow bypassing protections
- Require CODEOWNERS review when practical

Required checks should include only workflows that run automatically on PRs.

Recommended required checks:

- `test`
- `windows-install-smoke`

Do not require opt-in/manual tests such as the real daemon smoke test.

## Merge policy

Preferred merge method:

```text
Squash merge
```

Recommended repository setting:

- enable squash merge
- disable merge commits
- disable rebase merge
- automatically delete head branches

## Audit

Run:

```bash
scripts/audit-repo-governance.sh
```

## Apply recommended main protection

Run:

```bash
scripts/apply-main-branch-protection.sh
```

This requires GitHub CLI authentication with admin permissions on the repository.
