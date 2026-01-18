package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var validCharts = map[string]bool{
	"top-free": true,
	"top-paid": true,
}

const rssBaseURL = "https://rss.marketingtools.apple.com/api/v2"

type RSSResponse struct {
	Feed RSSFeed `json:"feed"`
}

type RSSFeed struct {
	Title   string    `json:"title"`
	Country string    `json:"country"`
	Updated string    `json:"updated"`
	Results []RSSApp  `json:"results"`
	Links   []RSSLink `json:"links"`
}

type RSSLink struct {
	Self string `json:"self"`
}

type RSSApp struct {
	ArtistName  string     `json:"artistName"`
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	ReleaseDate string     `json:"releaseDate"`
	Kind        string     `json:"kind"`
	ArtworkURL  string     `json:"artworkUrl100"`
	Genres      []RSSGenre `json:"genres"`
	URL         string     `json:"url"`
}

type RSSGenre struct {
	GenreID string `json:"genreId"`
	Name    string `json:"name"`
	URL     string `json:"url"`
}

func ValidChart(chart string) bool {
	return validCharts[chart]
}

func FetchTopChart(ctx context.Context, client *http.Client, country, chart string, limit int) (RSSResponse, string, error) {
	var resp RSSResponse
	if !ValidChart(chart) {
		return resp, "", fmt.Errorf("invalid chart: %s", chart)
	}
	url := fmt.Sprintf("%s/%s/apps/%s/%d/apps.json", rssBaseURL, country, chart, limit)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return resp, "", err
		}
		req.Header.Set("User-Agent", "app_download_analyzer/1.0")

		res, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			func() {
				defer res.Body.Close()
				if res.StatusCode != http.StatusOK {
					lastErr = fmt.Errorf("rss request failed: %s", res.Status)
					return
				}
				if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
					lastErr = err
					return
				}
				lastErr = nil
			}()
			if lastErr == nil {
				return resp, url, nil
			}
			if res.StatusCode < 500 && res.StatusCode != http.StatusTooManyRequests {
				return resp, "", lastErr
			}
		}

		if attempt < 2 {
			select {
			case <-time.After(time.Duration(500*(attempt+1)) * time.Millisecond):
			case <-ctx.Done():
				return resp, "", ctx.Err()
			}
		}
	}

	return resp, "", lastErr
}

func ExtractGenres(genres []RSSGenre) ([]string, []string) {
	names := make([]string, 0, len(genres))
	ids := make([]string, 0, len(genres))
	for _, genre := range genres {
		if genre.Name != "" {
			names = append(names, genre.Name)
		}
		if genre.GenreID != "" {
			ids = append(ids, genre.GenreID)
		}
	}
	return names, ids
}
