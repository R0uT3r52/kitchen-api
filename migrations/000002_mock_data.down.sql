-- 000002_seed_data.down.sql

DELETE FROM menu_items
WHERE external_id IN (
    'pizza_margharita',
    'pizza_pepperoni',
    'pizza_4cheeses',
    'tiramisu',
    'classic_burger',
    'double_beef_burger',
    'fries'
);

DELETE FROM restaurants
WHERE api_key IN (
    'partner_api_key_dominos',
    'partner_api_key_rostics',
    'partner_api_key_closed'
);
