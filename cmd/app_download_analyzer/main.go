package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"app_download_analyzer/internal/analysis"
	"app_download_analyzer/internal/store"
)

const (
	defaultCountry = "kr"
	defaultChart   = "top-free"
	defaultLimit   = 25
	defaultDBPath  = "data/appstore.db"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "fetch":
		if err := runFetch(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "report":
		if err := runReport(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "report-json":
		if err := runReportJSON(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "timeseries-json":
		if err := runTimeSeriesJSON(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  app_download_analyzer fetch [--country kr] [--chart top-free] [--limit 25] [--db data/appstore.db] [--no-itunes]")
	fmt.Println("  app_download_analyzer report [--country kr] [--chart top-free] [--db data/appstore.db] [--top 10] [--themes config/themes.json]")
	fmt.Println("  app_download_analyzer report-json [--country kr] [--chart top-free] [--db data/appstore.db] [--themes config/themes.json] [--out report.json]")
	fmt.Println("  app_download_analyzer timeseries-json [--country kr] [--chart top-free] [--db data/appstore.db] [--themes config/themes.json] [--out timeseries.json] [--top 10]")
	fmt.Println("  app_download_analyzer serve [--country kr] [--chart top-free] [--limit 25] [--db data/appstore.db] [--themes config/themes.json] [--addr :8080]")
	fmt.Println("    (optional) --auto-fetch --fetch-on-start --interval 6h --no-itunes")
}

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	country := fs.String("country", defaultCountry, "storefront country code")
	chart := fs.String("chart", defaultChart, "chart name (top-free, top-paid)")
	limit := fs.Int("limit", defaultLimit, "chart size (25 or 50 recommended)")
	dbPath := fs.String("db", defaultDBPath, "sqlite db path")
	noItunes := fs.Bool("no-itunes", false, "skip iTunes lookup enrichment")
	timeout := fs.Duration("timeout", 20*time.Second, "http timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	client := &http.Client{Timeout: *timeout}
	ctx := context.Background()

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	snapshotID, count, err := fetchSnapshot(ctx, client, st, *country, *chart, *limit, *noItunes)
	if err != nil {
		return err
	}

	log.Printf("saved snapshot %d (%s/%s, %d items)", snapshotID, *country, *chart, count)
	return nil
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	country := fs.String("country", defaultCountry, "storefront country code")
	chart := fs.String("chart", defaultChart, "chart name (top-free, top-paid)")
	dbPath := fs.String("db", defaultDBPath, "sqlite db path")
	topN := fs.Int("top", 10, "top N trending apps")
	themePath := fs.String("themes", "config/themes.json", "theme rules json")
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

	if *topN > len(payload.Trends) {
		*topN = len(payload.Trends)
	}

	fmt.Printf("Latest snapshot: %s (%s %s)\n", payload.Latest.CollectedAt.Format(time.RFC3339), payload.Latest.Country, payload.Latest.Chart)
	fmt.Printf("Previous snapshot: %s\n", payload.Previous.CollectedAt.Format(time.RFC3339))
	fmt.Println()

	fmt.Println("Most used (current rank):")
	current := append([]analysis.AppTrend{}, payload.Trends...)
	sort.Slice(current, func(i, j int) bool {
		return current[i].Rank < current[j].Rank
	})
	for i := 0; i < *topN && i < len(current); i++ {
		item := current[i]
		fmt.Printf("%2d. #%d %s (%s)\n", i+1, item.Rank, item.AppName, item.Theme)
	}
	fmt.Println()

	fmt.Println("Trending apps:")
	for i := 0; i < *topN; i++ {
		item := payload.Trends[i]
		rankDelta := fmt.Sprintf("%+d", item.RankDelta)
		reviewDelta := fmt.Sprintf("%+d", item.RatingDelta)
		flags := []string{}
		if item.NewEntry {
			flags = append(flags, "new")
		}
		meta := strings.Join(flags, ",")
		if meta != "" {
			meta = " [" + meta + "]"
		}
		fmt.Printf("%2d. #%d %s (%s) rank %s reviews %s score %.2f%s\n",
			i+1, item.Rank, item.AppName, item.Theme, rankDelta, reviewDelta, item.TrendScore, meta)
	}
	fmt.Println()

	fmt.Println("Theme momentum:")
	for _, pair := range payload.ThemeScores {
		fmt.Printf("  %s: %.2f\n", pair.Theme, pair.Score)
	}
	fmt.Println()

	fmt.Printf("Risk-on score: %.2f\n", payload.RiskOnScore)
	fmt.Printf("Risk-off score: %.2f\n", payload.RiskOffScore)
	fmt.Printf("Rotation index: %.2f\n", payload.RotationIndex)
	return nil
}
