package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// sudo docker exec -it pgstore-primary psql -U pgstore - access it via terminal
// EXPLAIN ANALYZE SELECT * FROM products; - analysis query

var Primary *sql.DB
var Replica *sql.DB
var Shard1 *sql.DB
var Shard2 *sql.DB
var Shards []*sql.DB

func GetShard(id int) *sql.DB {
	return Shards[id%2]
}

// Postgres uses MVCC — updates create new row versions, not overwrite.
// Readers see the version valid for their transaction, so readers never block writers.
// Write locks exist to prevent lost updates when the new value depends on the old one (price = price + 1);
// the blocked session re-reads the latest committed value when it unblocks.

func Connect() {
	var err error

	primaryConn := "host=localhost port=5432 user=pgstore password=pgstore123 dbname=pgstore sslmode=disable"
	Primary, err = sql.Open("postgres", primaryConn)

	if err != nil {
		log.Fatal("failed to open primary db:", err)
	}

	if err = Primary.Ping(); err != nil {
		log.Fatal("failed to ping primary db:", err)
	}
	fmt.Println("conected to primary db...")

	replicaConn := "host=localhost port=5433 user=pgstore password=pgstore123 dbname=pgstore sslmode=disable"
	Replica, err = sql.Open("postgres", replicaConn)

	if err != nil {
		log.Fatal("failed to open replica db:", err)
	}
	if err = Replica.Ping(); err != nil {
		log.Fatal("failed to ping replica db:", err)
	}
	fmt.Println("connected to replica db...")

	shard1Conn := "host=localhost port=5434 user=pgstore password=pgstore123 dbname=pgstore sslmode=disable"
	Shard1, err = sql.Open("postgres", shard1Conn)

	if err != nil {
		log.Fatal("failed to open shard1:", err)
	}
	if err = Shard1.Ping(); err != nil {
		log.Fatal("failed to ping shard1:", err)
	}

	fmt.Println("connected to shard1 db...")

	shard2Conn := "host=localhost port=5435 user=pgstore password=pgstore123 dbname=pgstore sslmode=disable"
	Shard2, err = sql.Open("postgres", shard2Conn)

	if err != nil {
		log.Fatal("failed to opne shard2:", err)
	}
	if err = Shard2.Ping(); err != nil {
		log.Fatal("failed to ping shard2:", err)
	}

	fmt.Println("connected to shard2 db...")

	Shards = []*sql.DB{Shard1, Shard2}
}
