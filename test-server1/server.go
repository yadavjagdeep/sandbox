package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Router() *gin.Engine {
	router := gin.Default()
	return router
}

func SetupRoutes(router *gin.Engine) {

	router.GET("/ping", ping)
	router.GET("/health", health)
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK,
		gin.H{
			"message": "pong",
			"port":    "8080",
		},
	)
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}
