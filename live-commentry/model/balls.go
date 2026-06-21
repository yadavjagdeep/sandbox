package models

import "time"

type Ball struct {
    ID          int64     `json:"id,omitempty"`
    MatchID     string    `json:"match_id"`
    BallNumber  int       `json:"ball_number"`
    OverNumber  string    `json:"over_number"`
    Bowler      string    `json:"bowler"`
    Batsman     string    `json:"batsman"`
    Runs        int       `json:"runs"`
    IsWicket    bool      `json:"is_wicket"`
    IsBoundary  bool      `json:"is_boundary"`
    Text        string    `json:"text"`
    Score       string    `json:"score"`
    CreatedAt   time.Time `json:"created_at,omitempty"`
}
