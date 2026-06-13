#!/usr/bin/env bash
set -euo pipefail

REPO=""
OUTPUT_JSON=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --json) OUTPUT_JSON=true; shift ;;
    *) echo "Usage: $0 [--repo owner/repo] [--json]"; exit 1 ;;
  esac
done

if [[ -z "$REPO" ]]; then
  # Detect from git remote.
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

if ! gh auth status >/dev/null 2>&1; then
  echo "ERROR: gh is not authenticated. Run: gh auth login" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required for JSON processing" >&2
  echo "Install: https://jqlang.github.io/jq/download/" >&2
  exit 1
fi

extract_json() {
  local data="$1"
  local query="$2"
  echo "$data" | jq -r "$query" 2>/dev/null || echo ""
}

echo_or_json() {
  local key="$1"
  local value="$2"
  local fmt="${3:-%s}"
  if $OUTPUT_JSON; then
    return
  fi
  printf "  ${key}: ${fmt}\n" "$value"
}

echo ""
if $OUTPUT_JSON; then
  echo "{"
  echo "  \"repository\": \"$REPO\","
else
  echo "Repository governance audit: $REPO"
  echo ""
fi

# Fetch repository info.
REPO_DATA=""
if ! REPO_DATA=$(gh api "repos/${REPO}" 2>/dev/null); then
  if $OUTPUT_JSON; then
    echo "  \"error\": \"failed to fetch repository info\""
    echo "}"
  else
    echo "ERROR: failed to fetch repository info" >&2
  fi
  exit 1
fi

DEFAULT_BRANCH=$(extract_json "$REPO_DATA" '.default_branch // "main"')
FORK=$(extract_json "$REPO_DATA" '.fork // false')
PARENT=$(extract_json "$REPO_DATA" '.parent.full_name // "none"')
SOURCE=$(extract_json "$REPO_DATA" '.source.full_name // "none"')
VISIBILITY=$(extract_json "$REPO_DATA" '.visibility // "public"')
SQUASH_MERGE=$(extract_json "$REPO_DATA" '.allow_squash_merge // false')
MERGE_COMMIT=$(extract_json "$REPO_DATA" '.allow_merge_commit // false')
REBASE_MERGE=$(extract_json "$REPO_DATA" '.allow_rebase_merge // false')
DELETE_BRANCH=$(extract_json "$REPO_DATA" '.delete_branch_on_merge // false')

if $OUTPUT_JSON; then
  echo "  \"default_branch\": \"$DEFAULT_BRANCH\","
  echo "  \"fork\": $FORK,"
  echo "  \"parent\": \"$PARENT\","
  echo "  \"source\": \"$SOURCE\","
  echo "  \"visibility\": \"$VISIBILITY\","
  echo "  \"allow_squash_merge\": $SQUASH_MERGE,"
  echo "  \"allow_merge_commit\": $MERGE_COMMIT,"
  echo "  \"allow_rebase_merge\": $REBASE_MERGE,"
  echo "  \"delete_branch_on_merge\": $DELETE_BRANCH,"
else
  echo "Repository:"
  echo_or_json "full_name" "$REPO"
  echo_or_json "default_branch" "$DEFAULT_BRANCH"
  echo_or_json "visibility" "$VISIBILITY"
  echo_or_json "fork" "$FORK"
  echo_or_json "parent" "$PARENT"
  echo_or_json "source" "$SOURCE"
  echo_or_json "allow_squash_merge" "$SQUASH_MERGE"
  echo_or_json "allow_merge_commit" "$MERGE_COMMIT"
  echo_or_json "allow_rebase_merge" "$REBASE_MERGE"
  echo_or_json "delete_branch_on_merge" "$DELETE_BRANCH"
fi

# Fetch branch protection info.
MAIN_PROTECTED=false
PR_REVIEWS=false
REQUIRED_CHECKS="none"
ENFORCE_ADMINS=false
ALLOW_FORCE_PUSHES=false
ALLOW_DELETIONS=false
HAS_LINEAR_HISTORY=false

MAIN_PROTECTED=false
PROTECTION_DATA=""
set +e
PROTECTION_DATA=$(gh api "repos/${REPO}/branches/${DEFAULT_BRANCH}/protection" 2>/dev/null)
PROTECTION_EXIT=$?
set -e
if [[ $PROTECTION_EXIT -eq 0 ]] && [[ -n "$PROTECTION_DATA" ]]; then
  MAIN_PROTECTED=true
  PR_REVIEWS=$(extract_json "$PROTECTION_DATA" '.required_pull_request_reviews | length > 0')
  ENFORCE_ADMINS=$(extract_json "$PROTECTION_DATA" '.enforce_admins.enabled // false')
  ALLOW_FORCE_PUSHES=$(extract_json "$PROTECTION_DATA" '.allow_force_pushes.enabled // false')
  ALLOW_DELETIONS=$(extract_json "$PROTECTION_DATA" '.allow_deletions.enabled // false')
  HAS_LINEAR_HISTORY=$(extract_json "$PROTECTION_DATA" '.required_linear_history.enabled // false')
  REQUIRED_CHECKS=$(extract_json "$PROTECTION_DATA" '
    if (.required_status_checks.contexts // [] | length) > 0 then
      (.required_status_checks.contexts | join(","))
    elif (.required_status_checks.checks // [] | length) > 0 then
      (.required_status_checks.checks | map(.context) | join(","))
    else
      "none"
    end
  ')
  if [[ -z "$REQUIRED_CHECKS" ]]; then
    REQUIRED_CHECKS="none"
  fi
fi

if $OUTPUT_JSON; then
  echo "  \"main_protected\": $MAIN_PROTECTED,"
  echo "  \"require_pull_request_reviews\": $PR_REVIEWS,"
  echo "  \"required_status_checks\": \"$REQUIRED_CHECKS\","
  echo "  \"enforce_admins\": $ENFORCE_ADMINS,"
  echo "  \"allow_force_pushes\": $ALLOW_FORCE_PUSHES,"
  echo "  \"allow_deletions\": $ALLOW_DELETIONS,"
  echo "  \"required_linear_history\": $HAS_LINEAR_HISTORY,"
else
  echo ""
  echo "Branch protection ${DEFAULT_BRANCH}:"
  echo_or_json "protected" "$MAIN_PROTECTED"
  if $MAIN_PROTECTED; then
    echo_or_json "require_pull_request_reviews" "$PR_REVIEWS"
    echo_or_json "required_status_checks" "$REQUIRED_CHECKS"
    echo_or_json "enforce_admins" "$ENFORCE_ADMINS"
    echo_or_json "allow_force_pushes" "$ALLOW_FORCE_PUSHES"
    echo_or_json "allow_deletions" "$ALLOW_DELETIONS"
    echo_or_json "required_linear_history" "$HAS_LINEAR_HISTORY"
  fi
fi

# Generate recommendations.
RECOMMENDATIONS=()
if [[ "$FORK" == "true" ]]; then
  RECOMMENDATIONS+=("Request fork detachment from GitHub Support (see docs/github-support-unfork-request.md)")
fi
if ! $MAIN_PROTECTED; then
  RECOMMENDATIONS+=("Enable main branch protection (run scripts/apply-main-branch-protection.sh)")
fi
if [[ "$MERGE_COMMIT" == "true" ]]; then
  RECOMMENDATIONS+=("Disable merge commits (prefer squash merges)")
fi
if [[ "$REBASE_MERGE" == "true" ]]; then
  RECOMMENDATIONS+=("Disable rebase merges or switch to squash-only")
fi
if [[ "$DELETE_BRANCH" == "false" ]]; then
  RECOMMENDATIONS+=("Enable auto-delete head branches on merge")
fi

if $OUTPUT_JSON; then
  echo "  \"recommendations\": ["
  for i in "${!RECOMMENDATIONS[@]}"; do
    comma=""
    if [[ $i -lt $((${#RECOMMENDATIONS[@]} - 1)) ]]; then comma=","; fi
    echo "    \"${RECOMMENDATIONS[$i]}\"$comma"
  done
  echo "  ]"
  echo "}"
else
  echo ""
  echo "Recommendations:"
  if [[ ${#RECOMMENDATIONS[@]} -eq 0 ]]; then
    echo "  (none)"
  else
    for r in "${RECOMMENDATIONS[@]}"; do
      echo "  - $r"
    done
  fi
fi
