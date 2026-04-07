#!/bin/bash

# starts pgstore (Postgres + go server)

if sudo docker ps -a --format '{{.Names}}' | grep -q pgstore-primary; then
    echo "Starting existing Postgres containers..."
    sudo docker start pgstore-primary pgstore-replica pgstore-shard1 pgstore-shard2
else
    echo "Creating Postgres containers..."
    sudo docker-compose up -d
fi

# Wait for Postgres to be ready
echo "Waiting for Postgres..."
until sudo docker exec pgstore-primary pg_isready -U pgstore > /dev/null 2>&1; do
    sleep 1
done
until sudo docker exec pgstore-replica pg_isready -U pgstore > /dev/null 2>&1; do
    sleep 1
done
until sudo docker exec pgstore-shard1 pg_isready -U pgstore > /dev/null 2>&1; do
    sleep 1
done
until sudo docker exec pgstore-shard2 pg_isready -U pgstore > /dev/null 2>&1; do
    sleep 1
done
echo "Postgres is ready!"

# Start the Go server
echo "Starting API server..."
go run main.go
