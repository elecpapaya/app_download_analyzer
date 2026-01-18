package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"app_download_analyzer/internal/analysis"
	"app_download_analyzer/internal/store"
)

type reportSnapshot struct {
	ID          int64     `json:"id"`
	CollectedAt time.Time `json:"collected_at"`
	Country     string    `json:"country"`
	Chart       string    `json:"chart"`
	Limit       int       `json:"limit"`
	SourceURL   string    `json:"source_url"`
}

type reportPayload struct {
	Latest        reportSnapshot        `json:"latest"`
	Previous      reportSnapshot        `json:"previous"`
	GeneratedAt   time.Time             `json:"generated_at"`
	Trends        []analysis.AppTrend   `json:"trends"`
	ThemeScores   []analysis.ThemeScore `json:"theme_scores"`
	RiskOnScore   float64               `json:"risk_on_score"`
	RiskOffScore  float64               `json:"risk_off_score"`
	RotationIndex float64               `json:"rotation_index"`
}

func computeReport(st *store.Store, country, chart, themePath string, cfg analysis.TrendConfig) (reportPayload, error) {
	latest, err := st.GetLatestSnapshot(country, chart)
	if err != nil {
		return reportPayload{}, err
	}
	previous, err := st.GetPreviousSnapshot(country, chart, latest.CollectedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return reportPayload{}, fmt.Errorf("need at least two snapshots for report")
		}
		return reportPayload{}, err
	}

	latestItems, err := st.GetSnapshotItems(latest.ID)
	if err != nil {
		return reportPayload{}, err
	}
	prevItems, err := st.GetSnapshotItems(previous.ID)
	if err != nil {
		return reportPayload{}, err
	}

	themeConfig, err := analysis.LoadThemeConfig(themePath)
	if err != nil {
		return reportPayload{}, err
	}

	result := analysis.AnalyzeTrends(latest, previous, latestItems, prevItems, cfg, themeConfig)

	payload := reportPayload{
		Latest: reportSnapshot{
			ID:          latest.ID,
			CollectedAt: latest.CollectedAt,
			Country:     latest.Country,
			Chart:       latest.Chart,
			Limit:       latest.Limit,
			SourceURL:   latest.SourceURL,
		},
		Previous: reportSnapshot{
			ID:          previous.ID,
			CollectedAt: previous.CollectedAt,
			Country:     previous.Country,
			Chart:       previous.Chart,
			Limit:       previous.Limit,
			SourceURL:   previous.SourceURL,
		},
		GeneratedAt:   time.Now().UTC(),
		Trends:        result.Trends,
		ThemeScores:   analysis.SortThemeScores(result.ThemeScores),
		RiskOnScore:   result.RiskOnScore,
		RiskOffScore:  result.RiskOffScore,
		RotationIndex: result.RotationIndex,
	}
	return payload, nil
}
