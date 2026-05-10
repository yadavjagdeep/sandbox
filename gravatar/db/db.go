package db

import (
	"database/sql"
	"fmt"
	"log"
)

var Database *sql.DB

func Connect() {
	conn := "host=localhost port=5432 user=gravatar password=gravatar123 dbname=gravatar sslmode=disable"

	var err error
	Database, err = sql.Open("postgres", conn)
	if err != nil {
		panic(err)
	}

	if err := Database.Ping(); err != nil {
		log.Fatal("failed to connect to db:", err)
	}
	fmt.Println("connected to gravatar database...")
}
