package store

import (
	"database/sql"
	"encoding/json"
	"job-schedular/models"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) InsertJob(command any, scheduledAt time.Time) (int64, error) {
	cmdJSON, err := json.Marshal(command)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRow(`
        INSERT INTO jobs (command, scheduled_at)
        VALUES ($1, $2)
        RETURNING id`,
		cmdJSON, scheduledAt,
	).Scan(&id)

	return id, err
}

func (s *PostgresStore) PickDueJobs(limit int) ([]models.Job, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(`
	SELECT id, command, scheduled_at
	FROM jobs
	WHERE scheduled_at <= NOW() + INTERVAL '30 seconds'
	AND picked_at IS NULL
	ORDER BY scheduled_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED`, limit)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	var jobs []models.Job
	var ids []int64

	for rows.Next() {
		var j models.Job
		var cmdBytes []byte
		if err := rows.Scan(&j.ID, &cmdBytes, &j.ScheduledAt); err != nil {
			rows.Close()
			tx.Rollback()
			return nil, err
		}
		json.Unmarshal(cmdBytes, &j.Command)
		jobs = append(jobs, j)
		ids = append(ids, j.ID)
	}
	rows.Close()

	if len(ids) > 0 {
		// Update picked_at for all selected jobs
		now := time.Now()
		for _, id := range ids {
			tx.Exec(`UPDATE jobs SET picked_at = $1 WHERE id = $2`, now, id)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *PostgresStore) UpdateCompletion(completions []models.JobCompletion) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	for _, c := range completions {
		_, err := tx.Exec(`
            UPDATE jobs SET started_at = $1, completed_at = $2 WHERE id = $3`,
			c.StartedAt, c.CompletedAt, c.JobID)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) GetPendingRecurringCount(recurringJobID int64) (int, error) {
	var count int
	err := s.db.QueryRow(`
        SELECT COUNT(*) FROM jobs
        WHERE recurring_job_id = $1 AND picked_at IS NULL`,
		recurringJobID).Scan(&count)
	return count, err
}

func (s *PostgresStore) GetActiveRecurringJobs() ([]models.RecurringJob, error) {
	rows, err := s.db.Query(`SELECT id, command, cron_expr, is_active FROM recurring_jobs WHERE is_active = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.RecurringJob
	for rows.Next() {
		var j models.RecurringJob
		var cmdBytes []byte
		if err := rows.Scan(&j.ID, &cmdBytes, &j.CronExpr, &j.IsActive); err != nil {
			return nil, err
		}
		json.Unmarshal(cmdBytes, &j.Command)
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *PostgresStore) InsertRecurringJob(command any, cronExpr string) (int64, error) {
	cmdJSON, _ := json.Marshal(command)
	var id int64
	err := s.db.QueryRow(`
        INSERT INTO recurring_jobs (command, cron_expr)
        VALUES ($1, $2) RETURNING id`, cmdJSON, cronExpr).Scan(&id)
	return id, err
}

func (s *PostgresStore) InsertScheduledJob(command any, scheduledAt time.Time, recurringJobID int64) error {
	cmdJSON, _ := json.Marshal(command)
	_, err := s.db.Exec(`
        INSERT INTO jobs (command, scheduled_at, recurring_job_id)
        VALUES ($1, $2, $3)`, cmdJSON, scheduledAt, recurringJobID)
	return err
}
