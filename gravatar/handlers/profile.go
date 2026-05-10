package handlers

import (
	"gravatar/db"
	"gravatar/models"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/labstack/echo/v4"
)

func UploadPhoto(c echo.Context) error {
	userId, _ := strconv.ParseInt(c.Param("user_id"), 10, 64)

	file, err := c.FormFile("photo")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var photo models.Photo
	photo.UserId = userId

	err = db.Database.QueryRow(
		"INSERT INTO photos (user_id, is_active) VALUES ($1, false) RETURNING id", userId,
	).Scan(&photo.Id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	//save file to disk as images/<photo_id>.png
	src, _ := file.Open()
	defer src.Close()

	dst := filepath.Join("images", strconv.FormatInt(photo.Id, 10)+".png")
	out, err := os.Create(dst)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer out.Close()

	if _, err = out.ReadFrom(src); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, photo)
}

// setActivePhoto - deactivates current, activates the chosen one (in a tx)

func SetActivePhoto(c echo.Context) error {
	user_id, _ := strconv.ParseInt(c.Param("user_id"), 10, 64)
	photoId, _ := strconv.ParseInt(c.Param("photo_id"), 10, 64)

	tx, err := db.Database.Begin()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// deactivate current active photo
	_, err = tx.Exec(
		"UPDATE photos SET is_active = false WHERE user_id = $1 AND is_active = true", user_id,
	)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	//activate new one
	_, err = tx.Exec(
		"UPDATE photos SET is_active = true WHERE id = $1 AND user_id = $2", photoId, user_id,
	)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	tx.Commit()
	return c.JSON(http.StatusOK, map[string]string{"message": "active photo updated"})
}

// GetPhotByHash - serves the active photo for a given email hash
func GetPhotByHash(c echo.Context) error {
	hash := c.Param("hash")

	var photoId int64
	err := db.Database.QueryRow(
		"SELECT p.id FROM photos p JOIN users u on p.user_id = u.id WHERE u.hash = $1 AND p.is_active = true", hash,
	).Scan(&photoId)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no active photo found"})
	}
	return c.File(filepath.Join("images", strconv.FormatInt(photoId, 10)+".png"))
}
