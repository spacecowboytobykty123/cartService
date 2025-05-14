CREATE TABLE IF NOT EXISTS cart_items (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES carts(user_id) ON DELETE CASCADE,
    toy_id BIGINT NOT NULL,
    quantity INT NOT NULL DEFAULT 1 CHECK  (quantity > 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, toy_id)
);