package main

import (
	"fmt"
	"pgstore/db"
	"pgstore/handlers"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	db.Connect()

	defer db.Primary.Close()
	defer db.Replica.Close()
	defer db.Shard1.Close()
	defer db.Shard2.Close()

	db.RunMigrations()

	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			fmt.Printf("%s %s → %d\n", v.Method, v.URI, v.Status)
			return nil
		},
	}))

	// Primary/replica
	e.POST("/products", handlers.CreateProduct)
	e.GET("/products", handlers.GetProducts)
	e.POST("/orders", handlers.CreateOrder)
	e.GET("/orders", handlers.GetOrders)

	// Sharded routes
	e.POST("/sharded/products", handlers.CreateShardedProduct)
	e.GET("/sharded/products", handlers.GetAllShardedProducts)
	e.GET("/sharded/products/:id", handlers.GetShardedProduct)

	e.Logger.Fatal(e.Start(":8080"))
}
