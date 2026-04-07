package models

import "time"

type Order struct {
	ID        int       `json:"id"`
	ProductID int       `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Total     float64   `json:"total"`
	OrderedAt time.Time `json:"ordered_at"`
}
