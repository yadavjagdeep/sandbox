package models

type ViewEvent struct {
	VideoID       string `json:"video_id"`
	UserID        string `json:"user_id"`
	WatchDuration int    `json:"watch_duration"` // seconds
	Timestamp     int64  `json:"timestamp"`
}

type VideoCount struct {
	VideoID   string `json:"video_id"`
	ViewCount int64  `json:"view_count"`
}
