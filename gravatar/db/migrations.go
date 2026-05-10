package db

import "log"

func RunMigration() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		hash VARCHAR(255) UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS photos (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id),
		is_active BOOLEAN DEFAULT false
	);
	`
	_, err := Database.Exec(query)
	if err != nil {
		log.Fatal("migration failed:", err)
	}

	log.Println("migration successfully completed")
}
