package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"app_download_analyzer/internal/analysis"
	"app_download_analyzer/internal/store"
)

func runReportJSON(args []string) error {
	fs := flag.NewFlagSet("report-json", flag.ExitOnError)
	country := fs.String("country", defaultCountry, "storefront country code")
	chart := fs.String("chart", defaultChart, "chart name (top-free, top-paid)")
	dbPath := fs.String("db", defaultDBPath, "sqlite db path")
	themePath := fs.String("themes", "config/themes.json", "theme rules json")
	outPath := fs.String("out", "report.json", "output file path or '-' for stdout")
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

	payload, err := computeReport(st, *country, *chart, *themePath, analysis.TrendConfig{
		RankWeight:    *rankWeight,
		ReviewWeight:  *reviewWeight,
		NewEntryBonus: *newEntryBonus,
	})
	if err != nil {
		return err
	}

	var out *os.File
	if *outPath == "-" {
		out = os.Stdout
	} else {
		if err := ensureDirForFile(*outPath); err != nil {
			return err
		}
		file, err := os.Create(*outPath)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}
