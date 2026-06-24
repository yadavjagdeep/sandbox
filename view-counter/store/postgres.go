package store

import (
	"database/sql"
	"view-counter/models"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Uses upsert: insert if not exists, increment if exists.
func (s *PostgresStore) IncrementCount(videoID string, count int64) error {
	_, err := s.db.Exec(`
	INSERT INTO video_counts (video_id, view_count)
	VALUES ($1, $2)
	ON CONFLICT (video_id)
	DO UPDATE SET view_count = video_counts.view_count + $2
	`, videoID, count)

	return err
}

func (s *PostgresStore) GetCount(videoID string) (*models.VideoCount, error) {
	var vc models.VideoCount
	err := s.db.QueryRow(`
	SELECT video_id, view_count FROM video_counts WHERE video_id = $1`,
		videoID).Scan(&vc.VideoID, &vc.ViewCount)

	if err == sql.ErrNoRows {
		return &models.VideoCount{VideoID: videoID, ViewCount: 0}, nil
	}
	if err != nil {
		return nil, err
	}

	return &vc, nil
}
