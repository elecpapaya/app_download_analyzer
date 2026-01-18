package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"app_download_analyzer/internal/analysis"
	"app_download_analyzer/internal/store"
)

type timeSeriesMeta struct {
	Country string `json:"country"`
	Chart   string `json:"chart"`
	Limit   int    `json:"limit"`
}

type timeSeriesPayload struct {
	Meta          timeSeriesMeta       `json:"meta"`
	Dates         []string             `json:"dates"`
	RotationIndex []float64            `json:"rotation_index"`
	RiskOnScore   []float64            `json:"risk_on_score"`
	RiskOffScore  []float64            `json:"risk_off_score"`
	ThemeScores   map[string][]float64 `json:"theme_scores"`
	TopApps       []timeSeriesTopApp   `json:"top_apps"`
}

type timeSeriesTopApp struct {
	AppID        string `json:"app_id"`
	AppName      string `json:"app_name"`
	AppURL       string `json:"app_url"`
	Ranks        []*int `json:"ranks"`
	RatingCounts []*int `json:"rating_counts"`
}

func runTimeSeriesJSON(args []string) error {
	fs := flag.NewFlagSet("timeseries-json", flag.ExitOnError)
	country := fs.String("country", defaultCountry, "storefront country code")
	chart := fs.String("chart", defaultChart, "chart name (top-free, top-paid)")
	dbPath := fs.String("db", defaultDBPath, "sqlite db path")
	themePath := fs.String("themes", "config/themes.json", "theme rules json")
	outPath := fs.String("out", "timeseries.json", "output file path or '-' for stdout")
	topN := fs.Int("top", 10, "top N apps for rank history")
	rankWeight := fs.Float64("rank-weight", 1.0, "weight for rank delta z-score")
	reviewWeight := fs.Float64("review-weight", 1.0, "weight for review growth z-score")
	newEntryBonus := fs.Float64("new-bonus", 0.5, "bonus for new chart entries")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	cfg := analysis.TrendConfig{
		RankWeight:    *rankWeight,
		ReviewWeight:  *reviewWeight,
		NewEntryBonus: *newEntryBonus,
	}

	payload, err := computeTimeSeries(st, *country, *chart, *themePath, cfg, *topN)
	if err != nil {
		return err
	}

	return writeJSON(outPath, payload)
}

func computeTimeSeries(st *store.Store, country, chart, themePath string, cfg analysis.TrendConfig, topN int) (timeSeriesPayload, error) {
	snapshots, err := st.ListSnapshots(country, chart)
	if err != nil {
		return timeSeriesPayload{}, err
	}
	if len(snapshots) == 0 {
		return timeSeriesPayload{}, fmt.Errorf("no snapshots found")
	}

	themeConfig, err := analysis.LoadThemeConfig(themePath)
	if err != nil {
		return timeSeriesPayload{}, err
	}

	themeNames := uniqueThemes(themeConfig)
	themeScores := map[string][]float64{}
	for _, theme := range themeNames {
		themeScores[theme] = []float64{}
	}

	dates := make([]string, 0, len(snapshots))
	rotation := make([]float64, 0, len(snapshots))
	riskOn := make([]float64, 0, len(snapshots))
	riskOff := make([]float64, 0, len(snapshots))

	snapshotItems := make([][]store.ChartItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		items, err := st.GetSnapshotItems(snapshot.ID)
		if err != nil {
			return timeSeriesPayload{}, err
		}
		snapshotItems = append(snapshotItems, items)
	}

	snapshots, snapshotItems = groupSnapshotsByDate(snapshots, snapshotItems)

	for idx, snapshot := range snapshots {
		currentItems := snapshotItems[idx]
		prevSnapshot := snapshot
		prevItems := currentItems
		if idx > 0 {
			prevSnapshot = snapshots[idx-1]
			prevItems = snapshotItems[idx-1]
		}

		result := analysis.AnalyzeTrends(snapshot, prevSnapshot, currentItems, prevItems, cfg, themeConfig)

		dates = append(dates, snapshot.CollectedAt.UTC().Format(time.RFC3339))
		rotation = append(rotation, result.RotationIndex)
		riskOn = append(riskOn, result.RiskOnScore)
		riskOff = append(riskOff, result.RiskOffScore)

		for _, theme := range themeNames {
			themeScores[theme] = append(themeScores[theme], result.ThemeScores[theme])
		}
	}

	topApps := buildTopApps(snapshotItems, snapshots, topN)

	payload := timeSeriesPayload{
		Meta: timeSeriesMeta{
			Country: country,
			Chart:   chart,
			Limit:   snapshots[len(snapshots)-1].Limit,
		},
		Dates:         dates,
		RotationIndex: rotation,
		RiskOnScore:   riskOn,
		RiskOffScore:  riskOff,
		ThemeScores:   themeScores,
		TopApps:       topApps,
	}

	return payload, nil
}

func groupSnapshotsByDate(snapshots []store.Snapshot, items [][]store.ChartItem) ([]store.Snapshot, [][]store.ChartItem) {
	if len(snapshots) == 0 {
		return snapshots, items
	}
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		loc = time.UTC
	}

	dateIndex := make(map[string]int, len(snapshots))
	for i, snapshot := range snapshots {
		key := snapshot.CollectedAt.In(loc).Format("2006-01-02")
		dateIndex[key] = i
	}

	seen := make(map[string]bool, len(dateIndex))
	groupedSnapshots := make([]store.Snapshot, 0, len(dateIndex))
	groupedItems := make([][]store.ChartItem, 0, len(dateIndex))
	for i, snapshot := range snapshots {
		key := snapshot.CollectedAt.In(loc).Format("2006-01-02")
		if dateIndex[key] != i || seen[key] {
			continue
		}
		seen[key] = true
		groupedSnapshots = append(groupedSnapshots, snapshot)
		groupedItems = append(groupedItems, items[i])
	}

	return groupedSnapshots, groupedItems
}

func uniqueThemes(cfg analysis.ThemeConfig) []string {
	seen := map[string]bool{"other": true}
	var themes []string
	themes = append(themes, "other")
	for _, rule := range cfg.Rules {
		theme := rule.Theme
		if theme == "" {
			continue
		}
		if !seen[theme] {
			seen[theme] = true
			themes = append(themes, theme)
		}
	}
	sort.Strings(themes)
	return themes
}

func buildTopApps(snapshotItems [][]store.ChartItem, snapshots []store.Snapshot, topN int) []timeSeriesTopApp {
	if len(snapshotItems) == 0 {
		return nil
	}
	latestItems := snapshotItems[len(snapshotItems)-1]
	if topN > len(latestItems) {
		topN = len(latestItems)
	}

	topApps := make([]timeSeriesTopApp, 0, topN)
	for i := 0; i < topN; i++ {
		item := latestItems[i]
		topApps = append(topApps, timeSeriesTopApp{
			AppID:   item.AppID,
			AppName: item.AppName,
			AppURL:  item.AppURL,
		})
	}

	itemMaps := make([]map[string]store.ChartItem, 0, len(snapshotItems))
	for _, items := range snapshotItems {
		itemMap := make(map[string]store.ChartItem, len(items))
		for _, item := range items {
			itemMap[item.AppID] = item
		}
		itemMaps = append(itemMaps, itemMap)
	}

	for idx := range topApps {
		topApps[idx].Ranks = make([]*int, len(snapshots))
		topApps[idx].RatingCounts = make([]*int, len(snapshots))
		for snapIdx, itemMap := range itemMaps {
			item, ok := itemMap[topApps[idx].AppID]
			if !ok {
				continue
			}
			rank := item.Rank
			topApps[idx].Ranks[snapIdx] = &rank
			if item.RatingCount.Valid {
				count := item.RatingCount.Value
				topApps[idx].RatingCounts[snapIdx] = &count
			}
		}
	}
	return topApps
}

func writeJSON(path *string, payload any) error {
	var out *os.File
	if *path == "-" {
		out = os.Stdout
	} else {
		if err := ensureDirForFile(*path); err != nil {
			return err
		}
		file, err := os.Create(*path)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func ensureDirForFile(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
