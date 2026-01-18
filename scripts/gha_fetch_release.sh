#!/usr/bin/env bash
set -euo pipefail

COUNTRY="${COUNTRY:-kr}"
CHART="${CHART:-top-free}"
LIMIT="${LIMIT:-25}"
DB_PATH="${DB_PATH:-data/appstore.db}"
NO_ITUNES="${NO_ITUNES:-false}"
TIMEOUT="${TIMEOUT:-20s}"
RELEASE_TAG="${RELEASE_TAG:-appstore-db}"

DB_DIR="$(dirname "$DB_PATH")"
DB_NAME="$(basename "$DB_PATH")"

mkdir -p "$DB_DIR"

if ! command -v gh >/dev/null 2>&1; then
  echo "gh CLI not found."
  exit 1
fi

if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
  gh release download "$RELEASE_TAG" --pattern "$DB_NAME" --dir "$DB_DIR" || true
fi

FETCH_CMD=(go run ./cmd/app_download_analyzer fetch --country "$COUNTRY" --chart "$CHART" --limit "$LIMIT" --db "$DB_PATH" --timeout "$TIMEOUT")
if [[ "$NO_ITUNES" == "true" ]]; then
  FETCH_CMD+=(--no-itunes)
fi

"${FETCH_CMD[@]}"

if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
  gh release upload "$RELEASE_TAG" "$DB_PATH" --clobber
else
  gh release create "$RELEASE_TAG" "$DB_PATH" --notes "Automated App Store snapshot storage"
fi
