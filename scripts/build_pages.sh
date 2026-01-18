#!/usr/bin/env bash
set -euo pipefail

COUNTRY="${COUNTRY:-kr}"
CHART="${CHART:-top-free}"
DB_PATH="${DB_PATH:-data/appstore.db}"
OUT_DIR="${OUT_DIR:-site}"
THEMES="${THEMES:-config/themes.json}"

mkdir -p "$OUT_DIR"
cp cmd/app_download_analyzer/index.html "$OUT_DIR/index.html"

go run ./cmd/app_download_analyzer report-json \
  --country "$COUNTRY" \
  --chart "$CHART" \
  --db "$DB_PATH" \
  --themes "$THEMES" \
  --out "$OUT_DIR/report.json"

go run ./cmd/app_download_analyzer timeseries-json \
  --country "$COUNTRY" \
  --chart "$CHART" \
  --db "$DB_PATH" \
  --themes "$THEMES" \
  --out "$OUT_DIR/timeseries.json"

cat <<EOF > "$OUT_DIR/.nojekyll"
EOF
