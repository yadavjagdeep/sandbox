CREATE TABLE inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id VARCHAR(50) NOT NULL,
    user_id VARCHAR(50),
    held_at TIMESTAMP,
    paid BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_inventory_available ON inventory(item_id) WHERE user_id IS NULL;
CREATE INDEX idx_inventory_held ON inventory(held_at) WHERE user_id IS NOT NULL AND paid = FALSE;

-- Seed: 10 units of "iphone" for testing
INSERT INTO inventory (item_id) VALUES
    ('iphone'), ('iphone'), ('iphone'), ('iphone'), ('iphone'),
    ('iphone'), ('iphone'), ('iphone'), ('iphone'), ('iphone');
