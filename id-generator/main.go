package main

import (
	"fmt"
	"net/http"

	"id-generator/snowflake"
	"id-generator/central"

	"github.com/labstack/echo/v4"
)

func main() {
	sf := snowflake.New(1)

	e := echo.New()

	e.GET("/snowflake", func(c echo.Context) error {
		id := sf.Genrate()
		ts, mid, seq := snowflake.Parse(id)

		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":         id,
			"timestamp":  ts,
			"machine_id": mid,
			"sequence":   seq,
		})

	})

	e.GET("/snowflake/batch", func(c echo.Context) error {
		var ids []map[string]interface{}
		for i := 0; i < 10; i++ {
			id := sf.Genrate()
			ts, mid, seq := snowflake.Parse(id)
			ids = append(ids, map[string]interface{}{
				"id":         id,
				"timestamp":  ts,
				"machine_id": mid,
				"sequence":   seq,
			})
		}
		return c.JSON(http.StatusOK, ids)
	})
	
	centralIdService := central.NewIDService()
	client := central.NewBatchClient(centralIdService, 1000)

	// Returns a range for external clients
	e.GET("/central/batch", func(c echo.Context) error {
		start, end := centralIdService.AllocateBatch(1000)
		return c.JSON(http.StatusOK, map[string]int64{
			"start": start,
			"end":   end,
		})
	})

	// Returns a single ID (uses batching internally)
	e.GET("/central/next", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]int64{"id": client.NextID()})
	})


	fmt.Println("ID generator on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
