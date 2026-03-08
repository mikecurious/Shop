-- Default categories
INSERT INTO categories (id, name, description) VALUES
    (uuid_generate_v4(), 'Beverages', 'Drinks and liquid products'),
    (uuid_generate_v4(), 'Snacks', 'Chips, biscuits and snack items'),
    (uuid_generate_v4(), 'Dairy', 'Milk, cheese and dairy products'),
    (uuid_generate_v4(), 'Household', 'Cleaning and household supplies'),
    (uuid_generate_v4(), 'Personal Care', 'Soaps, shampoos and toiletries')
ON CONFLICT DO NOTHING;
