# Gravatar Service

A simplified Gravatar-like service built with Go, Echo, and PostgreSQL. Users register with an email, and the service generates an MD5 hash of the email that serves as the avatar lookup key.

## Architecture

```
users table          photos table
-----------          ------------
id (PK)              id (PK)
email (unique)       user_id (FK → users)
hash (unique)        is_active (bool)
```

Photos are stored on disk at `images/<photo_id>.png`. The active photo for a user is served via their email hash.

## Setup

### Prerequisites
- Go 1.24+
- Docker & Docker Compose

### Run

```bash
# Start Postgres
docker-compose up -d

# Run the service
go run .
```

The server starts on `:8080`.

## API

### Create User
```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com"}'
```

### Upload Photo
```bash
curl -X POST http://localhost:8080/users/1/photos \
  -F "photo=@path/to/image.png"
```

### Set Active Photo
```bash
curl -X PUT http://localhost:8080/users/1/photos/1/activate
```

### Get Avatar by Hash
```bash
curl http://localhost:8080/avatar/<md5_hash> --output avatar.png
```

The hash is the MD5 of the trimmed, lowercased email:
```bash
echo -n "test@example.com" | md5sum
```

## Key Design Decisions

- **Hash generation** is server-side (MD5 of lowercased, trimmed email)
- **Active photo switch** uses a transaction to deactivate the current and activate the new one
- **Photo storage** is file-based using the photo's DB id as the filename
