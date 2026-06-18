package tier

import (
	"database/sql"
	"fmt"
	"multi-tier-storage/models"
)

type HotStore struct {
	db *sql.DB
}

func NewHotStore(db *sql.DB) *HotStore {
	return &HotStore{db: db}
}

func (s *HotStore) CreateOrder(order models.Order, items []models.OrderItem, payment models.Payment) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO orders (id, user_id, status, total_amount, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
		order.ID, order.UserID, order.Status, order.TotalAmount, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	for _, item := range items {
		_, err = tx.Exec(
			"INSERT INTO order_items (order_id, product_name, quantity, price) VALUES ($1, $2, $3, $4)",
			order.ID, item.ProductName, item.Quantity, item.Price,
		)
		if err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}

	_, err = tx.Exec(
		"INSERT INTO payments (order_id, method, status, amount) VALUES ($1, $2, $3, $4)",
		order.ID, payment.Method, payment.Status, payment.Amount,
	)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}

	return tx.Commit()
}

func (s *HotStore) GetOrder(id int64) (*models.OrderDocument, error) {
	var order models.Order
	err := s.db.QueryRow(
		"SELECT id, user_id, status, total_amount, created_at, updated_at FROM orders WHERE id = $1", id,
	).Scan(&order.ID, &order.UserID, &order.Status, &order.TotalAmount, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Get items
	rows, err := s.db.Query("SELECT id, order_id, product_name, quantity, price FROM order_items WHERE order_id = $1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductName, &item.Quantity, &item.Price); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	// Get payments
	pRows, err := s.db.Query("SELECT id, order_id, method, status, amount, paid_at FROM payments WHERE order_id = $1", id)
	if err != nil {
		return nil, err
	}
	defer pRows.Close()

	var payments []models.Payment
	for pRows.Next() {
		var p models.Payment
		if err := pRows.Scan(&p.ID, &p.OrderID, &p.Method, &p.Status, &p.Amount, &p.PaidAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	return &models.OrderDocument{
		ID:          order.ID,
		UserID:      order.UserID,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
		CreatedAt:   order.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   order.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Items:       items,
		Payments:    payments,
	}, nil
}

func (s *HotStore) DeleteOrder(id int64) error {
	_, err := s.db.Exec("DELETE FROM orders WHERE id = $1", id)
	return err
}
