ALTER TABLE order_items
    ADD COLUMN purchase_price DECIMAL(15,2) NULL AFTER unit_price;
