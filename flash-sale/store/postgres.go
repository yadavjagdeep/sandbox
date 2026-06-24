package store

import (
	"database/sql"
	"time"
)

type InventoryItem struct {
	ID     string     `json:"id"`
	ItemID string     `json:"item_id"`
	UserID *string    `json:"user_id,omitempty"`
	HeldAt *time.Time `json:"held_at,omitempty"`
	Paid   bool       `json:"paid"`
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) ClaimItem(itemID string, userID string) (*InventoryItem, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}

	var item InventoryItem
	err = tx.QueryRow(`
	SELECT id, item_id FROM inventory
	WHERE item_id = $1 AND user_id IS NULL
	LIMIT 1
	FOR UPDATE SKIP LOCKED`, itemID).Scan(&item.ID, &item.ItemID)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	now := time.Now()
	_, err = tx.Exec(`
	UPDATE inventory SET user_id = $1, held_at = $2
	WHERE id = $3`, userID, now, item.ID)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	item.UserID = &userID
	item.HeldAt = &now
	return &item, nil
}

func (s *PostgresStore) ConfirmPayment(inventoryID string, userID string) error {
	_, err := s.db.Exec(`
	UPDATE inventory SET paid = TRUE
	WHERE id = $1 AND user_id = $2`, inventoryID, userID)
	return err
}

func (s *PostgresStore) ReleaseExpireHolds(holdDuration time.Duration) ([]string, error) {
	rows, err := s.db.Query(`
	UPDATE inventory SET user_id = NULL, held_at = NULL
	WHERE held_at < $1 AND paid = FALSE AND user_id IS NOT NULL
	RETURNING item_id`, time.Now().Add(-holdDuration))

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var released []string
	for rows.Next() {
		var itemID string
		rows.Scan(&itemID)
		released = append(released, itemID)
	}
	return released, nil
}

func (s *PostgresStore) GetAvailableCount(itemID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
	SELECT COUNT(*) FROM inventory
	WHERE item_id = $1 AND user_id IS NULL`, itemID).Scan(&count)
	return count, err
}
