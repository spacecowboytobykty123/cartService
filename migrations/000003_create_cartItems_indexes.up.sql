CREATE INDEX IF NOT EXISTS idx_cart_items_user_id ON cart_items(user_id);

CREATE INDEX IF NOT EXISTS idx_cart_items_user_toy ON cart_items(user_id, toy_id);