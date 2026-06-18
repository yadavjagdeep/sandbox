package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"

	"multi-tier-storage/snowflake"
)

func main() {
	age := flag.String("age", "1d", "age of the order (e.g., 1d, 7m, 2y)")
	count := flag.Int("count", 1, "number of orders to create")
	flag.Parse()

	duration := parseDuration(*age)
	orderTime := time.Now().Add(-duration)

	db, err := sql.Open("postgres", "host=localhost port=5432 user=orders password=orders dbname=orders sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	for i := 0; i < *count; i++ {
		// Generate ID with backdated timestamp
		id := snowflake.GenerateWithTime(1, orderTime.Add(time.Duration(i)*time.Second))

		_, err := db.Exec(
			"INSERT INTO orders (id, user_id, status, total_amount, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
			id, 1001, "completed", 99.99, orderTime, orderTime,
		)
		if err != nil {
			log.Printf("insert order: %v", err)
			continue
		}

		_, err = db.Exec(
			"INSERT INTO order_items (order_id, product_name, quantity, price) VALUES ($1, $2, $3, $4)",
			id, "Test Product", 1, 99.99,
		)
		if err != nil {
			log.Printf("insert item: %v", err)
			continue
		}

		_, err = db.Exec(
			"INSERT INTO payments (order_id, method, status, amount) VALUES ($1, $2, $3, $4)",
			id, "credit_card", "completed", 99.99,
		)
		if err != nil {
			log.Printf("insert payment: %v", err)
			continue
		}

		fmt.Printf("Created order: %d (age: %s, tier: %s)\n", id, *age, tierName(duration))
	}
}

func parseDuration(s string) time.Duration {
	n := 0
	unit := ""
	fmt.Sscanf(s, "%d%s", &n, &unit)

	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour
	case "m":
		return time.Duration(n) * 30 * 24 * time.Hour
	case "y":
		return time.Duration(n) * 365 * 24 * time.Hour
	default:
		log.Fatalf("unknown unit: %s (use d/m/y)", unit)
		return 0
	}
}

func tierName(age time.Duration) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	twoYears := 2 * 365 * 24 * time.Hour

	switch {
	case age < sixMonths:
		return "hot (SQL)"
	case age < twoYears:
		return "warm (DynamoDB)"
	default:
		return "cold (S3)"
	}
}
