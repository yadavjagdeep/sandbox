CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Publication defines which tables changes are captured
-- replication slots will use this to know what to decode
CREATE PUBLICATION cdc_pub FOR TABLE users;