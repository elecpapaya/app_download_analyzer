package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"app_download_analyzer/internal/apple"
	"app_download_analyzer/internal/store"
)

func fetchSnapshot(ctx context.Context, client *http.Client, st *store.Store, country, chart string, limit int, noItunes bool) (int64, int, error) {
	if !apple.ValidChart(chart) {
		return 0, 0, fmt.Errorf("unsupported chart: %s", chart)
	}

	rss, sourceURL, err := apple.FetchTopChart(ctx, client, country, chart, limit)
	if err != nil {
		return 0, 0, err
	}
	if len(rss.Feed.Results) == 0 {
		return 0, 0, fmt.Errorf("rss returned no results")
	}

	snapshotID, err := st.InsertSnapshot(store.Snapshot{
		CollectedAt: time.Now().UTC(),
		Country:     country,
		Chart:       chart,
		Limit:       limit,
		SourceURL:   sourceURL,
	})
	if err != nil {
		return 0, 0, err
	}

	for idx, item := range rss.Feed.Results {
		rank := idx + 1
		genres, genreIDs := apple.ExtractGenres(item.Genres)

		var itunesMeta *apple.ItunesApp
		if !noItunes {
			meta, ok, err := apple.LookupApp(ctx, client, item.ID, country)
			if err != nil {
				log.Printf("itunes lookup failed for %s: %v", item.ID, err)
			} else if ok {
				itunesMeta = &meta
			}
			time.Sleep(150 * time.Millisecond)
		}

		chartItem := store.ChartItem{
			SnapshotID:   snapshotID,
			Rank:         rank,
			AppID:        item.ID,
			AppName:      item.Name,
			ArtistName:   item.ArtistName,
			AppURL:       item.URL,
			ReleaseDate:  item.ReleaseDate,
			Genres:       genres,
			GenreIDs:     genreIDs,
			PrimaryGenre: "",
			ItunesGenres: nil,
		}

		if itunesMeta != nil {
			chartItem.PrimaryGenre = itunesMeta.PrimaryGenreName
			chartItem.ItunesGenres = itunesMeta.Genres
			chartItem.RatingCount = store.NullableInt(itunesMeta.UserRatingCount)
			chartItem.AverageRating = store.NullableFloat(itunesMeta.AverageUserRating)
		}

		if err := st.InsertChartItem(chartItem); err != nil {
			return 0, 0, err
		}
	}

	return snapshotID, len(rss.Feed.Results), nil
}
