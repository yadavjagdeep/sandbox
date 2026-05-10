package handlers

import (
	"crypto/md5"
	"fmt"
	"gravatar/db"
	"gravatar/models"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func CreateUser(c echo.Context) error {
	var user models.User

	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if user.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email is required"})
	}

	// generate hash: MD5 of trimmed, lowercased email
	trimmed := strings.TrimSpace(strings.ToLower(user.Email))
	user.Hash = fmt.Sprintf("%x", md5.Sum([]byte(trimmed)))

	err := db.Database.QueryRow(
		"INSERT INTO users (email, hash) VALUES ($1, $2) RETURNING id",
		user.Email, user.Hash,
	).Scan(&user.Id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, user)
}
