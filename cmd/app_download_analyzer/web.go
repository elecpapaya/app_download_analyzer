package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"sync"
	"time"

	"app_download_analyzer/internal/analysis"
	"app_download_analyzer/internal/store"
)

//go:embed index.html
var indexHTML string

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	country := fs.String("country", defaultCountry, "storefront country code")
	chart := fs.String("chart", defaultChart, "chart name (top-free, top-paid)")
	dbPath := fs.String("db", defaultDBPath, "sqlite db path")
	themePath := fs.String("themes", "config/themes.json", "theme rules json")
	addr := fs.String("addr", ":8080", "http listen address")
	limit := fs.Int("limit", defaultLimit, "chart size (25 or 50 recommended)")
	autoFetch := fs.Bool("auto-fetch", true, "enable periodic snapshot fetch")
	fetchOnStart := fs.Bool("fetch-on-start", true, "fetch snapshot immediately on startup")
	interval := fs.Duration("interval", 6*time.Hour, "auto fetch interval")
	noItunes := fs.Bool("no-itunes", false, "skip iTunes lookup enrichment")
	timeout := fs.Duration("timeout", 20*time.Second, "http timeout")
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

	client := &http.Client{Timeout: *timeout}
	var mu sync.Mutex

	cfg := analysis.TrendConfig{
		RankWeight:    *rankWeight,
		ReviewWeight:  *reviewWeight,
		NewEntryBonus: *newEntryBonus,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	})

	http.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		payload, err := computeReport(st, *country, *chart, *themePath, cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/api/timeseries", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		payload, err := computeTimeSeries(st, *country, *chart, *themePath, cfg, *limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	if *autoFetch {
		go func() {
			doFetch := func() {
				mu.Lock()
				defer mu.Unlock()
				ctx := context.Background()
				snapshotID, count, err := fetchSnapshot(ctx, client, st, *country, *chart, *limit, *noItunes)
				if err != nil {
					log.Printf("auto fetch failed: %v", err)
					return
				}
				log.Printf("auto snapshot %d (%s/%s, %d items)", snapshotID, *country, *chart, count)
			}

			if *fetchOnStart {
				doFetch()
			}
			ticker := time.NewTicker(*interval)
			defer ticker.Stop()
			for range ticker.C {
				doFetch()
			}
		}()
	}

	log.Printf("serving report at http://localhost%s", *addr)
	return http.ListenAndServe(*addr, nil)
}
