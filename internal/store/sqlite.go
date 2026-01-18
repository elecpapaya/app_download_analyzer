package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Snapshot struct {
	ID          int64
	CollectedAt time.Time
	Country     string
	Chart       string
	Limit       int
	SourceURL   string
}

type ChartItem struct {
	SnapshotID    int64
	Rank          int
	AppID         string
	AppName       string
	ArtistName    string
	AppURL        string
	ReleaseDate   string
	Genres        []string
	GenreIDs      []string
	PrimaryGenre  string
	ItunesGenres  []string
	RatingCount   NullInt
	AverageRating NullFloat
}

type NullInt struct {
	Value int
	Valid bool
}

type NullFloat struct {
	Value float64
	Valid bool
}

func NullableInt(value int) NullInt {
	return NullInt{Value: value, Valid: true}
}

func NullableFloat(value float64) NullFloat {
	return NullFloat{Value: value, Valid: true}
}

func Open(path string) (*Store, error) {
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	st := &Store{db: db}
	if err := st.Init(); err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Init() error {
	schema := `
CREATE TABLE IF NOT EXISTS snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  collected_at TEXT NOT NULL,
  country TEXT NOT NULL,
  chart TEXT NOT NULL,
  limit_n INTEGER NOT NULL,
  source_url TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS chart_items (
  snapshot_id INTEGER NOT NULL,
  rank INTEGER NOT NULL,
  app_id TEXT NOT NULL,
  app_name TEXT NOT NULL,
  artist_name TEXT NOT NULL,
  app_url TEXT NOT NULL,
  release_date TEXT,
  genres TEXT,
  genre_ids TEXT,
  primary_genre TEXT,
  itunes_genres TEXT,
  rating_count INTEGER,
  average_rating REAL,
  PRIMARY KEY (snapshot_id, rank),
  UNIQUE (snapshot_id, app_id),
  FOREIGN KEY(snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chart_items_app ON chart_items(app_id);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) InsertSnapshot(snapshot Snapshot) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO snapshots (collected_at, country, chart, limit_n, source_url) VALUES (?, ?, ?, ?, ?)`,
		snapshot.CollectedAt.Format(time.RFC3339),
		snapshot.Country,
		snapshot.Chart,
		snapshot.Limit,
		snapshot.SourceURL,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) InsertChartItem(item ChartItem) error {
	var ratingCount sql.NullInt64
	var averageRating sql.NullFloat64
	if item.RatingCount.Valid {
		ratingCount = sql.NullInt64{Int64: int64(item.RatingCount.Value), Valid: true}
	}
	if item.AverageRating.Valid {
		averageRating = sql.NullFloat64{Float64: item.AverageRating.Value, Valid: true}
	}
	_, err := s.db.Exec(
		`INSERT INTO chart_items (snapshot_id, rank, app_id, app_name, artist_name, app_url, release_date, genres, genre_ids, primary_genre, itunes_genres, rating_count, average_rating)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.SnapshotID,
		item.Rank,
		item.AppID,
		item.AppName,
		item.ArtistName,
		item.AppURL,
		item.ReleaseDate,
		joinList(item.Genres),
		joinList(item.GenreIDs),
		item.PrimaryGenre,
		joinList(item.ItunesGenres),
		ratingCount,
		averageRating,
	)
	return err
}

func (s *Store) GetLatestSnapshot(country, chart string) (Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, collected_at, country, chart, limit_n, source_url
		 FROM snapshots
		 WHERE country = ? AND chart = ?
		 ORDER BY collected_at DESC
		 LIMIT 1`,
		country, chart,
	)
	return scanSnapshot(row)
}

func (s *Store) GetPreviousSnapshot(country, chart string, before time.Time) (Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, collected_at, country, chart, limit_n, source_url
		 FROM snapshots
		 WHERE country = ? AND chart = ? AND collected_at < ?
		 ORDER BY collected_at DESC
		 LIMIT 1`,
		country, chart, before.Format(time.RFC3339),
	)
	return scanSnapshot(row)
}

func (s *Store) GetSnapshotItems(snapshotID int64) ([]ChartItem, error) {
	rows, err := s.db.Query(
		`SELECT snapshot_id, rank, app_id, app_name, artist_name, app_url, release_date, genres, genre_ids, primary_genre, itunes_genres, rating_count, average_rating
		 FROM chart_items
		 WHERE snapshot_id = ?
		 ORDER BY rank ASC`,
		snapshotID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ChartItem
	for rows.Next() {
		var item ChartItem
		var genres, genreIDs, itunesGenres sql.NullString
		var ratingCount sql.NullInt64
		var averageRating sql.NullFloat64
		if err := rows.Scan(
			&item.SnapshotID,
			&item.Rank,
			&item.AppID,
			&item.AppName,
			&item.ArtistName,
			&item.AppURL,
			&item.ReleaseDate,
			&genres,
			&genreIDs,
			&item.PrimaryGenre,
			&itunesGenres,
			&ratingCount,
			&averageRating,
		); err != nil {
			return nil, err
		}
		if genres.Valid {
			item.Genres = splitList(genres.String)
		}
		if genreIDs.Valid {
			item.GenreIDs = splitList(genreIDs.String)
		}
		if itunesGenres.Valid {
			item.ItunesGenres = splitList(itunesGenres.String)
		}
		if ratingCount.Valid {
			item.RatingCount = NullInt{Value: int(ratingCount.Int64), Valid: true}
		}
		if averageRating.Valid {
			item.AverageRating = NullFloat{Value: averageRating.Float64, Valid: true}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanSnapshot(row *sql.Row) (Snapshot, error) {
	var snapshot Snapshot
	var collected string
	if err := row.Scan(
		&snapshot.ID,
		&collected,
		&snapshot.Country,
		&snapshot.Chart,
		&snapshot.Limit,
		&snapshot.SourceURL,
	); err != nil {
		return Snapshot{}, err
	}
	parsed, err := time.Parse(time.RFC3339, collected)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse collected_at: %w", err)
	}
	snapshot.CollectedAt = parsed
	return snapshot, nil
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func joinList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, "|")
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "|")
}
