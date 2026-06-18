package models

import "time"

// SQL models (hot tier - normalized)

type Order struct {
    ID          int64     `json:"id"`
    UserID      int64     `json:"user_id"`
    Status      string    `json:"status"`
    TotalAmount float64   `json:"total_amount"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type OrderItem struct {
    ID          int64   `json:"id"`
    OrderID     int64   `json:"order_id"`
    ProductName string  `json:"product_name"`
    Quantity    int     `json:"quantity"`
    Price       float64 `json:"price"`
}

type Payment struct {
    ID      int64      `json:"id"`
    OrderID int64      `json:"order_id"`
    Method  string     `json:"method"`
    Status  string     `json:"status"`
    Amount  float64    `json:"amount"`
    PaidAt  *time.Time `json:"paid_at,omitempty"`
}

// Combined document (warm tier - denormalized)

type OrderDocument struct {
    ID          int64       `json:"id" dynamodbav:"id"`
    UserID      int64       `json:"user_id" dynamodbav:"user_id"`
    Status      string      `json:"status" dynamodbav:"status"`
    TotalAmount float64     `json:"total_amount" dynamodbav:"total_amount"`
    CreatedAt   string      `json:"created_at" dynamodbav:"created_at"`
    UpdatedAt   string      `json:"updated_at" dynamodbav:"updated_at"`
    Items       []OrderItem `json:"items" dynamodbav:"items"`
    Payments    []Payment   `json:"payments" dynamodbav:"payments"`
    TTL         int64       `json:"ttl" dynamodbav:"ttl"`
}
