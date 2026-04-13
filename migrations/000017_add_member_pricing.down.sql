ALTER TABLE order_items DROP COLUMN regular_price;

ALTER TABLE orders
    DROP FOREIGN KEY fk_orders_member,
    DROP KEY idx_orders_member,
    DROP COLUMN member_id;

ALTER TABLE products DROP COLUMN member_price;
