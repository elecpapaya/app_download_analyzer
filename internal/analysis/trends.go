package analysis

import (
	"math"

	"app_download_analyzer/internal/store"
)

type TrendConfig struct {
	RankWeight    float64
	ReviewWeight  float64
	NewEntryBonus float64
}

type AppTrend struct {
	AppID       string  `json:"app_id"`
	AppName     string  `json:"app_name"`
	AppURL      string  `json:"app_url"`
	Rank        int     `json:"rank"`
	RankDelta   int     `json:"rank_delta"`
	RatingCount int     `json:"rating_count"`
	RatingDelta int     `json:"rating_delta"`
	TrendScore  float64 `json:"trend_score"`
	Theme       string  `json:"theme"`
	NewEntry    bool    `json:"new_entry"`
}

type TrendResult struct {
	Trends        []AppTrend
	ThemeScores   map[string]float64
	RiskOnScore   float64
	RiskOffScore  float64
	RotationIndex float64
}

func AnalyzeTrends(latest store.Snapshot, previous store.Snapshot, latestItems, previousItems []store.ChartItem, cfg TrendConfig, themes ThemeConfig) TrendResult {
	prevMap := map[string]store.ChartItem{}
	for _, item := range previousItems {
		prevMap[item.AppID] = item
	}

	rankDeltas := make([]float64, 0, len(latestItems))
	reviewDeltas := make([]float64, 0, len(latestItems))
	trends := make([]AppTrend, 0, len(latestItems))

	classifier := NewThemeClassifier(themes)

	for _, item := range latestItems {
		prev, ok := prevMap[item.AppID]
		prevRank := latest.Limit + 1
		if ok {
			prevRank = prev.Rank
		}
		rankDelta := prevRank - item.Rank

		ratingDelta := computeRatingDelta(item, prev, ok)
		rankDeltas = append(rankDeltas, float64(rankDelta))
		reviewDeltas = append(reviewDeltas, float64(ratingDelta))

		theme := classifier.Classify(ThemeInput{
			Name:         item.AppName,
			Genres:       item.Genres,
			GenreIDs:     item.GenreIDs,
			PrimaryGenre: item.PrimaryGenre,
			ItunesGenres: item.ItunesGenres,
		})

		trends = append(trends, AppTrend{
			AppID:       item.AppID,
			AppName:     item.AppName,
			AppURL:      item.AppURL,
			Rank:        item.Rank,
			RankDelta:   rankDelta,
			RatingCount: item.RatingCount.Value,
			RatingDelta: ratingDelta,
			Theme:       theme,
			NewEntry:    !ok,
		})
	}

	rankMean, rankStd := meanStd(rankDeltas)
	reviewMean, reviewStd := meanStd(reviewDeltas)

	for i := range trends {
		rankZ := zscore(float64(trends[i].RankDelta), rankMean, rankStd)
		reviewZ := zscore(float64(trends[i].RatingDelta), reviewMean, reviewStd)
		score := cfg.RankWeight*rankZ + cfg.ReviewWeight*reviewZ
		if trends[i].NewEntry {
			score += cfg.NewEntryBonus
		}
		trends[i].TrendScore = score
	}

	trends = sortTrends(trends)

	themeScores := map[string]float64{}
	themeCounts := map[string]int{}
	for _, trend := range trends {
		themeScores[trend.Theme] += trend.TrendScore
		themeCounts[trend.Theme]++
	}
	for theme, total := range themeScores {
		count := themeCounts[theme]
		if count > 0 {
			themeScores[theme] = total / float64(count)
		}
	}

	riskOnScore := averageThemes(themeScores, themes.RiskOn)
	riskOffScore := averageThemes(themeScores, themes.RiskOff)

	return TrendResult{
		Trends:        trends,
		ThemeScores:   themeScores,
		RiskOnScore:   riskOnScore,
		RiskOffScore:  riskOffScore,
		RotationIndex: riskOnScore - riskOffScore,
	}
}

func computeRatingDelta(current store.ChartItem, prev store.ChartItem, prevOk bool) int {
	if !current.RatingCount.Valid {
		return 0
	}
	if prevOk && prev.RatingCount.Valid {
		return current.RatingCount.Value - prev.RatingCount.Value
	}
	return current.RatingCount.Value
}

func meanStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	return mean, math.Sqrt(variance)
}

func zscore(value, mean, std float64) float64 {
	if std == 0 {
		return 0
	}
	return (value - mean) / std
}

func sortTrends(items []AppTrend) []AppTrend {
	out := append([]AppTrend{}, items...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].TrendScore > out[i].TrendScore {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func averageThemes(scores map[string]float64, themes []string) float64 {
	if len(themes) == 0 {
		return 0
	}
	var sum float64
	var count int
	for _, theme := range themes {
		if score, ok := scores[theme]; ok {
			sum += score
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
