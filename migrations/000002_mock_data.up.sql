-- 000002_seed_data.up.sql

-- Mogg data
INSERT INTO restaurants (name, description, api_key, is_active)
VALUES
    ('Dominos Pizza', 'Dominos Pizza', 'partner_api_key_dominos', TRUE),
    ('Rostics', 'KFC but Rostics', 'partner_api_key_rostics', TRUE),
    ('Closed Bistro', 'Closed for renovation', 'partner_api_key_closed', FALSE)
ON CONFLICT (api_key) DO NOTHING;

-- Mogg data
INSERT INTO menu_items (restaurant_id, external_id, name, price_cents, is_available)
SELECT r.id, v.external_id, v.name, v.price_cents, v.is_available
FROM (
    VALUES
        ('partner_api_key_dominos', 'pizza_margherita', 'Пицца Маргарита', 450000, TRUE),
        ('partner_api_key_dominos', 'pizza_pepperoni', 'Пицца Пепперони', 550000, TRUE),
        ('partner_api_key_dominos', 'pizza_4cheeses', 'Пицца 4 сыра', 620000, TRUE),
        ('partner_api_key_dominos', 'tiramisu', 'Десерт Тирамису', 280000, TRUE),
        ('partner_api_key_rostics', 'classic_burger', 'Бургер Классический', 390000, TRUE),
        ('partner_api_key_rostics', 'double_beef_burger', 'Бургер Двойной Говяжий', 590000, TRUE),
        ('partner_api_key_rostics', 'fries', 'Картофель фри', 180000, TRUE)
) AS v(restaurant_api_key, external_id, name, price_cents, is_available)
JOIN restaurants r ON r.api_key = v.restaurant_api_key
ON CONFLICT (restaurant_id, external_id) DO NOTHING;
