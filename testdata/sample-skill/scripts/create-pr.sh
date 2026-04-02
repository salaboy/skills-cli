#!/bin/bash
# Create a pull request using forgejo-cli
set -euo pipefail

REPO="${1:?Usage: create-pr.sh <repo> <title> <body> [base] [head]}"
TITLE="${2:?Missing title}"
BODY="${3:?Missing body}"
BASE="${4:-main}"
HEAD="${5:-$(git branch --show-current)}"

forgejo-cli pr create \
  --repo "$REPO" \
  --title "$TITLE" \
  --body "$BODY" \
  --base "$BASE" \
  --head "$HEAD"
