# App Download Analyzer (App Store)

Collect App Store (Apple) top chart data for Korea and compute simple trend and regime indicators.

## Quick start

Fetch the top-free chart and store a snapshot:

```bash
go run ./cmd/app_download_analyzer fetch --country kr --chart top-free --limit 25 --db data/appstore.db
```

Run it again later to build history, then generate a report:

```bash
go run ./cmd/app_download_analyzer report --country kr --chart top-free --db data/appstore.db --top 10
```

Start a local web dashboard:

```bash
go run ./cmd/app_download_analyzer serve --country kr --chart top-free --db data/appstore.db --addr :8080
```

The server can auto-collect snapshots while running:

```bash
go run ./cmd/app_download_analyzer serve --country kr --chart top-free --db data/appstore.db --interval 6h --auto-fetch --fetch-on-start
```

Generate static JSON for charts (GitHub Pages):

```bash
go run ./cmd/app_download_analyzer report-json --country kr --chart top-free --db data/appstore.db --out report.json
go run ./cmd/app_download_analyzer timeseries-json --country kr --chart top-free --db data/appstore.db --out timeseries.json
```

## GitHub Actions automation

This repo includes a GitHub Actions workflow that collects snapshots on a schedule and stores the SQLite DB as a GitHub Release asset (tag: `appstore-db`).

- Workflow file: `.github/workflows/snapshot.yml`
- Interval: edit the `cron` schedule in the workflow (default: every 6 hours).
- Manual run: use `workflow_dispatch` inputs to override country/chart/limit.

The workflow uses `scripts/gha_fetch_release.sh` to download the latest DB from the release, append a new snapshot, and upload the updated DB.

## GitHub Pages dashboard

The workflow `.github/workflows/pages.yml` generates a static dashboard and deploys it to GitHub Pages.

- The dashboard consumes `report.json` and does not require a running server.
- Pages build runs after the snapshot workflow succeeds (or via manual run).
- Enable Pages in GitHub: Settings → Pages → Source: GitHub Actions.

## Notes

- The Apple Marketing Tools RSS endpoint provides chart rank, not download counts.
- Trend scores are based on rank velocity and review count growth from iTunes lookup.
- Edit `config/themes.json` to tailor themes or risk-on/off buckets.

## Charts

Supported charts: `top-free`, `top-paid`.
