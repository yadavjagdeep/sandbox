package models

import "time"

type Job struct {
	ID             int64      `json:"id"`
	Command        any        `json:"command"`
	ScheduledAt    time.Time  `json:"scheduled_at"`
	PickedAt       *time.Time `json:"picked_at,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	RecurringJobID *int64     `json:"recurring_job_id,omitempty"`
}

type RecurringJob struct {
	ID       int64  `json:"id"`
	Command  any    `json:"command"`
	CronExpr string `json:"cron_expr"`
	IsActive bool   `json:"is_active"`
}

type JobCompletion struct {
	JobID       int64     `json:"job_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}
