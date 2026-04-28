package main

import (
	"database/sql"
	"fmt"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=5440 user=hashtag password=hashtag123 dbname=hashtag sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	e := echo.New()

	e.GET("/hastag/:name", func(c echo.Context) error {
		name := "#" + c.Param("name")

		var photoCount int64
		var topPhotos string
		err := db.QueryRow(
			"SELECT photo_count, top_photos FROM hashtags WHERE name = $1", name,
		).Scan(&photoCount, &topPhotos)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.JSON(404, map[string]string{
					"error": "hashtag not found",
				})
			}

			return c.JSON(500, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(200, map[string]interface{}{
			"name":        name,
			"photo_count": photoCount,
			"top_photos":  topPhotos,
		})
	})

	fmt.Println("API server on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
