package handlers

import (
	"net/http"
	"pgstore/db"
	"pgstore/models"

	"github.com/labstack/echo/v4"
)

func CreateOrder(c echo.Context) error {
	var o models.Order
	if err := c.Bind(&o); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	err := db.Primary.QueryRow(
		"INSERT INTO orders (product_id, quantity, total) VALUES ($1, $2, $3) RETURNING id, ordered_at",
		o.ProductID, o.Quantity, o.Total,
	).Scan(&o.ID, &o.OrderedAt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, o)
}

func GetOrders(c echo.Context) error {
	rows, err := db.Replica.Query("SELECT id, product_id, quantity, total, ordered_at FROM orders")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.ProductID, &o.Quantity, &o.Total, &o.OrderedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		orders = append(orders, o)
	}
	return c.JSON(http.StatusOK, orders)
}
