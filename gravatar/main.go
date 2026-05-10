package main

import (
	"gravatar/db"
	"gravatar/handlers"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

func main() {
	db.Connect()
	db.RunMigration()

	e := echo.New()

	e.POST("/users", handlers.CreateUser)
	e.POST("/users/:user_id/photos", handlers.UploadPhoto)
	e.PUT("/users/:user_id/photos/:photo_id/activate", handlers.SetActivePhoto)
	e.GET("/avatar/:hash", handlers.GetPhotByHash)

	e.Logger.Fatal(e.Start(":8080"))
}
