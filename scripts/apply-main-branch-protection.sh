#!/usr/bin/env bash
set -euo pipefail

REPO=""
DRY_RUN=false
YES=false
REQUIRED_CHECKS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    --yes) YES=true; shift ;;
    --required-check) REQUIRED_CHECKS+=("$2"); shift 2 ;;
    *) echo "Usage: $0 [--repo owner/repo] [--dry-run] [--yes] [--required-check <name>]"; exit 1 ;;
  esac
done

if [[ ${#REQUIRED_CHECKS[@]} -eq 0 ]]; then
  REQUIRED_CHECKS=("test" "windows-install-smoke")
fi

if [[ -z "$REPO" ]]; then
  origin_url="$(git config --get remote.origin.url 2>/dev/null || true)"
  if [[ -z "$origin_url" ]]; then
    echo "ERROR: --repo required or a git remote 'origin' must exist" >&2
    exit 1
  fi
  REPO="$(echo "$origin_url" | sed 's|^git@github.com:|https://github.com/|' | sed 's|\.git$||' | sed 's|https://github.com/||')"
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: gh (GitHub CLI) is required" >&2
  echo "Install: https://cli.github.com/" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required for JSON processing" >&2
  echo "Install: https://jqlang.github.io/jq/download/" >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "ERROR: gh is not authenticated. Run: gh auth login" >&2
  exit 1
fi

# Check admin permission via REST API.
IS_ADMIN=$(gh api "repos/${REPO}" --jq '.permissions.admin // false' 2>/dev/null || echo "false")
if [[ "$IS_ADMIN" != "true" ]]; then
  echo "ERROR: admin permission required on ${REPO}" >&2
  echo "  permissions.admin: ${IS_ADMIN}" >&2
  exit 1
fi

BRANCH="main"

CONTEXTS_JSON="["
for i in "${!REQUIRED_CHECKS[@]}"; do
  if [[ $i -gt 0 ]]; then CONTEXTS_JSON+=", "; fi
  CONTEXTS_JSON+="\"${REQUIRED_CHECKS[$i]}\""
done
CONTEXTS_JSON+="]"

PROTECTION_PAYLOAD=$(cat <<ENDJSON
{
  "required_status_checks": {
    "strict": false,
    "contexts": $CONTEXTS_JSON
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "require_code_owner_reviews": true,
    "dismiss_stale_reviews": true,
    "require_last_push_approval": false
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "allow_fork_syncing": true
}
ENDJSON
)

REPO_SETTINGS_PAYLOAD=$(cat <<ENDJSON
{
  "allow_squash_merge": true,
  "allow_merge_commit": false,
  "allow_rebase_merge": false,
  "delete_branch_on_merge": true,
  "allow_auto_merge": false
}
ENDJSON
)

echo "Target repository: ${REPO}"
echo "Branch: ${BRANCH}"
echo "Required checks: ${REQUIRED_CHECKS[*]}"
echo ""

if $DRY_RUN; then
  echo "--- DRY RUN (no mutations) ---"
  echo ""
  echo "Branch protection payload (PUT repos/${REPO}/branches/${BRANCH}/protection):"
  echo "$PROTECTION_PAYLOAD" | jq .
  echo ""
  echo "Repository settings payload (PATCH repos/${REPO}):"
  echo "$REPO_SETTINGS_PAYLOAD" | jq .
  echo ""
  echo "Dry run complete. No changes made."
  exit 0
fi

echo "WARNING: This will update repository settings and protect ${BRANCH} on ${REPO}."
echo ""
echo "Changes:"
echo "  - Enable branch protection with PR reviews required"
echo "  - Require CODEOWNERS review"
echo "  - Require status checks: ${REQUIRED_CHECKS[*]}"
echo "  - Require linear history"
echo "  - Block force pushes and deletions"
echo "  - Apply protection to administrators (enforce_admins: true)"
echo "  - Disable merge commits and rebase merges"
echo "  - Enable squash merges only"
echo "  - Enable auto-delete head branches"
echo ""

if ! $YES; then
  echo -n "Continue? [y/N] "
  read -r CONFIRM
  if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo "Aborted."
    exit 1
  fi
fi

echo ""
echo "Applying branch protection ..."
set +e
PROTECTION_RESPONSE=$(gh api "repos/${REPO}/branches/${BRANCH}/protection" -X PUT -H "Accept: application/vnd.github+json" --input - <<<"$PROTECTION_PAYLOAD" 2>&1)
PROTECTION_EXIT=$?
set -e
if [[ $PROTECTION_EXIT -eq 0 ]]; then
  echo "  Branch protection applied."
else
  echo "  ERROR: failed to apply branch protection:" >&2
  echo "  ${PROTECTION_RESPONSE}" >&2
  exit 1
fi

echo ""
echo "Applying repository settings ..."
set +e
SETTINGS_RESPONSE=$(gh api "repos/${REPO}" -X PATCH -H "Accept: application/vnd.github+json" --input - <<<"$REPO_SETTINGS_PAYLOAD" 2>&1)
SETTINGS_EXIT=$?
set -e
if [[ $SETTINGS_EXIT -eq 0 ]]; then
  echo "  Repository settings applied."
else
  echo "  WARNING: failed to apply repository settings:" >&2
  echo "  ${SETTINGS_RESPONSE}" >&2
fi

echo ""
echo "Main branch protection applied to ${REPO}."
echo "Verify with: scripts/audit-repo-governance.sh --repo ${REPO}"
