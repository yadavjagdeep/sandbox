package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func Router() *echo.Echo {
	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:  true,
		LogURI:     true,
		LogMethod:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("| %d | %v | %s %s", v.Status, v.Latency, v.Method, v.URI)
			return nil
		},
	}))
	return e
}

func SetUpRoutes(e *echo.Echo) {
	e.GET("/ping", ping)
	e.GET("/health", health)
}

func ping(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"message": "pong", "port": "8081"})
}

func health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"message": "OK"})
}
