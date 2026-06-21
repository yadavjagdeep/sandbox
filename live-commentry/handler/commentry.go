package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	models "live-commentry/model"
	"live-commentry/store"
)

type CommentaryHandler struct {
	redis    *store.RedisStore
	postgres *store.PostgresStore
}

func NewCommentaryHandler(redis *store.RedisStore, postgres *store.PostgresStore) *CommentaryHandler {
	return &CommentaryHandler{
		redis:    redis,
		postgres: postgres,
	}
}

type SubmitResponse struct {
	BallNumber int    `json:"ball_number"`
	Redis      string `json:"redis"`
	Postgres   string `json:"postgres"`
	Error      string `json:"error,omitempty"`
}

// PostBall handles a new ball submission from the commentator.
func (h *CommentaryHandler) PostBall(c echo.Context) error {
	var ball models.Ball
	if err := c.Bind(&ball); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if ball.MatchID == "" || ball.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "match_id and text required"})
	}

	ctx := context.Background()
	resp := SubmitResponse{BallNumber: ball.BallNumber}

	// Write to Redis
	if err := h.redis.PushBall(ctx, ball); err != nil {
		resp.Redis = "failed"
		resp.Error = err.Error()
	} else {
		resp.Redis = "success"
	}

	// Write to PostgreSQL
	if err := h.postgres.InsertBall(ball); err != nil {
		resp.Postgres = "failed"
		if resp.Error == "" {
			resp.Error = err.Error()
		}
	} else {
		resp.Postgres = "success"
	}

	// If Redis failed, reject (live users won't see it)
	if resp.Redis == "failed" {
		return c.JSON(http.StatusInternalServerError, resp)
	}

	return c.JSON(http.StatusCreated, resp)
}

// EditBall handles editing an existing ball's commentary.
func (h *CommentaryHandler) EditBall(c echo.Context) error {
	var ball models.Ball
	if err := c.Bind(&ball); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	resp := SubmitResponse{BallNumber: ball.BallNumber}

	// Update Redis
	if err := h.redis.UpdateBall(ctx, ball); err != nil {
		resp.Redis = "failed"
	} else {
		resp.Redis = "success"
	}

	// Update PostgreSQL
	if err := h.postgres.UpdateBall(ball); err != nil {
		resp.Postgres = "failed"
	} else {
		resp.Postgres = "success"
	}

	return c.JSON(http.StatusOK, resp)
}

// GetLive returns the last 15 balls for a match (served from Redis).
func (h *CommentaryHandler) GetLive(c echo.Context) error {
	matchID := c.Param("matchId")
	if matchID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "matchId required"})
	}

	ctx := context.Background()
	balls, err := h.redis.GetLive(ctx, matchID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "redis read failed"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"match_id": matchID,
		"balls":    balls,
		"count":    len(balls),
	})
}

// GetHistory returns paginated commentary from PostgreSQL.
func (h *CommentaryHandler) GetHistory(c echo.Context) error {
	matchID := c.Param("matchId")
	cursorStr := c.QueryParam("cursor")
	limitStr := c.QueryParam("limit")

	cursor := 9999 // default: start from latest
	if cursorStr != "" {
		cursor, _ = strconv.Atoi(cursorStr)
	}

	limit := 20
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
		if limit > 50 {
			limit = 50
		}
	}

	balls, err := h.postgres.GetPaginated(matchID, cursor, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var nextCursor *int
	if len(balls) == limit {
		last := balls[len(balls)-1].BallNumber
		nextCursor = &last
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"match_id":    matchID,
		"balls":       balls,
		"count":       len(balls),
		"next_cursor": nextCursor,
	})
}
