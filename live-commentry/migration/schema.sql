CREATE TABLE commentary (
    id BIGSERIAL PRIMARY KEY,
    match_id VARCHAR(50) NOT NULL,
    ball_number INT NOT NULL,
    over_number VARCHAR(10) NOT NULL,
    bowler VARCHAR(100) NOT NULL,
    batsman VARCHAR(100) NOT NULL,
    runs INT NOT NULL DEFAULT 0,
    is_wicket BOOLEAN NOT NULL DEFAULT FALSE,
    is_boundary BOOLEAN NOT NULL DEFAULT FALSE,
    text TEXT NOT NULL,
    score VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(match_id, ball_number)
);

CREATE INDEX idx_commentary_match_ball ON commentary(match_id, ball_number DESC);
