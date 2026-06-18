package main

import (
	"fmt"
	"net/http"
	"time"

	"id-generator/central"
	"id-generator/snowflake"

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

	e.GET("/debug/headers", func(c echo.Context) error {
		headers := make(map[string]string)
		for key, value := range c.Request().Header {
			headers[key] = value[0]
		}
		return c.JSON(http.StatusOK, headers)
	})

	// Simulate a slow endpoint (takes 5 seconds)
	e.GET("/slow", func(c echo.Context) error {
		time.Sleep(5 * time.Second)
		return c.JSON(http.StatusOK, map[string]string{"status": "finally done"})
	})

	e.GET("/flaky", func(c echo.Context) error {
		if time.Now().UnixNano()%2 == 0 {
			fmt.Println("flaky: returning 500")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "random failure"})
		}
		fmt.Println("flaky: returning 200")
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	fmt.Println("ID generator on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
