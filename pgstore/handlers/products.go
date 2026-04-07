package handlers

import (
	"net/http"
	"pgstore/db"
	"pgstore/models"

	"github.com/labstack/echo/v4"
)

func CreateProduct(c echo.Context) error {
	var p models.Product
	if err := c.Bind(&p); err != nil {
		return  c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	err := db.Primary.QueryRow(
		"INSERT INTO products (name, price, category) VALUES ($1, $2, $3) RETURNING id, created_at",
		p.Name, p.Price, p.Category,
	).Scan(&p.ID, &p.CreatedAt)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, p)
}

func GetProducts(c echo.Context) error {
	rows, err := db.Replica.Query("SELECT id, name, price, category, created_at FROM products")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Category, &p.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		products = append(products, p)
	}

	return c.JSON(http.StatusOK, products)
}
