package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/call-id-generator", func(c echo.Context) error {
		resp, err := http.Get("http://id-generator.default.svc.cluster.local:8080/snowflake")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return c.String(http.StatusOK, fmt.Sprintf("Got from id-generator: %s", string(body)))
	})

	e.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	fmt.Println("Caller service on :8081")
	e.Logger.Fatal(e.Start(":8081"))
}
