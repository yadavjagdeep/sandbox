package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/images/:image", func(c echo.Context) error {
		name := c.Param("image")
		path := "images/" + name

		data, err := os.ReadFile(path)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "image not found"})
		}

		fmt.Printf("ORIGIN HIT: serving %s from disk\n", name)
		return c.Blob(http.StatusOK, "image/jpeg", data)
	})

	e.GET("api/products", func(c echo.Context) error {
		fmt.Println("ORIGIN HIT: serving products")
		return c.JSON(http.StatusOK, []map[string]string{
			{"name": "Keyboard", "price": "89.99"},
			{"name": "Mouse", "price": "59.99"},
			{"name": "Monitor", "price": "299.99"},
			{"name": "Headset", "price": "199.99"},
		})
	})

	fmt.Println("Origin server on :3000")
	e.Logger.Fatal(e.Start(":3000"))
}
