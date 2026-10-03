-- 000001_init_schema.up.sql

CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS restaurants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GET /api/v1/restaurants
CREATE INDEX idx_restaurants_is_active ON restaurants (is_active) WHERE is_active = TRUE;

CREATE TRIGGER set_timestamp_restaurants
BEFORE UPDATE ON restaurants
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();

CREATE TABLE IF NOT EXISTS menu_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    name TEXT NOT NULL,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_menu_items_restaurant_external UNIQUE (restaurant_id, external_id)
);

-- GET /api/v1/restaurants/{id}/menu
CREATE INDEX idx_menu_items_restaurant_lookup ON menu_items (restaurant_id, is_available);

CREATE TRIGGER set_timestamp_menu_items
BEFORE UPDATE ON menu_items
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();

CREATE TABLE IF NOT EXISTS orders (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id TEXT NOT NULL,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('created', 'accepted', 'rejected', 'cooking', 'ready', 'completed', 'cancelled')),
    total_price_cents BIGINT NOT NULL CHECK (total_price_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GET /api/v1/partner/orders?status=created
CREATE INDEX idx_orders_restaurant_status ON orders (restaurant_id, status);

-- GET /api/v1/orders?user_id={id}
CREATE INDEX idx_orders_user_created ON orders (user_id, created_at DESC);

CREATE TRIGGER set_timestamp_orders
BEFORE UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();


CREATE TABLE IF NOT EXISTS order_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id BIGINT NOT NULL REFERENCES menu_items(id) ON DELETE RESTRICT,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);

CREATE TABLE IF NOT EXISTS order_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    old_status TEXT CHECK (old_status IS NULL OR old_status IN ('created', 'accepted', 'rejected', 'cooking', 'ready', 'completed', 'cancelled')),
    new_status TEXT NOT NULL CHECK (new_status IN ('created', 'accepted', 'rejected', 'cooking', 'ready', 'completed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Get order statuses history (for debug)
CREATE INDEX idx_order_events_order_id ON order_events (order_id, created_at ASC);
