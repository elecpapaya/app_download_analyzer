package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ItunesResponse struct {
	ResultCount int         `json:"resultCount"`
	Results     []ItunesApp `json:"results"`
}

type ItunesApp struct {
	TrackID                            int64    `json:"trackId"`
	TrackName                          string   `json:"trackName"`
	SellerName                         string   `json:"sellerName"`
	Description                        string   `json:"description"`
	PrimaryGenreName                   string   `json:"primaryGenreName"`
	Genres                             []string `json:"genres"`
	UserRatingCount                    int      `json:"userRatingCount"`
	AverageUserRating                  float64  `json:"averageUserRating"`
	UserRatingCountForCurrentVersion   int      `json:"userRatingCountForCurrentVersion"`
	AverageUserRatingForCurrentVersion float64  `json:"averageUserRatingForCurrentVersion"`
}

func LookupApp(ctx context.Context, client *http.Client, appID, country string) (ItunesApp, bool, error) {
	var resp ItunesResponse
	url := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&country=%s", appID, country)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ItunesApp{}, false, err
	}
	req.Header.Set("User-Agent", "app_download_analyzer/1.0")

	res, err := client.Do(req)
	if err != nil {
		return ItunesApp{}, false, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ItunesApp{}, false, fmt.Errorf("itunes request failed: %s", res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return ItunesApp{}, false, err
	}
	if resp.ResultCount < 1 || len(resp.Results) == 0 {
		return ItunesApp{}, false, nil
	}
	return resp.Results[0], true, nil
}
