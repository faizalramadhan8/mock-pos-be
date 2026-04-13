-- Member-specific pricing per product (NULL = no member price, use selling_price)
ALTER TABLE products
    ADD COLUMN member_price DECIMAL(15,2) NULL AFTER selling_price;

-- Link orders to a registered member (NULL = walk-in / non-member)
ALTER TABLE orders
    ADD COLUMN member_id VARCHAR(36) NULL AFTER customer,
    ADD KEY idx_orders_member (member_id),
    ADD CONSTRAINT fk_orders_member
        FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE SET NULL;

-- Capture the non-member price at sale time so we can report savings
-- accurately even after future price changes
ALTER TABLE order_items
    ADD COLUMN regular_price DECIMAL(15,2) NULL AFTER unit_price;
