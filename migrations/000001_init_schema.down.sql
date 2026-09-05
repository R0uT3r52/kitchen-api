-- 000001_init_schema.down.sql

DROP TRIGGER IF EXISTS set_timestamp_orders ON orders;
DROP TRIGGER IF EXISTS set_timestamp_menu_items ON menu_items;
DROP TRIGGER IF EXISTS set_timestamp_restaurants ON restaurants;

DROP FUNCTION IF EXISTS trigger_set_timestamp();

DROP TABLE IF EXISTS order_events CASCADE;
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS menu_items CASCADE;
DROP TABLE IF EXISTS restaurants CASCADE;
