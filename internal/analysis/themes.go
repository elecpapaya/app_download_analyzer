package analysis

import (
	"encoding/json"
	"os"
	"strings"
)

type ThemeRule struct {
	Theme    string   `json:"theme"`
	GenreIDs []string `json:"genre_ids"`
	Genres   []string `json:"genres"`
	Keywords []string `json:"keywords"`
}

type ThemeConfig struct {
	Rules   []ThemeRule `json:"rules"`
	RiskOn  []string    `json:"risk_on"`
	RiskOff []string    `json:"risk_off"`
}

type ThemeScore struct {
	Theme string  `json:"theme"`
	Score float64 `json:"score"`
}

func LoadThemeConfig(path string) (ThemeConfig, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return defaultThemeConfig(), nil
		}
		return ThemeConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemeConfig{}, err
	}
	var cfg ThemeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ThemeConfig{}, err
	}
	if len(cfg.Rules) == 0 {
		return defaultThemeConfig(), nil
	}
	return cfg, nil
}

func defaultThemeConfig() ThemeConfig {
	return ThemeConfig{
		Rules: []ThemeRule{
			{Theme: "games", GenreIDs: []string{"6014"}, Genres: []string{"games"}},
			{Theme: "entertainment", GenreIDs: []string{"6016", "6011", "6008", "6005"}, Genres: []string{"entertainment", "music", "photo", "social networking"}},
			{Theme: "commerce", GenreIDs: []string{"6024", "6023"}, Genres: []string{"shopping", "food", "drink", "food & drink"}},
			{Theme: "travel", GenreIDs: []string{"6003", "6010"}, Genres: []string{"travel", "navigation"}},
			{Theme: "finance", GenreIDs: []string{"6015"}, Genres: []string{"finance"}},
			{Theme: "productivity", GenreIDs: []string{"6007", "6002", "6000"}, Genres: []string{"productivity", "utilities", "business"}},
			{Theme: "education", GenreIDs: []string{"6017"}, Genres: []string{"education"}},
			{Theme: "health", GenreIDs: []string{"6013", "6018"}, Genres: []string{"health", "fitness", "medical"}},
			{Theme: "news", GenreIDs: []string{"6009", "6006", "6001"}, Genres: []string{"news", "reference", "weather"}},
			{Theme: "sports", GenreIDs: []string{"6004"}, Genres: []string{"sports"}},
		},
		RiskOn:  []string{"games", "entertainment", "commerce", "travel", "sports"},
		RiskOff: []string{"productivity", "education", "health", "finance", "news"},
	}
}

type ThemeClassifier struct {
	rules []normalizedRule
}

type normalizedRule struct {
	theme    string
	genreIDs map[string]bool
	genres   []string
	keywords []string
}

type ThemeInput struct {
	Name         string
	Genres       []string
	GenreIDs     []string
	PrimaryGenre string
	ItunesGenres []string
}

func NewThemeClassifier(cfg ThemeConfig) *ThemeClassifier {
	rules := make([]normalizedRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		n := normalizedRule{
			theme:    strings.ToLower(rule.Theme),
			genreIDs: map[string]bool{},
			genres:   normalizeList(rule.Genres),
			keywords: normalizeList(rule.Keywords),
		}
		for _, id := range rule.GenreIDs {
			n.genreIDs[strings.TrimSpace(id)] = true
		}
		rules = append(rules, n)
	}
	return &ThemeClassifier{rules: rules}
}

func (c *ThemeClassifier) Classify(input ThemeInput) string {
	genres := normalizeList(append(input.Genres, append(input.ItunesGenres, input.PrimaryGenre)...))
	genreIDs := make(map[string]bool, len(input.GenreIDs))
	for _, id := range input.GenreIDs {
		genreIDs[strings.TrimSpace(id)] = true
	}
	name := strings.ToLower(input.Name)

	for _, rule := range c.rules {
		for id := range genreIDs {
			if rule.genreIDs[id] {
				return rule.theme
			}
		}
		for _, genre := range genres {
			if containsAny(genre, rule.genres) {
				return rule.theme
			}
		}
		if rule.keywords != nil && containsAny(name, rule.keywords) {
			return rule.theme
		}
	}
	return "other"
}

func SortThemeScores(scores map[string]float64) []ThemeScore {
	list := make([]ThemeScore, 0, len(scores))
	for theme, score := range scores {
		list = append(list, ThemeScore{Theme: theme, Score: score})
	}
	sortThemeScores(list)
	return list
}

func sortThemeScores(list []ThemeScore) {
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].Score > list[i].Score {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
}

func normalizeList(items []string) []string {
	var out []string
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func containsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
