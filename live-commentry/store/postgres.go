package store

import (
	"database/sql"
	models "live-commentry/model"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// InsertBall inserts a new commentary entry.
func (s *PostgresStore) InsertBall(ball models.Ball) error {
	_, err := s.db.Exec(`
        INSERT INTO commentary (match_id, ball_number, over_number, bowler, batsman, runs, is_wicket, is_boundary, text, score)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		ball.MatchID, ball.BallNumber, ball.OverNumber, ball.Bowler, ball.Batsman,
		ball.Runs, ball.IsWicket, ball.IsBoundary, ball.Text, ball.Score,
	)
	return err
}

// UpdateBall updates an existing commentary entry.
func (s *PostgresStore) UpdateBall(ball models.Ball) error {
	_, err := s.db.Exec(`
        UPDATE commentary SET text = $1, runs = $2, is_wicket = $3, is_boundary = $4, score = $5
        WHERE match_id = $6 AND ball_number = $7`,
		ball.Text, ball.Runs, ball.IsWicket, ball.IsBoundary, ball.Score,
		ball.MatchID, ball.BallNumber,
	)
	return err
}

// GetPaginated returns commentary for a match, paginated by cursor (ball_number).
// Returns balls older than the cursor.
func (s *PostgresStore) GetPaginated(matchID string, cursor int, limit int) ([]models.Ball, error) {
	query := `
        SELECT id, match_id, ball_number, over_number, bowler, batsman, runs, is_wicket, is_boundary, text, score, created_at
        FROM commentary
        WHERE match_id = $1 AND ball_number < $2
        ORDER BY ball_number DESC
        LIMIT $3`

	rows, err := s.db.Query(query, matchID, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balls []models.Ball
	for rows.Next() {
		var b models.Ball
		if err := rows.Scan(&b.ID, &b.MatchID, &b.BallNumber, &b.OverNumber,
			&b.Bowler, &b.Batsman, &b.Runs, &b.IsWicket, &b.IsBoundary,
			&b.Text, &b.Score, &b.CreatedAt); err != nil {
			return nil, err
		}
		balls = append(balls, b)
	}

	return balls, nil
}
