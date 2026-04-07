package handlers

import (
	"net/http"
	"pgstore/db"
	"pgstore/models"
	"strconv"
	"sync/atomic"

	"github.com/labstack/echo/v4"
)

var shardIdx int32

func CreateShardedProduct(c echo.Context) error {
	var p models.Product
	if err := c.Bind(&p); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Round-robin between shards
	idx := atomic.AddInt32(&shardIdx, 1) % int32(len(db.Shards))
	shard := db.Shards[idx]

	// Let the shard generate its own ID (even or odd based on sequence config)
	err := shard.QueryRow(
		"INSERT INTO products (name, price, category) VALUES ($1, $2, $3) RETURNING id, created_at",
		p.Name, p.Price, p.Category,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, p)
}

func GetShardedProduct(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	shard := db.GetShard(id)

	var p models.Product
	err := shard.QueryRow(
		"SELECT id, name, price, category, created_at FROM products WHERE id = $1",
		id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Category, &p.CreatedAt)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "product not found"})
	}
	return c.JSON(http.StatusOK, p)
}

func GetAllShardedProducts(c echo.Context) error {
	var allProducts []models.Product

	// Query both shards and merge
	for _, shard := range db.Shards {
		rows, err := shard.Query("SELECT id, name, price, category, created_at FROM products")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		defer rows.Close()

		for rows.Next() {
			var p models.Product
			rows.Scan(&p.ID, &p.Name, &p.Price, &p.Category, &p.CreatedAt)
			allProducts = append(allProducts, p)
		}
	}

	return c.JSON(http.StatusOK, allProducts)
}



// Sharding tradeoffs:

// - Cross-shard queries are slow — must query all shards and merge in app code
// - JOINs across shards impossible in SQL, must join in application
// - Sorting requires merge-sort across shard results
// - Pagination (LIMIT/OFFSET) doesn't work — must over-fetch from each shard and merge
// - Aggregations: COUNT = sum per shard, AVG = need SUM+COUNT per shard (can't average averages)
// - No cross-shard transactions (no native ACID across shards)
// - Adding a new shard means rebalancing existing data
// - Shard key is permanent — changing it means re-sharding everything
// - Bad shard key = hotspots (most data on one shard)
// - More operational overhead — more nodes to monitor, backup, upgrade

// When to shard: single node can't handle write volume, or data exceeds single machine capacity.

// When not to: try partitioning, read replicas, or vertical scaling first. Don't shard until you have to.