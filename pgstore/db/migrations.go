package db

import "log"

func RunMigrations() {
	query := `
	CREATE TABLE IF NOT EXISTS products (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		price NUMERIC(10,2) NOT NULL,
		category TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL,
		product_id INT REFERENCES products(id),
		quantity INT NOT NULL,
		total NUMERIC(10, 2) NOT NULL,
		ordered_at TIMESTAMP NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, ordered_at)
	) PARTITION BY RANGE (ordered_at);


	CREATE TABLE IF NOT EXISTS orders_2025_q1 PARTITION OF orders FOR VALUES FROM ('2025-01-01') TO ('2025-04-01');
	CREATE TABLE IF NOT EXISTS orders_2025_q2 PARTITION OF orders FOR VALUES FROM ('2025-04-01') TO ('2025-07-01');
	CREATE TABLE IF NOT EXISTS orders_2025_q3 PARTITION OF orders FOR VALUES FROM ('2025-07-01') TO ('2025-10-01');
	CREATE TABLE IF NOT EXISTS orders_2025_q4 PARTITION OF orders FOR VALUES FROM ('2025-10-01') TO ('2026-01-01');
	CREATE TABLE IF NOT EXISTS orders_2026_q1 PARTITION OF orders FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');
	CREATE TABLE IF NOT EXISTS orders_2026_q2 PARTITION OF orders FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
	`

	_, err := Primary.Exec(query)
	if err != nil {
		log.Fatal("migration failed:", err)
	}

	for i, shard := range Shards {
		_, err := shard.Exec(query)
		if err != nil {
			log.Fatalf("migration failed on shard %d: %v", i+1, err)
		}
	}

	log.Println("migration done on all nodes")
}
