package main

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)


func main()  {
	e := echo.New()

	e.Any("/*", func(c echo.Context) error {
		token := c.Request().Header.Get("Authorization")
		path := c.Request().Header.Get("X-Original-Path")

		fmt.Printf("Auth check: path=%s, token=%s\n", path, token)

		if token == "Bearer my-secret-token" {
			c.Response().Header().Set("x-user-id", "Jagdeep Yadav")
			return c.NoContent(http.StatusOK)
		}
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	})

	fmt.Println("Auth service on :9000")
	e.Logger.Fatal(e.Start(":9000"))
}